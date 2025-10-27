package bot

import (
	"context"
	"fmt"
	"log"
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
				if msg != nil && msg.Content != "" {
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

				// Skip empty messages
				content := strings.TrimSpace(msg.Content)
				if content == "" {
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
				if !addressed {
					addressed = strings.Contains(strings.ToLower(content), "@grok")
				}

				// Clean content for history
				cleanContent := content
				if addressed {
					cleanContent = strings.ReplaceAll(cleanContent, fmt.Sprintf("<@%s>", discord.State.User.ID), "")
					cleanContent = strings.ReplaceAll(strings.ToLower(cleanContent), "@grok", "")
					cleanContent = strings.TrimSpace(cleanContent)
				}

				if cleanContent != "" {
					// Add user message to history
					chatHistory.Append(channel.ID, ChatMessage{
						Role:     "user",
						Content:  cleanContent,
						Username: msg.Author.Username,
					})

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

	if !doesMessageMention(message.Mentions, discord.State.User.ID) {
		chatHistory.Append(channelID, ChatMessage{Role: "user", Content: content, Username: message.Author.Username})
	} else {
		// Remove the bot mention from the content
		content = strings.ReplaceAll(content, fmt.Sprintf("<@%s>", discord.State.User.ID), "")

		// Build messages with system prompt + prior channel history + new user message
		prior := chatHistory.Get(channelID)
		messages := make([]ChatMessage, 0, 1+len(prior)+1)
		messages = append(messages, ChatMessage{Role: "system", Content: config.Bot.DefaultSystemMessage})
		messages = append(messages, prior...)
		messages = append(messages, ChatMessage{Role: "user", Content: content, Username: message.Author.Username})

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
		chatHistory.Append(channelID, ChatMessage{Role: "user", Content: content, Username: message.Author.Username})
		chatHistory.Append(channelID, ChatMessage{Role: "assistant", Content: response})

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

func doesMessageMention(users []*discordgo.User, id string) bool {
	for _, user := range users {
		if user.ID == id {
			return true
		}
	}
	return false
}
