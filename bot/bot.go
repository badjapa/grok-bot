package bot

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

var grokClient *GrokClient
var chatHistory *ChatHistory
var config *Config

// Discord message limits
const (
	MaxDiscordMessageLength = 2000            // Discord's character limit for messages
	MaxDiscordFileSize      = 8 * 1024 * 1024 // 8MB file size limit
)

// RunWithConfig runs the bot with the provided configuration
func RunWithConfig(cfg *Config) {
	config = cfg

	// Initialize Grok client
	grokClient = NewGrokClient(&config.Grok)

	// Initialize chat history with configurable size
	chatHistory = NewChatHistory(config.Bot.MaxHistory)

	if config.Discord.Token == "" {
		log.Fatal("Discord Bot token not provided")
	}

	discord, err := newDiscordSession(config.Discord.Token)
	if err != nil {
		log.Fatal("Error connecting to discord")
	}

	err = discord.Open()
	if err != nil {
		log.Fatal("Error opening Discord connection:", err)
	}

	defer discord.Close()

	// Populate chat history with recent messages if enabled
	if config.Bot.EnableHistory {
		populateHistoryFromChannels(discord)
	}

	log.Println("Grok-bot running...")

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
}

// RunWithConfigAsync runs the bot with the provided configuration and supports context cancellation
func RunWithConfigAsync(ctx context.Context, cfg *Config) {
	config = cfg

	// Initialize Grok client
	grokClient = NewGrokClient(&config.Grok)

	// Initialize chat history with configurable size
	chatHistory = NewChatHistory(config.Bot.MaxHistory)

	if config.Discord.Token == "" {
		log.Fatal("Discord Bot token not provided")
	}

	discord, err := newDiscordSession(config.Discord.Token)
	if err != nil {
		log.Fatal("Error connecting to discord")
	}

	err = discord.Open()
	if err != nil {
		log.Fatal("Error opening Discord connection:", err)
	}

	defer discord.Close()

	// Populate chat history with recent messages if enabled
	if config.Bot.EnableHistory {
		populateHistoryFromChannels(discord)
	}

	log.Println("Grok-bot running...")

	// Wait for context cancellation instead of signal
	<-ctx.Done()
	log.Println("Discord bot shutting down...")
}

func newDiscordSession(token string) (*discordgo.Session, error) {
	discord, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}

	discord.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages | discordgo.IntentsMessageContent
	discord.AddHandler(handleMessage)
	return discord, nil
}

// populateHistoryFromChannels reads recent messages from channels with read/write access to populate chat history
func populateHistoryFromChannels(discord *discordgo.Session) {
	log.Println("=== Populating chat history from recent messages ===")

	for _, guild := range discord.State.Guilds {
		log.Printf("Reading messages from server: %s", guild.Name)

		channels, err := discord.GuildChannels(guild.ID)
		if err != nil {
			log.Printf("Error getting channels for guild %s: %v", guild.Name, err)
			continue
		}

		for _, channel := range channels {
			// Only process text channels
			if channel.Type != discordgo.ChannelTypeGuildText {
				continue
			}

			// Check if bot has permission to read and send messages
			permissions, err := discord.UserChannelPermissions(discord.State.User.ID, channel.ID)
			if err != nil {
				log.Printf("Error checking permissions for channel %s: %v", channel.Name, err)
				continue
			}

			canReadMessages := permissions&discordgo.PermissionViewChannel != 0
			canSendMessages := permissions&discordgo.PermissionSendMessages != 0

			// Only initialize history for channels with both read and write access
			if !canReadMessages || !canSendMessages {
				continue
			}

			// Get recent messages (last maxHistory messages per channel)
			var messages []*discordgo.Message
			var msgErr error

			messages, msgErr = discord.ChannelMessages(channel.ID, chatHistory.GetMax(), "", "", "")
			if msgErr != nil && strings.Contains(msgErr.Error(), "unknown component type") {
				log.Printf("Channel %s has messages with unknown components, trying smaller batches...", channel.Name)

				for batchSize := chatHistory.GetMax() / 2; batchSize >= 5; batchSize /= 2 {
					messages, msgErr = discord.ChannelMessages(channel.ID, batchSize, "", "", "")
					if msgErr == nil {
						log.Printf("Successfully retrieved %d messages from %s using batch size %d", len(messages), channel.Name, batchSize)
						break
					}
					if !strings.Contains(msgErr.Error(), "unknown component type") {
						break
					}
				}
			}

			if msgErr != nil {
				log.Printf("Error getting messages from channel %s: %v", channel.Name, msgErr)
				continue
			}

			var validMessages []*discordgo.Message
			for _, msg := range messages {
				if msg != nil && (msg.Content != "" || len(msg.Attachments) > 0) {
					validMessages = append(validMessages, msg)
				}
			}
			messages = validMessages

			// Process messages in reverse order (oldest first)
			for i := len(messages) - 1; i >= 0; i-- {
				msg := messages[i]

				if msg.Author.ID == discord.State.User.ID {
					continue
				}

				content := strings.TrimSpace(msg.Content)
				imageURLs := extractImageURLsFromAttachments(msg.Attachments)
				if content == "" && len(imageURLs) == 0 {
					continue
				}

				addressed := false
				for _, mention := range msg.Mentions {
					if mention.ID == discord.State.User.ID {
						addressed = true
						break
					}
				}
				if !addressed && content != "" {
					addressed = strings.Contains(strings.ToLower(content), "@grok")
				}

				cleanContent := content
				if addressed {
					cleanContent = strings.ReplaceAll(cleanContent, fmt.Sprintf("<@%s>", discord.State.User.ID), "")
					cleanContent = strings.ReplaceAll(strings.ToLower(cleanContent), "@grok", "")
					cleanContent = strings.TrimSpace(cleanContent)
				}

				if cleanContent != "" || len(imageURLs) > 0 {
					multimodalMsg := CreateMultimodalMessage("user", cleanContent, imageURLs, msg.Author.Username)
					chatHistory.Append(channel.ID, multimodalMsg)

					if addressed {
						for j := i - 1; j >= 0 && j > i-5; j-- {
							responseMsg := messages[j]
							if responseMsg.Author.ID == discord.State.User.ID {
								responseContent := strings.TrimSpace(responseMsg.Content)
								if responseContent != "" {
									chatHistory.Append(channel.ID, ChatMessage{
										Role:    "assistant",
										Content: responseContent,
									})
								}
								break
							}
						}
					}
				}
			}

			log.Printf("  - Processed %d messages from #%s", len(messages), channel.Name)
		}
	}

	log.Println("=== Finished populating chat history ===")
}

const (
	kekwEmoji       = "KEKW:734305966190887012"
	kekwReactChance = 0.1
)

func maybeReactKEKW(discord *discordgo.Session, message *discordgo.MessageCreate) {
	if message.Author.Bot {
		return
	}
	if config != nil && !config.Bot.EnableEmojis {
		return
	}
	if rand.Float64() >= kekwReactChance {
		return
	}
	if err := discord.MessageReactionAdd(message.ChannelID, message.ID, kekwEmoji); err != nil {
		log.Printf("Error adding KEKW reaction: %v", err)
	}
}

func handleMessage(discord *discordgo.Session, message *discordgo.MessageCreate) {
	if message.Author.ID == discord.State.User.ID {
		return
	}

	// Feature: Replace fixupx.com with x.com links
	if !message.Author.Bot && strings.Contains(strings.ToLower(message.Content), "fixupx.com") {
		// Replace all occurrences of fixupx.com with x.com
		fixedContent := strings.ReplaceAll(message.Content, "fixupx.com", "x.com")
		fixedContent = strings.ReplaceAll(fixedContent, "Fixupx.com", "x.com")
		fixedContent = strings.ReplaceAll(fixedContent, "FIXUPX.COM", "x.com")

		// Construct the replacement message format
		repostMessage := fmt.Sprintf("**%s** sent:\n%s", message.Author.Username, fixedContent)

		// Delete original message
		err := discord.ChannelMessageDelete(message.ChannelID, message.ID)
		if err != nil {
			log.Printf("Error deleting message containing fixupx.com: %v", err)
		}

		// Send corrected text message
		err = sendMessage(discord, message.ChannelID, repostMessage)
		if err != nil {
			log.Printf("Error reposting fixed x.com message: %v", err)
		}
		return
	}

	maybeReactKEKW(discord, message)

	content := strings.TrimSpace(message.Content)
	channelID := message.ChannelID
	attachments := message.Attachments
	imageURLs := extractImageURLsFromAttachments(attachments)

	if !doesMessageMention(message.Mentions, discord.State.User.ID) {
		chatHistory.Append(channelID, CreateMultimodalMessage("user", content, imageURLs, message.Author.Username))
	} else {
		content = strings.ReplaceAll(content, fmt.Sprintf("<@%s>", discord.State.User.ID), "")

		prior := chatHistory.Get(channelID)
		messages := make([]ChatMessage, 0, 1+len(prior)+1)
		messages = append(messages, ChatMessage{Role: "system", Content: config.Bot.DefaultSystemMessage})
		messages = append(messages, prior...)
		messages = append(messages, CreateMultimodalMessage("user", content, imageURLs, message.Author.Username))

		discord.ChannelTyping(message.ChannelID)

		response, err := grokClient.CreateChatCompletion(messages)
		if err != nil {
			log.Printf("Error getting Grok response: %v", err)
			discord.ChannelMessageSend(message.ChannelID, "Sorry, I encountered an error processing your request. Please try again.")
			return
		}

		chatHistory.Append(channelID, CreateMultimodalMessage("user", content, imageURLs, message.Author.Username))
		chatHistory.Append(channelID, CreateTextMessage("assistant", response, ""))

		err = sendMessage(discord, message.ChannelID, response)
		if err != nil {
			log.Printf("Error sending message: %v", err)
		}
	}
}

// sendMessage sends a message to Discord, handling size limits by sending as file if needed
func sendMessage(discord *discordgo.Session, channelID, content string) error {
	maxLength := config.Bot.MaxMessageSize
	if len(content) <= maxLength {
		_, err := discord.ChannelMessageSend(channelID, content)
		return err
	}

	return sendAsMarkdownFile(discord, channelID, content)
}

// sendAsMarkdownFile sends content as a markdown file attachment
func sendAsMarkdownFile(discord *discordgo.Session, channelID, content string) error {
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("grok_response_%s.md", timestamp)

	file, err := os.CreateTemp("", filename)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(file.Name())

	_, err = file.WriteString(content)
	if err != nil {
		file.Close()
		return fmt.Errorf("failed to write to temp file: %w", err)
	}
	file.Close()

	fileInfo, err := os.Stat(file.Name())
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	if fileInfo.Size() > MaxDiscordFileSize {
		return fmt.Errorf("response too large even for file upload (%d bytes)", fileInfo.Size())
	}

	fileReader, err := os.Open(file.Name())
	if err != nil {
		return fmt.Errorf("failed to open file for reading: %w", err)
	}
	defer fileReader.Close()

	_, err = discord.ChannelFileSend(channelID, filename, fileReader)
	if err != nil {
		return fmt.Errorf("failed to send file: %w", err)
	}

	return nil
}

// downloadImage downloads an image from a URL and returns the bytes and content type
func downloadImage(url string) ([]byte, string, error) {
	if url == "" {
		return nil, "", fmt.Errorf("empty URL")
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("image download failed with status %d", resp.StatusCode)
	}

	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read image data: %w", err)
	}

	const maxImageSize = 20 * 1024 * 1024
	if len(imageData) > maxImageSize {
		return nil, "", fmt.Errorf("image too large: %d bytes (max %d bytes)", len(imageData), maxImageSize)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}

	return imageData, contentType, nil
}

// imageToDataURL converts image bytes to a base64 data URL
func imageToDataURL(imageData []byte, contentType string) string {
	base64Data := base64.StdEncoding.EncodeToString(imageData)
	return fmt.Sprintf("data:%s;base64,%s", contentType, base64Data)
}

// validateImageURL checks if an image URL is accessible (not 404)
func validateImageURL(url string) bool {
	if url == "" {
		return false
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Head(url)
	if err != nil {
		log.Printf("Error validating image URL %s: %v", url, err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return true
	}

	log.Printf("Image URL %s returned status %d", url, resp.StatusCode)
	return false
}

// extractImageURLsFromAttachments extracts image URLs from Discord message attachments,
// downloads them, and converts them to base64 data URLs for reliable API access
func extractImageURLsFromAttachments(attachments []*discordgo.MessageAttachment) []string {
	var imageURLs []string

	for _, attachment := range attachments {
		if attachment == nil || !isImageAttachment(attachment) {
			continue
		}

		imageData, contentType, err := downloadImage(attachment.URL)
		if err != nil {
			log.Printf("Failed to download image from %s: %v", attachment.URL, err)
			continue
		}

		dataURL := imageToDataURL(imageData, contentType)
		imageURLs = append(imageURLs, dataURL)

		log.Printf("Successfully converted image %s to base64 data URL (%d bytes)", attachment.Filename, len(imageData))
	}

	return imageURLs
}

// isImageAttachment checks if a Discord attachment is an image supported by Grok API
func isImageAttachment(attachment *discordgo.MessageAttachment) bool {
	if attachment == nil {
		return false
	}

	supportedImageTypes := []string{
		"image/jpeg",
		"image/jpg",
		"image/png",
		"image/webp",
	}

	if attachment.ContentType != "" {
		contentType := strings.ToLower(attachment.ContentType)
		for _, supportedType := range supportedImageTypes {
			if contentType == supportedType {
				return true
			}
		}
	}

	if attachment.Filename != "" {
		filename := strings.ToLower(attachment.Filename)
		supportedExtensions := []string{".jpg", ".jpeg", ".png", ".webp"}
		for _, ext := range supportedExtensions {
			if strings.HasSuffix(filename, ext) {
				return true
			}
		}
	}

	return false
}

func doesMessageMention(users []*discordgo.User, id string) bool {
	for _, user := range users {
		if user.ID == id {
			return true
		}
	}
	return false
}
