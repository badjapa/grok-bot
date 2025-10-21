package bot

import "sync"

// ChatHistory manages per-channel rolling histories of ChatMessage
// It stores the most recent maxMessages messages for each channel.
type ChatHistory struct {
	mu                sync.Mutex
	maxMessages       int
	channelToMessages map[string][]ChatMessage
}

// NewChatHistory constructs a ChatHistory with a given capacity per channel.
func NewChatHistory(max int) *ChatHistory {
	if max <= 0 {
		max = 1
	}
	return &ChatHistory{
		maxMessages:       max,
		channelToMessages: make(map[string][]ChatMessage),
	}
}

// Append adds a message to the history for a channel, trimming old entries.
func (h *ChatHistory) Append(channelID string, message ChatMessage) {
	h.mu.Lock()
	defer h.mu.Unlock()

	messages := h.channelToMessages[channelID]
	messages = append(messages, message)
	// Trim to last maxMessages
	if len(messages) > h.maxMessages {
		messages = messages[len(messages)-h.maxMessages:]
	}
	h.channelToMessages[channelID] = messages
}

// Get returns a COPY of the current history for a channel (may be empty).
func (h *ChatHistory) Get(channelID string) []ChatMessage {
	h.mu.Lock()
	defer h.mu.Unlock()

	src := h.channelToMessages[channelID]
	out := make([]ChatMessage, len(src))
	copy(out, src)
	return out
}

// SetMax updates the max history size and trims all channel histories as needed.
func (h *ChatHistory) SetMax(newMax int) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if newMax <= 0 {
		newMax = 1
	}
	h.maxMessages = newMax
	for cid, msgs := range h.channelToMessages {
		if len(msgs) > h.maxMessages {
			h.channelToMessages[cid] = msgs[len(msgs)-h.maxMessages:]
		}
	}
}

// GetMax returns the current maximum history size
func (h *ChatHistory) GetMax() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.maxMessages
}
