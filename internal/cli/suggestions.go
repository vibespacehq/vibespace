package cli

// levenshtein calculates the Levenshtein distance between two strings
func levenshtein(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	// Create matrix
	matrix := make([][]int, len(a)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(b)+1)
		matrix[i][0] = i
	}
	for j := range matrix[0] {
		matrix[0][j] = j
	}

	// Fill matrix
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[len(a)][len(b)]
}

// knownVibespaceSubcommands is the list of known subcommands for a vibespace
var knownVibespaceSubcommands = []string{
	"agent",
	"connect",
	"exec",
	"config",
	"start",
	"stop",
	"forward",
}

// suggestVibespaceCommand suggests a command based on input typo
// Returns empty string if no good suggestion found
func suggestVibespaceCommand(input string) string {
	return suggestCommand(input, knownVibespaceSubcommands)
}

// suggestCommand finds the closest matching command using Levenshtein distance
// Returns empty string if no good suggestion found (distance > 3)
func suggestCommand(input string, commands []string) string {
	if len(input) == 0 {
		return ""
	}

	bestMatch := ""
	bestDistance := 4 // Threshold: only suggest if distance <= 3

	for _, cmd := range commands {
		distance := levenshtein(input, cmd)
		if distance < bestDistance {
			bestDistance = distance
			bestMatch = cmd
		}
	}

	return bestMatch
}
