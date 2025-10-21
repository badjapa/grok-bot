package bot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// GrokClient handles communication with XAI's Grok API
type GrokClient struct {
	Config *GrokConfig
	Client *http.Client
}

// ChatMessage represents a message in the chat completion request
type ChatMessage struct {
	Role     string `json:"role"`
	Content  string `json:"content"`
	Username string `json:"username,omitempty"` // Optional username for context
}

// ChatCompletionRequest represents the request payload for chat completions
type ChatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

// ChatCompletionResponse represents the response from the chat completions API
type ChatCompletionResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// XAIError represents an error response from the XAI API
type XAIError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// NewGrokClient creates a new instance of GrokClient
func NewGrokClient(config *GrokConfig) *GrokClient {
	return &GrokClient{
		Config: config,
		Client: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// CreateChatCompletion sends a chat completion request to the XAI API
func (g *GrokClient) CreateChatCompletion(messages []ChatMessage) (string, error) {
	// Format messages with usernames for context
	formattedMessages := make([]ChatMessage, len(messages))
	for i, msg := range messages {
		formattedMessages[i] = msg
		if msg.Username != "" && msg.Role == "user" {
			formattedMessages[i].Content = fmt.Sprintf("[%s]: %s", msg.Username, msg.Content)
		}
	}

	request := ChatCompletionRequest{
		Model:       g.Config.Model,
		Messages:    formattedMessages,
		Temperature: g.Config.Temperature,
		MaxTokens:   g.Config.MaxTokens,
		Stream:      g.Config.Stream,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := g.Config.BaseURL + "/chat/completions"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.Config.APIKey)

	resp, err := g.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var xaiErr XAIError
		if err := json.Unmarshal(body, &xaiErr); err == nil && xaiErr.Error.Message != "" {
			return "", fmt.Errorf("XAI API error: %s (code: %s)", xaiErr.Error.Message, xaiErr.Error.Code)
		}
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var response ChatCompletionResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no choices returned from XAI API")
	}

	return response.Choices[0].Message.Content, nil
}

// CompleteText is a convenience method for simple text completion
func (g *GrokClient) CompleteText(prompt string, systemMessage string) (string, error) {
	messages := []ChatMessage{
		{
			Role:    "system",
			Content: systemMessage,
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}
	return g.CreateChatCompletion(messages)
}

// CompleteTextWithSystem allows custom system message for specialized contexts
func (g *GrokClient) CompleteTextWithSystem(systemMessage, userMessage string) (string, error) {
	messages := []ChatMessage{
		{
			Role:    "system",
			Content: systemMessage,
		},
		{
			Role:    "user",
			Content: userMessage,
		},
	}
	return g.CreateChatCompletion(messages)
}
