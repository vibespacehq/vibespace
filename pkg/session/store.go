package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// validSessionName matches alphanumeric, dash, and underscore (1-64 chars)
var validSessionName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

func ValidateSessionName(name string) error {
	if name == "" {
		return fmt.Errorf("session name cannot be empty")
	}
	if !validSessionName.MatchString(name) {
		return fmt.Errorf("session name '%s' is invalid: must be 1-64 characters, alphanumeric with dash/underscore, starting with alphanumeric", name)
	}
	return nil
}

// Store manages session persistence
type Store struct {
	dir string // ~/.vibespace/sessions/
}

func NewStore() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	dir := filepath.Join(home, ".vibespace", "sessions")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create sessions directory: %w", err)
	}

	return &Store{dir: dir}, nil
}

func (s *Store) sessionPath(name string) string {
	return filepath.Join(s.dir, name+".json")
}

func (s *Store) List() ([]Session, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read sessions directory: %w", err)
	}

	var sessions []Session
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".json")
		sess, err := s.Get(name)
		if err != nil {
			continue // Skip invalid files
		}
		sessions = append(sessions, *sess)
	}

	// Sort by last used (most recent first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastUsed.After(sessions[j].LastUsed)
	})

	return sessions, nil
}

// Get retrieves a session by name
func (s *Store) Get(name string) (*Session, error) {
	path := s.sessionPath(name)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("session '%s' not found", name)
		}
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, fmt.Errorf("failed to parse session file: %w", err)
	}

	return &sess, nil
}

// Save stores a session
func (s *Store) Save(session *Session) error {
	if err := ValidateSessionName(session.Name); err != nil {
		return err
	}

	// Update last used time
	session.LastUsed = time.Now()

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	path := s.sessionPath(session.Name)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}

	return nil
}

func (s *Store) SaveNew(session *Session) error {
	if err := ValidateSessionName(session.Name); err != nil {
		return err
	}

	if s.Exists(session.Name) {
		return fmt.Errorf("session '%s' already exists", session.Name)
	}

	return s.Save(session)
}

// Delete removes a session
func (s *Store) Delete(name string) error {
	path := s.sessionPath(name)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("session '%s' not found", name)
		}
		return fmt.Errorf("failed to delete session file: %w", err)
	}
	return nil
}

func (s *Store) Exists(name string) bool {
	path := s.sessionPath(name)
	_, err := os.Stat(path)
	return err == nil
}

// Create creates a new session
func (s *Store) Create(name string) (*Session, error) {
	if s.Exists(name) {
		return nil, fmt.Errorf("session '%s' already exists", name)
	}

	session := &Session{
		Name:       name,
		CreatedAt:  time.Now(),
		LastUsed:   time.Now(),
		Vibespaces: []VibespaceEntry{},
		Layout: Layout{
			Mode: LayoutModeSplit,
		},
	}

	if err := s.Save(session); err != nil {
		return nil, err
	}

	return session, nil
}
