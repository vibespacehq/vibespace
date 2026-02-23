package tui

import (
	"testing"
)

func TestChatHistoryAddAndLen(t *testing.T) {
	h := NewChatHistory(100)
	if h.Len() != 0 {
		t.Fatalf("expected empty history, got len=%d", h.Len())
	}

	h.Add(NewSystemMessage("hello"))
	h.Add(NewSystemMessage("world"))
	h.Add(NewSystemMessage("foo"))

	if h.Len() != 3 {
		t.Fatalf("expected 3 messages, got %d", h.Len())
	}
}

func TestChatHistoryMaxSize(t *testing.T) {
	h := NewChatHistory(3)
	for i := 0; i < 5; i++ {
		h.Add(NewSystemMessage("msg"))
	}
	if h.Len() != 3 {
		t.Fatalf("expected 3 (max), got %d", h.Len())
	}
}

func TestChatHistoryGetAll(t *testing.T) {
	h := NewChatHistory(100)
	h.Add(NewSystemMessage("a"))
	h.Add(NewSystemMessage("b"))

	all := h.GetAll()
	if len(all) != 2 {
		t.Fatalf("expected 2, got %d", len(all))
	}

	// Mutating returned slice should not affect history
	all[0] = nil
	if h.GetAll()[0] == nil {
		t.Fatal("GetAll should return a copy, not a reference")
	}
}

func TestChatHistoryGetVisible(t *testing.T) {
	h := NewChatHistory(100)
	for i := 0; i < 10; i++ {
		h.Add(NewSystemMessage("msg"))
	}

	// Height 5 should return last 5 messages (at bottom)
	visible := h.GetVisible(5)
	if len(visible) != 5 {
		t.Fatalf("expected 5 visible, got %d", len(visible))
	}

	// Height greater than total should return all
	visible = h.GetVisible(20)
	if len(visible) != 10 {
		t.Fatalf("expected 10 visible, got %d", len(visible))
	}
}

func TestChatHistoryScrollUpDown(t *testing.T) {
	h := NewChatHistory(100)
	for i := 0; i < 10; i++ {
		h.Add(NewSystemMessage("msg"))
	}

	// Start at bottom
	if !h.IsAtBottom() {
		t.Fatal("should start at bottom")
	}

	// Scroll up
	h.ScrollUp(3)
	if h.GetScrollPosition() != 3 {
		t.Fatalf("expected scrollPos=3, got %d", h.GetScrollPosition())
	}
	if h.IsAtBottom() {
		t.Fatal("should not be at bottom after scrolling up")
	}

	// Scroll down
	h.ScrollDown(2)
	if h.GetScrollPosition() != 1 {
		t.Fatalf("expected scrollPos=1, got %d", h.GetScrollPosition())
	}

	// Scroll down past bottom clamps to 0
	h.ScrollDown(10)
	if h.GetScrollPosition() != 0 {
		t.Fatalf("expected scrollPos=0, got %d", h.GetScrollPosition())
	}
}

func TestChatHistoryScrollUpClamp(t *testing.T) {
	h := NewChatHistory(100)
	for i := 0; i < 5; i++ {
		h.Add(NewSystemMessage("msg"))
	}

	// Scroll up way past top
	h.ScrollUp(100)
	if h.GetScrollPosition() != 4 {
		t.Fatalf("expected scrollPos=4 (max for 5 messages), got %d", h.GetScrollPosition())
	}
}

func TestChatHistoryScrollToTopBottom(t *testing.T) {
	h := NewChatHistory(100)
	for i := 0; i < 10; i++ {
		h.Add(NewSystemMessage("msg"))
	}

	h.ScrollToTop()
	if h.GetScrollPosition() != 9 {
		t.Fatalf("expected scrollPos=9 at top, got %d", h.GetScrollPosition())
	}

	h.ScrollToBottom()
	if h.GetScrollPosition() != 0 {
		t.Fatalf("expected scrollPos=0 at bottom, got %d", h.GetScrollPosition())
	}
	if !h.IsAtBottom() {
		t.Fatal("should be at bottom")
	}
}

func TestChatHistoryClear(t *testing.T) {
	h := NewChatHistory(100)
	h.Add(NewSystemMessage("a"))
	h.Add(NewSystemMessage("b"))
	h.ScrollUp(1)

	h.Clear()
	if h.Len() != 0 {
		t.Fatalf("expected 0 after clear, got %d", h.Len())
	}
	if h.GetScrollPosition() != 0 {
		t.Fatalf("expected scrollPos=0 after clear, got %d", h.GetScrollPosition())
	}
}

func TestChatHistorySetMessages(t *testing.T) {
	h := NewChatHistory(5)

	msgs := make([]*Message, 8)
	for i := range msgs {
		msgs[i] = NewSystemMessage("msg")
	}

	h.SetMessages(msgs)
	if h.Len() != 5 {
		t.Fatalf("expected 5 (trimmed to max), got %d", h.Len())
	}
	if h.GetScrollPosition() != 0 {
		t.Fatalf("expected scrollPos=0 after SetMessages, got %d", h.GetScrollPosition())
	}
}

func TestChatHistorySetScrollPosition(t *testing.T) {
	h := NewChatHistory(100)
	h.Add(NewSystemMessage("a"))

	h.SetScrollPosition(true)
	if h.IsAtBottom() {
		t.Fatal("should not be at bottom after SetScrollPosition(true)")
	}

	h.SetScrollPosition(false)
	if !h.IsAtBottom() {
		t.Fatal("should be at bottom after SetScrollPosition(false)")
	}
}

func TestChatHistoryGetVisibleEdgeCases(t *testing.T) {
	h := NewChatHistory(100)

	// Empty history
	if got := h.GetVisible(5); got != nil {
		t.Fatalf("expected nil for empty history, got %v", got)
	}

	// Height = 0
	h.Add(NewSystemMessage("a"))
	if got := h.GetVisible(0); got != nil {
		t.Fatalf("expected nil for height=0, got %v", got)
	}

	// Negative height
	if got := h.GetVisible(-1); got != nil {
		t.Fatalf("expected nil for height=-1, got %v", got)
	}
}

func TestChatHistoryScrollEmptyHistory(t *testing.T) {
	h := NewChatHistory(100)

	// Scrolling empty history should not panic
	h.ScrollUp(5)
	h.ScrollDown(5)
	h.ScrollToTop()
	h.ScrollToBottom()

	if h.GetScrollPosition() != 0 {
		t.Fatalf("expected scrollPos=0 for empty history, got %d", h.GetScrollPosition())
	}
}
