package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/vibespacehq/vibespace/pkg/ui"
)

func (t *SessionsTab) View() string {
	// Wizard modes render full-width
	if t.mode == sessionsModeNewName || t.mode == sessionsModeNewVibespaces || t.mode == sessionsModeNewAgents {
		return t.viewWizard()
	}

	if len(t.flatTree) == 0 {
		if t.err != "" {
			return lipgloss.NewStyle().
				Foreground(ui.ColorError).
				Padding(1, 2).
				Render(fmt.Sprintf("Error: %s", t.err))
		}
		return t.viewEmpty()
	}

	// Split pane: left tree (~45%) + divider + right preview (~55%)
	leftWidth := t.width * 45 / 100
	if leftWidth < 30 {
		leftWidth = 30
	}
	rightWidth := t.width - leftWidth - 1 // 1 for divider

	contentHeight := t.height
	if t.mode == sessionsModeDelete || t.err != "" {
		contentHeight -= 2 // reserve space for prompt/error
	}
	if contentHeight < 3 {
		contentHeight = 3
	}

	left := t.viewTree(leftWidth, contentHeight)
	right := t.viewPreview(rightWidth, contentHeight)

	// Build divider column
	var divLines []string
	for i := 0; i < contentHeight; i++ {
		divLines = append(divLines, "│")
	}
	divider := lipgloss.NewStyle().Foreground(ui.ColorMuted).
		Render(strings.Join(divLines, "\n"))

	main := lipgloss.JoinHorizontal(lipgloss.Top, left, divider, right)

	var parts []string
	parts = append(parts, main)

	if t.mode == sessionsModeDelete && t.cursor < len(t.flatTree) && t.flatTree[t.cursor].Type == sessionItemMulti {
		name := t.flatTree[t.cursor].MultiSession.Name
		parts = append(parts, lipgloss.NewStyle().Foreground(ui.ColorWarning).Padding(0, 2).
			Render(fmt.Sprintf("Delete session %q? y to confirm, Esc to cancel", name)))
	}

	if t.err != "" && t.mode == sessionsModeList {
		parts = append(parts, lipgloss.NewStyle().Foreground(ui.ColorError).Padding(0, 2).Render(t.err))
	}

	return strings.Join(parts, "\n")
}

func (t *SessionsTab) viewEmpty() string {
	msg := lipgloss.NewStyle().
		Foreground(ui.ColorDim).
		Padding(2, 0).
		Render("No sessions yet.")
	hint := lipgloss.NewStyle().
		Foreground(ui.ColorDim).
		Render("Press n to create a new session.")
	block := lipgloss.JoinVertical(lipgloss.Center, msg, hint)
	return lipgloss.Place(t.width, t.height, lipgloss.Center, lipgloss.Center, block)
}

func (t *SessionsTab) viewTree(width, height int) string {
	var lines []string
	for i, item := range t.flatTree {
		lines = append(lines, t.renderTreeItem(item, i == t.cursor, width-4))
	}

	// Legend
	dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDim)
	lines = append(lines, "")
	lines = append(lines, dimStyle.Render("● single  ◆ multi"))

	content := strings.Join(lines, "\n")
	return lipgloss.NewStyle().
		Padding(1, 2).
		Width(width).
		Height(height).
		Render(content)
}

func (t *SessionsTab) renderTreeItem(item treeItem, selected bool, _ int) string {
	dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDim)

	switch item.Type {
	case sessionItemGroup:
		arrow := "▶ "
		if item.Expanded {
			arrow = "▼ "
		}

		var badge string
		if item.Loading {
			badge = " …"
		} else if !item.Expanded {
			var parts []string
			if item.SingleCount > 0 {
				parts = append(parts, fmt.Sprintf("%d●", item.SingleCount))
			}
			if item.MultiCount > 0 {
				parts = append(parts, fmt.Sprintf("%d◆", item.MultiCount))
			}
			if len(parts) > 0 {
				badge = " " + strings.Join(parts, " ")
			}
		}

		if selected {
			return renderGradientText(arrow+item.VSName+badge, getBrandGradient())
		}
		return dimStyle.Render(arrow) +
			lipgloss.NewStyle().Bold(true).Foreground(ui.ColorText).Render(item.VSName) +
			dimStyle.Render(badge)

	case sessionItemSingle:
		connector := "  ├── "
		if item.isLastChild {
			connector = "  └── "
		}
		marker := "● "
		age := formatSessionAge(item.Session.LastTime)
		shortID := item.Session.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		agentColor := t.agentColor(item.AgentName)

		if selected {
			return renderGradientText(connector+marker+item.AgentName+" "+shortID+" "+age, getBrandGradient())
		}
		return dimStyle.Render(connector) +
			lipgloss.NewStyle().Foreground(agentColor).Render(marker+item.AgentName) +
			" " + dimStyle.Render(shortID) +
			" " + lipgloss.NewStyle().Foreground(ui.ColorTimestamp).Render(age)

	case sessionItemMulti:
		connector := "  ├── "
		if item.isLastChild {
			connector = "  └── "
		}
		marker := "◆ "
		name := item.MultiSession.Name
		agents := sessAgentCount(*item.MultiSession)
		age := timeAgo(item.MultiSession.LastUsed)

		var crossVS string
		if item.CrossVSCount > 0 {
			crossVS = fmt.Sprintf(" +%dvs", item.CrossVSCount)
		}

		if selected {
			return renderGradientText(connector+marker+name+" "+agents+" agt"+crossVS+" "+age, getBrandGradient())
		}
		return dimStyle.Render(connector) +
			lipgloss.NewStyle().Foreground(ui.Orange).Render(marker+name) +
			" " + dimStyle.Render(agents+" agt"+crossVS) +
			" " + lipgloss.NewStyle().Foreground(ui.ColorTimestamp).Render(age)
	}

	return ""
}

func (t *SessionsTab) viewPreview(width, height int) string {
	if t.cursor >= len(t.flatTree) {
		return lipgloss.NewStyle().Width(width).Height(height).Render("")
	}

	item := t.flatTree[t.cursor]
	dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDim)
	labelStyle := lipgloss.NewStyle().Foreground(ui.ColorMuted)

	var lines []string

	switch item.Type {
	case sessionItemGroup:
		lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(ui.ColorText).Render(item.VSName))
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("%s  %s", labelStyle.Render("Status"), dimStyle.Render(item.VSStatus)))
		gs := t.groupStates[item.VSName]
		if gs != nil && gs.AgentsLoaded {
			lines = append(lines, fmt.Sprintf("%s  %s", labelStyle.Render("Agents"), dimStyle.Render(fmt.Sprintf("%d", len(gs.Agents)))))
			totalSingle := 0
			for _, sessions := range gs.AgentSessions {
				totalSingle += len(sessions)
			}
			lines = append(lines, fmt.Sprintf("%s  %s single, %d multi",
				labelStyle.Render("Sessions"),
				dimStyle.Render(fmt.Sprintf("%d", totalSingle)),
				item.MultiCount))
		} else if gs != nil && gs.Loading {
			lines = append(lines, dimStyle.Render("Loading agents..."))
		}

	case sessionItemSingle:
		sess := item.Session
		agentColor := t.agentColor(item.AgentName)
		lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(ui.ColorText).Render("Session"))
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("%s  %s", labelStyle.Render("ID"), dimStyle.Render(sess.ID)))
		lines = append(lines, fmt.Sprintf("%s  %s", labelStyle.Render("Agent"),
			lipgloss.NewStyle().Foreground(agentColor).Render(item.AgentName)+" "+dimStyle.Render("("+string(item.AgentType)+")")))
		lines = append(lines, fmt.Sprintf("%s  %s", labelStyle.Render("Prompts"), dimStyle.Render(fmt.Sprintf("%d", sess.Prompts))))
		lines = append(lines, fmt.Sprintf("%s  %s", labelStyle.Render("Last active"), dimStyle.Render(formatSessionAge(sess.LastTime))))

		if t.singlePreviewID == sess.ID && len(t.singlePreviewMsgs) > 0 {
			lines = append(lines, "")
			lines = append(lines, lipgloss.NewStyle().Italic(true).Foreground(ui.ColorMuted).Render("Recent Messages"))
			for _, m := range t.singlePreviewMsgs {
				ts := m.Time.Format("15:04")
				tsStr := lipgloss.NewStyle().Foreground(ui.ColorTimestamp).Render(ts)
				text := truncate(m.Text, 60)
				if m.Role == "user" {
					lines = append(lines, fmt.Sprintf("%s %s %s",
						tsStr,
						lipgloss.NewStyle().Bold(true).Foreground(ui.Teal).Render("You →"),
						dimStyle.Render(text)))
				} else {
					lines = append(lines, fmt.Sprintf("%s %s %s",
						tsStr,
						lipgloss.NewStyle().Bold(true).Foreground(agentColor).Render(item.AgentName),
						dimStyle.Render(text)))
				}
			}
		} else if t.singlePreviewID == sess.ID {
			lines = append(lines, "")
			lines = append(lines, lipgloss.NewStyle().Italic(true).Foreground(ui.ColorMuted).Render("No messages yet."))
		}

	case sessionItemMulti:
		ms := item.MultiSession
		lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(ui.ColorText).Render(ms.Name))
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("%s  %s", labelStyle.Render("Vibespaces"), dimStyle.Render(sessVibespaceNames(*ms))))
		lines = append(lines, fmt.Sprintf("%s  %s", labelStyle.Render("Agents"), dimStyle.Render(sessAgentCount(*ms))))
		lines = append(lines, fmt.Sprintf("%s  %s", labelStyle.Render("Created"), dimStyle.Render(ms.CreatedAt.Format("2006-01-02 15:04:05"))))
		lines = append(lines, fmt.Sprintf("%s  %s", labelStyle.Render("Last used"), dimStyle.Render(timeAgo(ms.LastUsed))))

		if t.previewSessionName == ms.Name && t.previewTotal > 0 {
			lines = append(lines, fmt.Sprintf("%s  %s", labelStyle.Render("Messages"), dimStyle.Render(fmt.Sprintf("%d", t.previewTotal))))
		}

		if t.previewSessionName == ms.Name && len(t.previewMsgs) > 0 {
			lines = append(lines, "")
			lines = append(lines, lipgloss.NewStyle().Italic(true).Foreground(ui.ColorMuted).Render("Recent Messages"))

			agentColorMap := make(map[string]lipgloss.Color)
			agentIdx := 0
			for _, m := range t.previewMsgs {
				if m.Type != MessageTypeUser && m.Sender != "" {
					if _, ok := agentColorMap[m.Sender]; !ok {
						agentColorMap[m.Sender] = ui.GetAgentColor(agentIdx)
						agentIdx++
					}
				}
			}

			for _, m := range t.previewMsgs {
				ts := m.Timestamp.Format("15:04")
				tsStr := lipgloss.NewStyle().Foreground(ui.ColorTimestamp).Render(ts)
				switch m.Type {
				case MessageTypeUser:
					lines = append(lines, fmt.Sprintf("%s %s %s",
						tsStr,
						lipgloss.NewStyle().Bold(true).Foreground(ui.Teal).Render("You →"),
						dimStyle.Render(truncate(m.Content, 60))))
				case MessageTypeAssistant:
					lines = append(lines, fmt.Sprintf("%s %s %s",
						tsStr,
						lipgloss.NewStyle().Bold(true).Foreground(agentColorMap[m.Sender]).Render(m.Sender),
						dimStyle.Render(truncate(m.Content, 60))))
				case MessageTypeToolUse:
					lines = append(lines, fmt.Sprintf("%s %s %s",
						tsStr,
						lipgloss.NewStyle().Bold(true).Foreground(agentColorMap[m.Sender]).Render(m.Sender),
						lipgloss.NewStyle().Foreground(ui.Orange).Render(fmt.Sprintf("[%s] %s", m.ToolName, truncate(m.ToolInput, 40)))))
				}
			}
		} else if t.previewSessionName == ms.Name {
			lines = append(lines, "")
			lines = append(lines, lipgloss.NewStyle().Italic(true).Foreground(ui.ColorMuted).Render("No messages yet."))
		}
	}

	content := strings.Join(lines, "\n")
	return lipgloss.NewStyle().
		Padding(1, 2).
		Width(width).
		Height(height).
		Render(content)
}

func (t *SessionsTab) viewWizard() string {
	style := lipgloss.NewStyle().Padding(1, 2)
	dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDim)
	labelStyle := lipgloss.NewStyle().Foreground(ui.Teal).Bold(true)
	hintStyle := lipgloss.NewStyle().Foreground(ui.ColorMuted).Italic(true)

	var content string

	switch t.mode {
	case sessionsModeNewName:
		step := dimStyle.Render("Step 1/3")
		label := labelStyle.Render("Session name: ")
		hint := hintStyle.Render("  enter to skip (auto-generated)")
		content = step + "  " + label + t.nameInput.View() + hint
		if t.err != "" {
			content += "\n" + lipgloss.NewStyle().Foreground(ui.ColorError).Render(t.err)
		}

	case sessionsModeNewVibespaces:
		step := dimStyle.Render("Step 2/3")
		header := labelStyle.Render(fmt.Sprintf("Select vibespaces for %q", t.newSessionName))

		var lines []string
		lines = append(lines, step+"  "+header)

		if len(t.newVibespaces) == 0 {
			lines = append(lines, "  "+hintStyle.Render("No vibespaces found. Press enter to create an empty session."))
		} else {
			for i, vs := range t.newVibespaces {
				cursor := "  "
				if i == t.newVSCursor {
					cursor = renderGradientText("› ", getBrandGradient())
				}
				check := "[ ]"
				if t.newVSSelected[i] {
					check = "[x]"
				}
				status := dimStyle.Render(vs.Status)
				name := vs.Name
				if i == t.newVSCursor {
					name = lipgloss.NewStyle().Bold(true).Foreground(ui.ColorWhite).Render(name)
				} else {
					name = dimStyle.Render(name)
				}
				lines = append(lines, fmt.Sprintf("  %s%s %s  %s", cursor, check, name, status))
			}
		}

		lines = append(lines, "  "+hintStyle.Render("x toggle  enter confirm  enter with none to skip"))
		if t.err != "" {
			lines = append(lines, lipgloss.NewStyle().Foreground(ui.ColorError).Render(t.err))
		}
		content = strings.Join(lines, "\n")

	case sessionsModeNewAgents:
		vsName := ""
		if t.newAgentVSIndex < len(t.newSelectedVS) {
			vsName = t.newSelectedVS[t.newAgentVSIndex].Name
		}
		step := dimStyle.Render("Step 3/3")
		header := labelStyle.Render(fmt.Sprintf("Select agents from %q", vsName))

		var lines []string
		lines = append(lines, step+"  "+header)

		if len(t.newAgents) == 0 {
			lines = append(lines, "  "+hintStyle.Render("Loading agents..."))
		} else {
			for i, a := range t.newAgents {
				cursor := "  "
				if i == t.newAgentCursor {
					cursor = renderGradientText("› ", getBrandGradient())
				}
				check := "[ ]"
				if t.newAgentSelected[i] {
					check = "[x]"
				}
				status := dimStyle.Render(a.Status)
				agentType := dimStyle.Render(a.AgentType)
				name := a.Name
				if i == t.newAgentCursor {
					name = lipgloss.NewStyle().Bold(true).Foreground(ui.ColorWhite).Render(name)
				} else {
					name = dimStyle.Render(name)
				}
				lines = append(lines, fmt.Sprintf("  %s%s %s  %s  %s", cursor, check, name, agentType, status))
			}
		}

		lines = append(lines, "  "+hintStyle.Render("x toggle  enter confirm  enter with none for all agents"))
		if t.err != "" {
			lines = append(lines, lipgloss.NewStyle().Foreground(ui.ColorError).Render(t.err))
		}
		content = strings.Join(lines, "\n")
	}

	return style.Render(content)
}
