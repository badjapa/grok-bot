package bot

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
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

	discord, err := discordgo.New("Bot " + config.Discord.Token)
	if err != nil {
		log.Fatal("Error connecting to discord")
	}

	discord.AddHandler(handleMessage)

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

	discord, err := discordgo.New("Bot " + config.Discord.Token)
	if err != nil {
		log.Fatal("Error connecting to discord")
	}

	discord.AddHandler(handleMessage)

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
			// Try different batch sizes to work around unknown component type errors
			var messages []*discordgo.Message
			var msgErr error

			// Try to get messages, starting with the full amount
			messages, msgErr = discord.ChannelMessages(channel.ID, chatHistory.GetMax(), "", "", "")
			if msgErr != nil && strings.Contains(msgErr.Error(), "unknown component type") {
				// If we get unknown component type error, try smaller batches
				log.Printf("Channel %s has messages with unknown components, trying smaller batches...", channel.Name)

				// Try smaller batches to work around problematic messages
				for batchSize := chatHistory.GetMax() / 2; batchSize >= 5; batchSize /= 2 {
					messages, msgErr = discord.ChannelMessages(channel.ID, batchSize, "", "", "")
					if msgErr == nil {
						log.Printf("Successfully retrieved %d messages from %s using batch size %d", len(messages), channel.Name, batchSize)
						break
					}
					if !strings.Contains(msgErr.Error(), "unknown component type") {
						break // Different error, don't retry
					}
				}
			}

			if msgErr != nil {
				log.Printf("Error getting messages from channel %s: %v", channel.Name, msgErr)
				continue
			}

			// Filter out messages that couldn't be parsed due to unknown components
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

				// Skip bot's own messages
				if msg.Author.ID == discord.State.User.ID {
					continue
				}

				// Skip empty messages (no content and no attachments)
				content := strings.TrimSpace(msg.Content)
				imageURLs := extractImageURLsFromAttachments(msg.Attachments)
				if content == "" && len(imageURLs) == 0 {
					continue
				}

				// Determine if this was a message that addressed the bot
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

				// Clean content for history
				cleanContent := content
				if addressed {
					cleanContent = strings.ReplaceAll(cleanContent, fmt.Sprintf("<@%s>", discord.State.User.ID), "")
					cleanContent = strings.ReplaceAll(strings.ToLower(cleanContent), "@grok", "")
					cleanContent = strings.TrimSpace(cleanContent)
				}

				if cleanContent != "" || len(imageURLs) > 0 {
					// Add user message to history using multimodal message creation
					multimodalMsg := CreateMultimodalMessage("user", cleanContent, imageURLs, msg.Author.Username)
					chatHistory.Append(channel.ID, multimodalMsg)

					// If this was an addressed message, look for bot's response in subsequent messages
					if addressed {
						// Look for bot's response in the next few messages
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

func handleMessage(discord *discordgo.Session, message *discordgo.MessageCreate) {
	if message.Author.ID == discord.State.User.ID {
		return
	}

	content := strings.TrimSpace(message.Content)
	channelID := message.ChannelID
	attachments := message.Attachments
	imageURLs := extractImageURLsFromAttachments(attachments)

	if !doesMessageMention(message.Mentions, discord.State.User.ID) {

		chatHistory.Append(channelID, CreateMultimodalMessage("user", content, imageURLs, message.Author.Username))
	} else {
		// Remove the bot mention from the content
		content = strings.ReplaceAll(content, fmt.Sprintf("<@%s>", discord.State.User.ID), "")

		// Build messages with system prompt + prior channel history + new user message
		prior := chatHistory.Get(channelID)
		messages := make([]ChatMessage, 0, 1+len(prior)+1)
		messages = append(messages, ChatMessage{Role: "system", Content: config.Bot.DefaultSystemMessage})
		messages = append(messages, prior...)
		messages = append(messages, CreateMultimodalMessage("user", content, imageURLs, message.Author.Username))

		// Send typing indicator
		discord.ChannelTyping(message.ChannelID)

		// Get response from Grok
		response, err := grokClient.CreateChatCompletion(messages)
		if err != nil {
			log.Printf("Error getting Grok response: %v", err)
			discord.ChannelMessageSend(message.ChannelID, "Sorry, I encountered an error processing your request. Please try again.")
			return
		}

		// Append to history: user then assistant
		chatHistory.Append(channelID, CreateMultimodalMessage("user", content, imageURLs, message.Author.Username))
		chatHistory.Append(channelID, CreateTextMessage("assistant", response, ""))

		// Send the response back to Discord
		err = sendMessage(discord, message.ChannelID, response)
		if err != nil {
			log.Printf("Error sending message: %v", err)
		}
	}

}

// sendMessage sends a message to Discord, handling size limits by sending as file if needed
func sendMessage(discord *discordgo.Session, channelID, content string) error {
	// Check if message is within Discord's character limit
	maxLength := config.Bot.MaxMessageSize
	if len(content) <= maxLength {
		_, err := discord.ChannelMessageSend(channelID, content)
		return err
	}

	// Message is too long, send as markdown file
	return sendAsMarkdownFile(discord, channelID, content)
}

// sendAsMarkdownFile sends content as a markdown file attachment
func sendAsMarkdownFile(discord *discordgo.Session, channelID, content string) error {
	// Create a temporary file
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("grok_response_%s.md", timestamp)

	// Create the file
	file, err := os.CreateTemp("", filename)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(file.Name()) // Clean up temp file

	// Write content to file
	_, err = file.WriteString(content)
	if err != nil {
		file.Close()
		return fmt.Errorf("failed to write to temp file: %w", err)
	}
	file.Close()

	// Check file size
	fileInfo, err := os.Stat(file.Name())
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	if fileInfo.Size() > MaxDiscordFileSize {
		return fmt.Errorf("response too large even for file upload (%d bytes)", fileInfo.Size())
	}

	// Send file to Discord
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

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second, // Longer timeout for downloading image content
	}

	// Download the image
	resp, err := client.Get(url)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	// Check if the response is successful
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("image download failed with status %d", resp.StatusCode)
	}

	// Read the image data
	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read image data: %w", err)
	}

	// Check image size (limit to 20MB to be safe with API limits)
	const maxImageSize = 20 * 1024 * 1024 // 20MB
	if len(imageData) > maxImageSize {
		return nil, "", fmt.Errorf("image too large: %d bytes (max %d bytes)", len(imageData), maxImageSize)
	}

	// Get content type from response header
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		// Fallback to detecting from URL or default
		contentType = "image/jpeg"
	}

	return imageData, contentType, nil
}

// imageToDataURL converts image bytes to a base64 data URL
func imageToDataURL(imageData []byte, contentType string) string {
	// Encode to base64
	base64Data := base64.StdEncoding.EncodeToString(imageData)

	// Return data URL
	return fmt.Sprintf("data:%s;base64,%s", contentType, base64Data)
}

// validateImageURL checks if an image URL is accessible (not 404)
func validateImageURL(url string) bool {
	if url == "" {
		return false
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Use HEAD request to check if URL is accessible without downloading the full content
	resp, err := client.Head(url)
	if err != nil {
		log.Printf("Error validating image URL %s: %v", url, err)
		return false
	}
	defer resp.Body.Close()

	// Check if the response is successful (2xx status codes)
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

		// Download the image
		imageData, contentType, err := downloadImage(attachment.URL)
		if err != nil {
			log.Printf("Failed to download image from %s: %v", attachment.URL, err)
			continue
		}

		// Convert to base64 data URL
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

	// Grok API supported image formats: JPEG, PNG, WebP
	supportedImageTypes := []string{
		"image/jpeg",
		"image/jpg",
		"image/png",
		"image/webp",
	}

	// Check content type first (most reliable)
	if attachment.ContentType != "" {
		contentType := strings.ToLower(attachment.ContentType)
		for _, supportedType := range supportedImageTypes {
			if contentType == supportedType {
				return true
			}
		}
	}

	// Fallback: check file extension if content type is not available
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
