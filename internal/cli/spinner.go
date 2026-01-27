package cli

import (
	"fmt"
	"sync"
	"time"

	"github.com/yagizdagabak/vibespace/pkg/ui"
)

// Spinner provides a simple text spinner for long operations
type Spinner struct {
	message string
	done    chan struct{}
	wg      sync.WaitGroup
	active  bool
	mu      sync.Mutex
	out     *Output
}

// spinner frames for animation
var spinnerFrames = []string{"|", "/", "-", "\\"}

// NewSpinner creates a new spinner with the given message
func NewSpinner(message string) *Spinner {
	return &Spinner{
		message: message,
		done:    make(chan struct{}),
		out:     getOutput(),
	}
}

// Start begins the spinner animation
func (s *Spinner) Start() {
	s.mu.Lock()
	if s.active {
		s.mu.Unlock()
		return
	}
	s.active = true
	s.mu.Unlock()

	// If not a TTY or quiet/json mode, just print the message once
	if !s.out.IsTTY() || s.out.IsQuiet() || s.out.IsJSONMode() {
		if !s.out.IsQuiet() && !s.out.IsJSONMode() {
			fmt.Printf("%s %s\n", s.out.Teal(ui.IconStep), s.message)
		}
		return
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		frame := 0
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		// Initial print
		fmt.Printf("\r%s %s %s", s.out.Teal(spinnerFrames[frame]), s.message, "   ")

		for {
			select {
			case <-s.done:
				// Clear the line
				fmt.Printf("\r%s\r", "                                                                    ")
				return
			case <-ticker.C:
				frame = (frame + 1) % len(spinnerFrames)
				fmt.Printf("\r%s %s %s", s.out.Teal(spinnerFrames[frame]), s.message, "   ")
			}
		}
	}()
}

// Stop stops the spinner without printing a final message
func (s *Spinner) Stop() {
	s.mu.Lock()
	if !s.active {
		s.mu.Unlock()
		return
	}
	s.active = false
	s.mu.Unlock()

	if s.out.IsTTY() && !s.out.IsQuiet() && !s.out.IsJSONMode() {
		close(s.done)
		s.wg.Wait()
	}
}

// Success stops the spinner and prints a success message
func (s *Spinner) Success(msg string) {
	s.Stop()
	if !s.out.IsQuiet() && !s.out.IsJSONMode() {
		prefix := ui.SuccessPrefix(s.out.NoColor())
		fmt.Printf("%s %s\n", s.out.Green(prefix), msg)
	}
}

// Fail stops the spinner and prints a failure message
func (s *Spinner) Fail(msg string) {
	s.Stop()
	if !s.out.IsJSONMode() {
		prefix := ui.ErrorPrefix(s.out.NoColor())
		fmt.Printf("%s %s\n", s.out.Red(prefix), msg)
	}
}
