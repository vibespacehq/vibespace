package tui

import (
	"sync"
)

// ChatHistory stores the unified chronological message history
type ChatHistory struct {
	messages  []*Message
	maxSize   int
	scrollPos int // Lines from bottom (0 = at bottom)
	mu        sync.RWMutex
}

// NewChatHistory creates a new chat history with a maximum size
func NewChatHistory(maxSize int) *ChatHistory {
	return &ChatHistory{
		messages: make([]*Message, 0, maxSize),
		maxSize:  maxSize,
	}
}

// Add adds a message to the history
func (h *ChatHistory) Add(msg *Message) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.messages = append(h.messages, msg)

	// Trim if exceeds max
	if len(h.messages) > h.maxSize {
		h.messages = h.messages[len(h.messages)-h.maxSize:]
	}

	// Auto-scroll to bottom when new message arrives (unless user scrolled up)
	// Only auto-scroll if we were already at the bottom
	if h.scrollPos == 0 {
		// Already at bottom, stay there
	}
	// If user has scrolled up (scrollPos > 0), don't auto-scroll
}

// GetVisible returns messages visible in the given height
// Takes into account scroll position
func (h *ChatHistory) GetVisible(height int) []*Message {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if height <= 0 || len(h.messages) == 0 {
		return nil
	}

	// Calculate the range of messages to return
	totalMessages := len(h.messages)

	// End index is total - scrollPos (but not less than 0)
	endIdx := totalMessages - h.scrollPos
	if endIdx < 0 {
		endIdx = 0
	}
	if endIdx > totalMessages {
		endIdx = totalMessages
	}

	// Start index is end - height (but not less than 0)
	startIdx := endIdx - height
	if startIdx < 0 {
		startIdx = 0
	}

	// Return the slice
	if startIdx >= endIdx {
		return nil
	}
	return h.messages[startIdx:endIdx]
}

// GetAll returns all messages
func (h *ChatHistory) GetAll() []*Message {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]*Message, len(h.messages))
	copy(result, h.messages)
	return result
}

// ScrollUp scrolls up by the specified number of lines
func (h *ChatHistory) ScrollUp(lines int) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.scrollPos += lines
	maxScroll := len(h.messages) - 1
	if maxScroll < 0 {
		maxScroll = 0
	}
	if h.scrollPos > maxScroll {
		h.scrollPos = maxScroll
	}
}

// ScrollDown scrolls down by the specified number of lines
func (h *ChatHistory) ScrollDown(lines int) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.scrollPos -= lines
	if h.scrollPos < 0 {
		h.scrollPos = 0
	}
}

// ScrollToBottom scrolls to the bottom of the history
func (h *ChatHistory) ScrollToBottom() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.scrollPos = 0
}

// ScrollToTop scrolls to the top of the history
func (h *ChatHistory) ScrollToTop() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.scrollPos = len(h.messages) - 1
	if h.scrollPos < 0 {
		h.scrollPos = 0
	}
}

// IsAtBottom returns true if scrolled to the bottom
func (h *ChatHistory) IsAtBottom() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.scrollPos == 0
}

// Len returns the number of messages
func (h *ChatHistory) Len() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.messages)
}

// Clear removes all messages
func (h *ChatHistory) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.messages = make([]*Message, 0, h.maxSize)
	h.scrollPos = 0
}

// GetScrollPosition returns the current scroll position
func (h *ChatHistory) GetScrollPosition() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.scrollPos
}

// SetMessages replaces all messages (used for loading from persistence)
func (h *ChatHistory) SetMessages(messages []*Message) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.messages = make([]*Message, len(messages))
	copy(h.messages, messages)

	// Trim if exceeds max
	if len(h.messages) > h.maxSize {
		h.messages = h.messages[len(h.messages)-h.maxSize:]
	}

	h.scrollPos = 0
}

// SetScrollPosition sets whether we're scrolled away from bottom
// Used to track scroll state when using external viewport
func (h *ChatHistory) SetScrollPosition(scrolledUp bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if scrolledUp {
		h.scrollPos = 1 // Any non-zero value means scrolled up
	} else {
		h.scrollPos = 0 // At bottom
	}
}
