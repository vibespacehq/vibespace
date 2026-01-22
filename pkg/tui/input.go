package tui

import (
	"fmt"
	"strings"

	"github.com/yagizdagabak/vibespace/pkg/session"
)

// Action represents a parsed user input action (SendAction or CommandAction)
type Action any

// SendAction represents a message to send to one or more agents
type SendAction struct {
	Targets []session.AgentAddress
	Message string
}

// CommandAction represents a TUI command
type CommandAction struct {
	Cmd  string
	Args []string
}

// ParseInput parses user input and returns the appropriate action
// Supports:
//   - @<agent> <message>           - Send to specific agent (uses default vibespace)
//   - @<agent>@<vibespace> <message> - Send to agent in specific vibespace
//   - @all <message>               - Broadcast to all agents
//   - @all@<vibespace> <message>   - Broadcast to all in one vibespace
//   - /<command> [args]            - TUI command
func ParseInput(input string, defaultVibespace string) (Action, error) {
	input = strings.TrimSpace(input)

	if input == "" {
		return nil, nil
	}

	// Handle commands
	if strings.HasPrefix(input, "/") {
		return parseCommand(input[1:])
	}

	// Handle @mentions
	if strings.HasPrefix(input, "@") {
		return parseMention(input[1:], defaultVibespace)
	}

	// Bare text - broadcast to all
	return SendAction{
		Targets: []session.AgentAddress{{Agent: "all", Vibespace: ""}},
		Message: input,
	}, nil
}

// parseCommand parses a /command
func parseCommand(input string) (Action, error) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	cmd := strings.ToLower(parts[0])
	args := parts[1:]

	return CommandAction{
		Cmd:  cmd,
		Args: args,
	}, nil
}

// parseMention parses an @mention
func parseMention(input string, defaultVibespace string) (Action, error) {
	// Split at first space to get target and message
	parts := strings.SplitN(input, " ", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("usage: @<agent> <message> or @all <message>")
	}

	target := parts[0]
	message := parts[1]

	if strings.TrimSpace(message) == "" {
		return nil, fmt.Errorf("message cannot be empty")
	}

	// Parse target
	targets, err := parseTarget(target, defaultVibespace)
	if err != nil {
		return nil, err
	}

	return SendAction{
		Targets: targets,
		Message: message,
	}, nil
}

// parseTarget parses a target specification
// Supports:
//   - "all" - all agents in all vibespaces
//   - "all@vibespace" - all agents in specific vibespace
//   - "agent" - specific agent (default vibespace)
//   - "agent@vibespace" - specific agent in specific vibespace
func parseTarget(target string, defaultVibespace string) ([]session.AgentAddress, error) {
	// Check for @vibespace suffix
	atIndex := strings.LastIndex(target, "@")

	var agent, vibespace string
	if atIndex > 0 {
		agent = target[:atIndex]
		vibespace = target[atIndex+1:]
	} else {
		agent = target
		vibespace = defaultVibespace
	}

	if agent == "" {
		return nil, fmt.Errorf("agent name cannot be empty")
	}

	// "all" is a special target
	if strings.ToLower(agent) == "all" {
		return []session.AgentAddress{{Agent: "all", Vibespace: vibespace}}, nil
	}

	return []session.AgentAddress{{Agent: agent, Vibespace: vibespace}}, nil
}
