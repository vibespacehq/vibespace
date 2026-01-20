package tui

import (
	"fmt"
	"strings"

	"vibespace/pkg/session"
)

// Action represents a parsed user input action
type Action interface {
	isAction()
}

// SendAction represents a message to send to one or more agents
type SendAction struct {
	Targets []session.AgentAddress
	Message string
}

func (SendAction) isAction() {}

// CommandAction represents a TUI command
type CommandAction struct {
	Cmd  string
	Args []string
}

func (CommandAction) isAction() {}

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

// IsQuitCommand checks if the action is a quit command
func IsQuitCommand(action Action) bool {
	if cmd, ok := action.(CommandAction); ok {
		switch cmd.Cmd {
		case "quit", "exit", "q":
			return true
		}
	}
	return false
}

// IsFocusCommand checks if the action is a focus command
func IsFocusCommand(action Action) (bool, string) {
	if cmd, ok := action.(CommandAction); ok {
		if cmd.Cmd == "focus" && len(cmd.Args) > 0 {
			return true, cmd.Args[0]
		}
	}
	return false, ""
}

// IsSplitCommand checks if the action is a split command
func IsSplitCommand(action Action) bool {
	if cmd, ok := action.(CommandAction); ok {
		return cmd.Cmd == "split"
	}
	return false
}

// IsListCommand checks if the action is a list command
func IsListCommand(action Action) bool {
	if cmd, ok := action.(CommandAction); ok {
		return cmd.Cmd == "list" || cmd.Cmd == "ls"
	}
	return false
}

// IsHelpCommand checks if the action is a help command
func IsHelpCommand(action Action) bool {
	if cmd, ok := action.(CommandAction); ok {
		return cmd.Cmd == "help" || cmd.Cmd == "h" || cmd.Cmd == "?"
	}
	return false
}

// IsSaveCommand checks if the action is a save command
func IsSaveCommand(action Action) (bool, string) {
	if cmd, ok := action.(CommandAction); ok {
		if cmd.Cmd == "save" {
			name := ""
			if len(cmd.Args) > 0 {
				name = cmd.Args[0]
			}
			return true, name
		}
	}
	return false, ""
}

// IsAddCommand checks if the action is an add command
func IsAddCommand(action Action) (bool, string, string) {
	if cmd, ok := action.(CommandAction); ok {
		if cmd.Cmd == "add" && len(cmd.Args) >= 1 {
			vibespace := cmd.Args[0]
			agent := ""
			if len(cmd.Args) >= 2 {
				agent = cmd.Args[1]
			}
			return true, vibespace, agent
		}
	}
	return false, "", ""
}

// IsRemoveCommand checks if the action is a remove command
func IsRemoveCommand(action Action) (bool, string) {
	if cmd, ok := action.(CommandAction); ok {
		if (cmd.Cmd == "remove" || cmd.Cmd == "rm") && len(cmd.Args) > 0 {
			return true, cmd.Args[0]
		}
	}
	return false, ""
}

// IsPortsCommand checks if the action is a ports command
func IsPortsCommand(action Action) bool {
	if cmd, ok := action.(CommandAction); ok {
		return cmd.Cmd == "ports"
	}
	return false
}

// IsForwardCommand checks if the action is a forward command
func IsForwardCommand(action Action) (bool, string, string) {
	if cmd, ok := action.(CommandAction); ok {
		if cmd.Cmd == "forward" && len(cmd.Args) >= 1 {
			port := cmd.Args[0]
			vibespace := ""
			if len(cmd.Args) >= 2 {
				vibespace = cmd.Args[1]
			}
			return true, port, vibespace
		}
	}
	return false, "", ""
}

// IsOpenCommand checks if the action is an open command
func IsOpenCommand(action Action) (bool, string, string) {
	if cmd, ok := action.(CommandAction); ok {
		if cmd.Cmd == "open" && len(cmd.Args) >= 1 {
			port := cmd.Args[0]
			vibespace := ""
			if len(cmd.Args) >= 2 {
				vibespace = cmd.Args[1]
			}
			return true, port, vibespace
		}
	}
	return false, "", ""
}
