package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/vibespacehq/vibespace/pkg/permission"
	"github.com/vibespacehq/vibespace/pkg/ui"
)

// PermissionPrompt is a Bubble Tea component for displaying permission prompts.
type PermissionPrompt struct {
	request  *permission.Request
	selected int // 0=Allow, 1=Deny
	width    int
	height   int
}

// NewPermissionPrompt creates a new permission prompt for the given request.
func NewPermissionPrompt(req *permission.Request) *PermissionPrompt {
	return &PermissionPrompt{
		request:  req,
		selected: 1, // Default to Deny for safety
	}
}

// SetSize sets the prompt dimensions.
func (p *PermissionPrompt) SetSize(width, height int) {
	p.width = width
	p.height = height
}

// Request returns the permission request.
func (p *PermissionPrompt) Request() *permission.Request {
	return p.request
}

// Selected returns the currently selected option (0=Allow, 1=Deny).
func (p *PermissionPrompt) Selected() int {
	return p.selected
}

// MoveLeft moves selection to the left (Allow).
func (p *PermissionPrompt) MoveLeft() {
	p.selected = 0
}

// MoveRight moves selection to the right (Deny).
func (p *PermissionPrompt) MoveRight() {
	p.selected = 1
}

// GetDecision returns the decision based on current selection.
func (p *PermissionPrompt) GetDecision() permission.Decision {
	if p.selected == 0 {
		return permission.DecisionAllow
	}
	return permission.DecisionDeny
}

// View renders the permission prompt as a centered overlay.
func (p *PermissionPrompt) View() string {
	// Box dimensions
	boxWidth := 50
	if p.width > 60 && p.width < 100 {
		boxWidth = p.width - 10
	} else if p.width >= 100 {
		boxWidth = 60
	}

	// Styles
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.Orange).
		Padding(1, 2).
		Width(boxWidth)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.Orange)

	labelStyle := lipgloss.NewStyle().
		Foreground(ui.ColorDim)

	valueStyle := lipgloss.NewStyle().
		Foreground(ui.ColorWhite)

	allowButtonStyle := lipgloss.NewStyle().
		Padding(0, 2).
		Background(lipgloss.Color("#1a1a2e")).
		Foreground(ui.ColorWhite)

	denyButtonStyle := lipgloss.NewStyle().
		Padding(0, 2).
		Background(lipgloss.Color("#1a1a2e")).
		Foreground(ui.ColorWhite)

	selectedStyle := lipgloss.NewStyle().
		Padding(0, 2).
		Bold(true)

	hintStyle := lipgloss.NewStyle().
		Foreground(ui.ColorDim).
		Italic(true)

	// Build content
	var lines []string

	// Title
	lines = append(lines, titleStyle.Render("Permission Request"))
	lines = append(lines, "")

	// Agent
	lines = append(lines, fmt.Sprintf("%s %s",
		labelStyle.Render("Agent:"),
		valueStyle.Render(p.request.AgentKey)))

	// Tool
	lines = append(lines, fmt.Sprintf("%s %s",
		labelStyle.Render("Tool: "),
		valueStyle.Render(p.request.ToolName)))

	// Input (formatted)
	inputStr := p.formatToolInput()
	if inputStr != "" {
		// Wrap long input
		maxInputWidth := boxWidth - 10
		if len(inputStr) > maxInputWidth {
			inputStr = inputStr[:maxInputWidth-3] + "..."
		}
		lines = append(lines, fmt.Sprintf("%s %s",
			labelStyle.Render("Input:"),
			valueStyle.Render(inputStr)))
	}

	lines = append(lines, "")

	// Buttons
	allowBtn := " Allow "
	denyBtn := " Deny "

	if p.selected == 0 {
		activeAllow := selectedStyle
		allowBtn = activeAllow.
			Background(ui.Teal).
			Foreground(ui.ColorBlack).
			Render(" Allow ")
		denyBtn = denyButtonStyle.Render(denyBtn)
	} else {
		allowBtn = allowButtonStyle.Render(allowBtn)
		activeDeny := selectedStyle
		denyBtn = activeDeny.
			Background(ui.ColorError).
			Foreground(ui.ColorWhite).
			Render(" Deny ")
	}

	buttons := lipgloss.JoinHorizontal(lipgloss.Center, allowBtn, "  ", denyBtn)
	lines = append(lines, lipgloss.PlaceHorizontal(boxWidth-6, lipgloss.Center, buttons))

	lines = append(lines, "")

	// Hint
	hint := hintStyle.Render("← → to select, Enter to confirm")
	lines = append(lines, lipgloss.PlaceHorizontal(boxWidth-6, lipgloss.Center, hint))

	// Join all lines
	content := strings.Join(lines, "\n")
	box := borderStyle.Render(content)

	// Center the box on screen
	if p.width > 0 && p.height > 0 {
		return lipgloss.Place(p.width, p.height, lipgloss.Center, lipgloss.Center, box)
	}

	return box
}

// formatToolInput extracts the most relevant part of the tool input for display.
func (p *PermissionPrompt) formatToolInput() string {
	if len(p.request.ToolInput) == 0 {
		return ""
	}

	// Try to parse as JSON and extract relevant field
	switch p.request.ToolName {
	case "Bash":
		var input struct {
			Command string `json:"command"`
		}
		if json.Unmarshal(p.request.ToolInput, &input) == nil && input.Command != "" {
			return input.Command
		}

	case "Read", "Write", "Edit":
		var input struct {
			FilePath string `json:"file_path"`
		}
		if json.Unmarshal(p.request.ToolInput, &input) == nil && input.FilePath != "" {
			return input.FilePath
		}

	case "Glob":
		var input struct {
			Pattern string `json:"pattern"`
		}
		if json.Unmarshal(p.request.ToolInput, &input) == nil && input.Pattern != "" {
			return input.Pattern
		}

	case "Grep":
		var input struct {
			Pattern string `json:"pattern"`
			Path    string `json:"path"`
		}
		if json.Unmarshal(p.request.ToolInput, &input) == nil {
			if input.Path != "" {
				return fmt.Sprintf("%s in %s", input.Pattern, input.Path)
			}
			return input.Pattern
		}

	case "WebFetch":
		var input struct {
			URL string `json:"url"`
		}
		if json.Unmarshal(p.request.ToolInput, &input) == nil && input.URL != "" {
			return input.URL
		}

	case "WebSearch":
		var input struct {
			Query string `json:"query"`
		}
		if json.Unmarshal(p.request.ToolInput, &input) == nil && input.Query != "" {
			return input.Query
		}
	}

	// Fallback: show raw JSON (truncated)
	raw := string(p.request.ToolInput)
	if len(raw) > 50 {
		return raw[:47] + "..."
	}
	return raw
}
