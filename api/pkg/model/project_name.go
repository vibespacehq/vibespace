package model

import (
	"errors"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"
)

var (
	// ErrInvalidProjectName is returned when a project name is invalid
	ErrInvalidProjectName = errors.New("invalid project name")

	// ErrProjectNameTooShort is returned when a project name is too short
	ErrProjectNameTooShort = errors.New("project name must be at least 3 characters")

	// ErrProjectNameTooLong is returned when a project name is too long
	ErrProjectNameTooLong = errors.New("project name must be at most 32 characters")

	// ErrProjectNameInvalidChars is returned when a project name contains invalid characters
	ErrProjectNameInvalidChars = errors.New("project name can only contain lowercase letters, numbers, and hyphens")

	// ErrProjectNameInvalidFormat is returned when a project name has invalid format
	ErrProjectNameInvalidFormat = errors.New("project name must start and end with a letter or number")
)

// projectNameRegex validates DNS-compatible project names
// Rules:
// - 3-32 characters
// - Lowercase letters, numbers, hyphens only
// - Must start and end with letter or number
// - No consecutive hyphens
var projectNameRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{1,30}[a-z0-9])?$`)

// ValidateProjectName validates a project name for DNS compatibility
func ValidateProjectName(name string) error {
	if name == "" {
		return ErrInvalidProjectName
	}

	if len(name) < 3 {
		return ErrProjectNameTooShort
	}

	if len(name) > 32 {
		return ErrProjectNameTooLong
	}

	// Check for invalid characters
	for _, ch := range name {
		if !((ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-') {
			return ErrProjectNameInvalidChars
		}
	}

	// Check format (must start/end with letter/number, no consecutive hyphens)
	if !projectNameRegex.MatchString(name) {
		return ErrProjectNameInvalidFormat
	}

	// Check for consecutive hyphens
	if strings.Contains(name, "--") {
		return ErrProjectNameInvalidFormat
	}

	return nil
}

// NormalizeProjectName converts a user-provided name to a valid project name
// Examples:
//   - "My Project" -> "my-project"
//   - "My_Project_123" -> "my-project-123"
//   - "hello world!!!" -> "hello-world"
//   - "---test---" -> "test"
func NormalizeProjectName(name string) string {
	// Lowercase
	normalized := strings.ToLower(name)

	// Replace spaces, underscores, and multiple hyphens with single hyphen
	normalized = regexp.MustCompile(`[\s_]+`).ReplaceAllString(normalized, "-")

	// Remove invalid characters (keep only a-z, 0-9, -)
	normalized = regexp.MustCompile(`[^a-z0-9-]+`).ReplaceAllString(normalized, "")

	// Remove consecutive hyphens
	normalized = regexp.MustCompile(`-+`).ReplaceAllString(normalized, "-")

	// Remove leading/trailing hyphens
	normalized = strings.Trim(normalized, "-")

	// Ensure minimum length
	if len(normalized) < 3 {
		// If too short, generate a random suffix
		return fmt.Sprintf("%s-%d", normalized, rand.Intn(1000))
	}

	// Truncate if too long
	if len(normalized) > 32 {
		normalized = normalized[:32]
		normalized = strings.TrimRight(normalized, "-")
	}

	return normalized
}

// GenerateProjectName generates a random project name
// Format: adjective-noun-number (e.g., "happy-cloud-42")
func GenerateProjectName() string {
	adjectives := []string{
		"happy", "swift", "bright", "clever", "brave",
		"calm", "eager", "fair", "gentle", "jolly",
		"kind", "lively", "proud", "quiet", "wise",
		"bold", "cool", "witty", "warm", "noble",
	}

	nouns := []string{
		"cloud", "wave", "star", "moon", "sun",
		"tree", "river", "mountain", "ocean", "forest",
		"sky", "wind", "fire", "earth", "light",
		"shadow", "crystal", "diamond", "ruby", "pearl",
	}

	rand.Seed(time.Now().UnixNano())

	adjective := adjectives[rand.Intn(len(adjectives))]
	noun := nouns[rand.Intn(len(nouns))]
	number := rand.Intn(100)

	return fmt.Sprintf("%s-%s-%d", adjective, noun, number)
}

// GenerateUniqueProjectName generates a project name and ensures it's unique
// by checking against a list of existing names
func GenerateUniqueProjectName(existingNames []string) string {
	existingMap := make(map[string]bool)
	for _, name := range existingNames {
		existingMap[name] = true
	}

	// Try to generate a unique name (max 100 attempts)
	for i := 0; i < 100; i++ {
		name := GenerateProjectName()
		if !existingMap[name] {
			return name
		}
	}

	// Fallback: use timestamp-based name
	return fmt.Sprintf("vibespace-%d", time.Now().Unix())
}

// AllocatePorts returns port allocations for a vibespace.
// In Knative mode with Caddy routing (single-port architecture):
// - External port: Always 8080 (Caddy listens here)
// - Internal ports: 8081 (code-server), 3000 (preview), 3001 (prod)
//
// The basePort parameter is ignored in Knative mode but kept for API compatibility.
// All external traffic arrives at port 8080, where Caddy routes internally based on Host header.
// See ADR 0009 for architectural rationale.
//
// Returns external-facing ports (all 8080 in Knative mode).
func AllocatePorts(basePort int) Ports {
	// Return external ports (all services accessed via port 8080 + Caddy)
	// Caddy handles internal routing: 8080 → 8081 (code), 3000 (preview), 3001 (prod)
	return Ports{
		Code:    8080, // External-facing port (Caddy listens here)
		Preview: 8080, // External-facing port (same for all services)
		Prod:    8080, // External-facing port (same for all services)
	}
}

// GenerateURLs generates the 3 URLs for a vibespace based on project name
// Format:
//   - code.{project}.vibe.space
//   - preview.{project}.vibe.space
//   - prod.{project}.vibe.space
func GenerateURLs(projectName string) map[string]string {
	return map[string]string{
		"code":    fmt.Sprintf("http://code.%s.vibe.space", projectName),
		"preview": fmt.Sprintf("http://preview.%s.vibe.space", projectName),
		"prod":    fmt.Sprintf("http://prod.%s.vibe.space", projectName),
	}
}
