package tui

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// HistoryStore handles persistence of chat history to JSONL files
type HistoryStore struct {
	dir string // Base directory for history files (~/.vibespace/history/)
}

// NewHistoryStore creates a new history store
func NewHistoryStore() (*HistoryStore, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	dir := filepath.Join(homeDir, ".vibespace", "history")

	// Ensure directory exists
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create history directory: %w", err)
	}

	return &HistoryStore{dir: dir}, nil
}

// historyPath returns the path to a session's history file
func (s *HistoryStore) historyPath(sessionName string) string {
	// Sanitize session name for filesystem
	safeName := sanitizeFilename(sessionName)
	return filepath.Join(s.dir, safeName+".jsonl")
}

// sanitizeFilename makes a string safe for use as a filename
func sanitizeFilename(name string) string {
	// Replace problematic characters
	result := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch c {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|', ' ':
			result = append(result, '_')
		default:
			result = append(result, c)
		}
	}
	if len(result) == 0 {
		return "default"
	}
	return string(result)
}

// Append adds a message to a session's history file
func (s *HistoryStore) Append(sessionName string, msg *Message) error {
	path := s.historyPath(sessionName)

	// Open file in append mode, create if not exists
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open history file: %w", err)
	}
	defer f.Close()

	// Encode message as JSON
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Write JSON line
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	return nil
}

// Load reads all messages from a session's history file
func (s *HistoryStore) Load(sessionName string) ([]*Message, error) {
	path := s.historyPath(sessionName)

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			// No history yet, return empty
			return nil, nil
		}
		return nil, fmt.Errorf("failed to open history file: %w", err)
	}
	defer f.Close()

	var messages []*Message
	scanner := bufio.NewScanner(f)
	// Increase buffer size for potentially large messages
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if line == "" {
			continue
		}

		var msg Message
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			// Skip malformed lines but log
			continue
		}

		messages = append(messages, &msg)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read history file: %w", err)
	}

	return messages, nil
}

// Clear removes all history for a session
func (s *HistoryStore) Clear(sessionName string) error {
	path := s.historyPath(sessionName)

	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove history file: %w", err)
	}

	return nil
}

// List returns all available session names with history
func (s *HistoryStore) List() ([]string, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read history directory: %w", err)
	}

	var sessions []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) == ".jsonl" {
			// Remove .jsonl extension
			sessions = append(sessions, name[:len(name)-6])
		}
	}

	return sessions, nil
}

// Exists checks if history exists for a session
func (s *HistoryStore) Exists(sessionName string) bool {
	path := s.historyPath(sessionName)
	_, err := os.Stat(path)
	return err == nil
}

// Truncate keeps only the last N messages for a session
func (s *HistoryStore) Truncate(sessionName string, keepLast int) error {
	messages, err := s.Load(sessionName)
	if err != nil {
		return err
	}

	if len(messages) <= keepLast {
		return nil
	}

	// Keep only last N messages
	messages = messages[len(messages)-keepLast:]

	// Clear and rewrite
	if err := s.Clear(sessionName); err != nil {
		return err
	}

	for _, msg := range messages {
		if err := s.Append(sessionName, msg); err != nil {
			return err
		}
	}

	return nil
}
