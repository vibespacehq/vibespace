package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vibespacehq/vibespace/pkg/agent"
	"github.com/vibespacehq/vibespace/pkg/daemon"
	"github.com/vibespacehq/vibespace/pkg/ui"
)

func (t *VibespacesTab) createFieldVisible(f createFormField) bool {
	switch f {
	case createFieldWorktree:
		return t.createRepo != ""
	case createFieldBranch:
		return t.createWorktree
	default:
		return true
	}
}

func (t *VibespacesTab) createFieldNext() {
	for {
		t.createField++
		if t.createField >= createFieldCount || t.createFieldVisible(t.createField) {
			return
		}
	}
}

func (t *VibespacesTab) createFieldPrev() {
	for {
		if t.createField <= 0 {
			return
		}
		t.createField--
		if t.createFieldVisible(t.createField) {
			return
		}
	}
}

func (t *VibespacesTab) handleCreateFormKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	switch {
	case k == "esc":
		t.mode = vibespacesModeList
		return t, nil

	case k == "ctrl+s":
		if t.createName == "" {
			return t, nil
		}
		return t, t.submitCreateForm()

	case k == "tab":
		t.createFieldNext()
		if t.createField >= createFieldCount {
			if t.createName == "" {
				t.createField = createFieldName
				return t, nil
			}
			return t, t.submitCreateForm()
		}
		return t, nil

	case k == "enter":
		if t.createField == createFieldName && t.createName == "" {
			return t, nil
		}
		t.createFieldNext()
		if t.createField >= createFieldCount {
			return t, t.submitCreateForm()
		}
		return t, nil

	case k == "down":
		if t.createField < createFieldCount-1 {
			t.createFieldNext()
		}
		return t, nil

	case k == "up":
		t.createFieldPrev()
		return t, nil

	case k == "backspace":
		switch t.createField {
		case createFieldName:
			if len(t.createName) > 0 {
				t.createName = t.createName[:len(t.createName)-1]
			}
		case createFieldRepo:
			if len(t.createRepo) > 0 {
				t.createRepo = t.createRepo[:len(t.createRepo)-1]
			}
			// When repo is cleared, reset worktree state
			if t.createRepo == "" {
				t.createWorktree = false
				t.createBranch = ""
			}
		case createFieldBranch:
			if len(t.createBranch) > 0 {
				t.createBranch = t.createBranch[:len(t.createBranch)-1]
			}
		case createFieldCPU:
			if len(t.createCPU) > 0 {
				t.createCPU = t.createCPU[:len(t.createCPU)-1]
			}
		case createFieldMemory:
			if len(t.createMemory) > 0 {
				t.createMemory = t.createMemory[:len(t.createMemory)-1]
			}
		case createFieldStorage:
			if len(t.createStorage) > 0 {
				t.createStorage = t.createStorage[:len(t.createStorage)-1]
			}
		}
		return t, nil

	default:
		if t.createField == createFieldAgentType {
			if k == "j" || k == "k" {
				if t.createAgentType == agent.TypeClaudeCode {
					t.createAgentType = agent.TypeCodex
				} else {
					t.createAgentType = agent.TypeClaudeCode
				}
			}
			return t, nil
		}

		if t.createField == createFieldWorktree {
			if k == "j" || k == "k" {
				t.createWorktree = !t.createWorktree
				if !t.createWorktree {
					t.createBranch = ""
				}
			}
			return t, nil
		}

		if msg.Type == tea.KeyRunes {
			text := string(msg.Runes)
			switch t.createField {
			case createFieldName:
				t.createName += text
			case createFieldRepo:
				t.createRepo += text
			case createFieldBranch:
				t.createBranch += text
			case createFieldCPU:
				t.createCPU += text
			case createFieldMemory:
				t.createMemory += text
			case createFieldStorage:
				t.createStorage += text
			}
		}
		return t, nil
	}
}

func (t *VibespacesTab) handleDeleteConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	switch {
	case k == "esc":
		t.mode = vibespacesModeList
		return t, nil

	case k == "enter":
		if t.deleteInput == t.deleteName {
			return t, t.submitDelete()
		}
		return t, nil

	case k == "backspace":
		if len(t.deleteInput) > 0 {
			t.deleteInput = t.deleteInput[:len(t.deleteInput)-1]
		}
		return t, nil

	default:
		if msg.Type == tea.KeyRunes {
			t.deleteInput += string(msg.Runes)
		}
		return t, nil
	}
}

func (t *VibespacesTab) handleDeleteAgentConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	switch {
	case k == "esc":
		t.mode = vibespacesModeAgentView
		return t, nil

	case k == "enter":
		if t.deleteAgentInput == t.deleteAgentName {
			t.mode = vibespacesModeAgentView
			return t, t.deleteAgent(t.selectedVS.Name, t.deleteAgentName)
		}
		return t, nil

	case k == "backspace":
		if len(t.deleteAgentInput) > 0 {
			t.deleteAgentInput = t.deleteAgentInput[:len(t.deleteAgentInput)-1]
		}
		return t, nil

	default:
		if msg.Type == tea.KeyRunes {
			t.deleteAgentInput += string(msg.Runes)
		}
		return t, nil
	}
}

func (t *VibespacesTab) handleAddAgentKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	switch {
	case k == "esc":
		t.mode = vibespacesModeAgentView
		return t, nil

	case k == "ctrl+s":
		return t, t.submitAddAgent()

	case k == "tab", k == "enter":
		t.addAgentField++
		// Skip branch field when vibespace has no worktree
		if t.addAgentField == addAgentFieldBranch && (t.selectedVS == nil || !t.selectedVS.Worktree) {
			t.addAgentField++
		}
		if t.addAgentField >= addAgentFieldCount {
			return t, t.submitAddAgent()
		}
		// Reset tools cursor when entering a multi-select field
		if t.addAgentField == addAgentFieldAllowedTools || t.addAgentField == addAgentFieldDisallowedTools {
			t.addAgentToolsCursor = 0
		}
		return t, nil

	case k == "down":
		if t.addAgentField < addAgentFieldCount-1 {
			t.addAgentField++
			// Skip branch field when vibespace has no worktree
			if t.addAgentField == addAgentFieldBranch && (t.selectedVS == nil || !t.selectedVS.Worktree) {
				t.addAgentField++
			}
			if t.addAgentField == addAgentFieldAllowedTools || t.addAgentField == addAgentFieldDisallowedTools {
				t.addAgentToolsCursor = 0
			}
		}
		return t, nil

	case k == "up":
		if t.addAgentField > 0 {
			t.addAgentField--
			// Skip branch field when vibespace has no worktree
			if t.addAgentField == addAgentFieldBranch && (t.selectedVS == nil || !t.selectedVS.Worktree) {
				t.addAgentField--
			}
			if t.addAgentField == addAgentFieldAllowedTools || t.addAgentField == addAgentFieldDisallowedTools {
				t.addAgentToolsCursor = 0
			}
		}
		return t, nil

	case k == "backspace":
		t.addAgentBackspace()
		return t, nil

	default:
		// Selector fields: j/k to toggle value
		switch t.addAgentField {
		case addAgentFieldType:
			if k == "j" || k == "k" {
				if t.addAgentType == agent.TypeClaudeCode {
					t.addAgentType = agent.TypeCodex
				} else {
					t.addAgentType = agent.TypeClaudeCode
				}
				// Refresh tools list for the new agent type and reset selections
				t.addAgentToolsList = agentSupportedTools(t.addAgentType)
				t.addAgentAllowedSet = make(map[string]bool)
				t.addAgentDisallowedSet = make(map[string]bool)
				t.addAgentToolsCursor = 0
			}
			return t, nil
		case addAgentFieldShareCreds:
			if k == "j" || k == "k" {
				t.addAgentShareCreds = !t.addAgentShareCreds
			}
			return t, nil
		case addAgentFieldSkipPerms:
			if k == "j" || k == "k" {
				t.addAgentSkipPerms = !t.addAgentSkipPerms
			}
			return t, nil

		// Multi-select fields: j/k navigate, space toggles
		case addAgentFieldAllowedTools:
			return t, t.handleToolsMultiSelect(k, t.addAgentAllowedSet, t.addAgentDisallowedSet)
		case addAgentFieldDisallowedTools:
			return t, t.handleToolsMultiSelect(k, t.addAgentDisallowedSet, t.addAgentAllowedSet)
		}

		// Text fields
		if msg.Type == tea.KeyRunes {
			text := string(msg.Runes)
			switch t.addAgentField {
			case addAgentFieldName:
				t.addAgentName += text
			case addAgentFieldBranch:
				t.addAgentBranch += text
			case addAgentFieldModel:
				t.addAgentModel += text
			case addAgentFieldMaxTurns:
				t.addAgentMaxTurns += text
			}
		}
		return t, nil
	}
}

func (t *VibespacesTab) handleToolsMultiSelect(k string, set, oppositeSet map[string]bool) tea.Cmd {
	return t.handleToolsMultiSelectWith(k, t.addAgentToolsList, &t.addAgentToolsCursor, set, oppositeSet)
}

// handleToolsMultiSelectWith is the generic helper for tool multi-select navigation.
// oppositeSet is the mutually exclusive set — toggling a tool on removes it from the opposite set.
func (t *VibespacesTab) handleToolsMultiSelectWith(k string, toolsList []string, cursor *int, set, oppositeSet map[string]bool) tea.Cmd {
	n := len(toolsList)
	if n == 0 {
		return nil
	}
	switch k {
	case "j", "down":
		*cursor = min(*cursor+1, n-1)
	case "k", "up":
		*cursor = max(*cursor-1, 0)
	case " ":
		tool := toolsList[*cursor]
		if set[tool] {
			delete(set, tool)
		} else {
			set[tool] = true
			delete(oppositeSet, tool)
		}
	}
	return nil
}

func (t *VibespacesTab) addAgentBackspace() {
	switch t.addAgentField {
	case addAgentFieldName:
		if len(t.addAgentName) > 0 {
			t.addAgentName = t.addAgentName[:len(t.addAgentName)-1]
		}
	case addAgentFieldBranch:
		if len(t.addAgentBranch) > 0 {
			t.addAgentBranch = t.addAgentBranch[:len(t.addAgentBranch)-1]
		}
	case addAgentFieldModel:
		if len(t.addAgentModel) > 0 {
			t.addAgentModel = t.addAgentModel[:len(t.addAgentModel)-1]
		}
	case addAgentFieldMaxTurns:
		if len(t.addAgentMaxTurns) > 0 {
			t.addAgentMaxTurns = t.addAgentMaxTurns[:len(t.addAgentMaxTurns)-1]
		}
	}
}

func (t *VibespacesTab) handleEditConfigKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	switch {
	case k == "esc":
		t.mode = vibespacesModeAgentView
		return t, nil

	case k == "ctrl+s":
		return t, t.submitEditConfig()

	case k == "tab", k == "enter":
		t.editConfigField++
		if t.editConfigField >= editConfigFieldCount {
			return t, t.submitEditConfig()
		}
		// Reset tools cursor when entering a multi-select field
		if t.editConfigField == editConfigFieldAllowedTools || t.editConfigField == editConfigFieldDisallowedTools {
			t.editConfigToolsCursor = 0
		}
		return t, nil

	case k == "down":
		if t.editConfigField < editConfigFieldCount-1 {
			t.editConfigField++
			if t.editConfigField == editConfigFieldAllowedTools || t.editConfigField == editConfigFieldDisallowedTools {
				t.editConfigToolsCursor = 0
			}
		}
		return t, nil

	case k == "up":
		if t.editConfigField > 0 {
			t.editConfigField--
			if t.editConfigField == editConfigFieldAllowedTools || t.editConfigField == editConfigFieldDisallowedTools {
				t.editConfigToolsCursor = 0
			}
		}
		return t, nil

	case k == "backspace":
		switch t.editConfigField {
		case editConfigFieldModel:
			if len(t.editConfigModel) > 0 {
				t.editConfigModel = t.editConfigModel[:len(t.editConfigModel)-1]
			}
		case editConfigFieldMaxTurns:
			if len(t.editConfigMaxTurns) > 0 {
				t.editConfigMaxTurns = t.editConfigMaxTurns[:len(t.editConfigMaxTurns)-1]
			}
		}
		return t, nil

	default:
		switch t.editConfigField {
		case editConfigFieldSkipPerms:
			if k == "j" || k == "k" {
				t.editConfigSkipPerms = !t.editConfigSkipPerms
			}
			return t, nil
		case editConfigFieldAllowedTools:
			return t, t.handleToolsMultiSelectWith(k, t.editConfigToolsList, &t.editConfigToolsCursor, t.editConfigAllowedSet, t.editConfigDisallowedSet)
		case editConfigFieldDisallowedTools:
			return t, t.handleToolsMultiSelectWith(k, t.editConfigToolsList, &t.editConfigToolsCursor, t.editConfigDisallowedSet, t.editConfigAllowedSet)
		}

		// Text fields
		if msg.Type == tea.KeyRunes {
			text := string(msg.Runes)
			switch t.editConfigField {
			case editConfigFieldModel:
				t.editConfigModel += text
			case editConfigFieldMaxTurns:
				t.editConfigMaxTurns += text
			}
		}
		return t, nil
	}
}

func (t *VibespacesTab) handleForwardManagerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if t.fwdManagerAdding {
		return t.handleFwdManagerAddKey(msg)
	}

	k := msg.String()
	switch k {
	case "esc", "backspace":
		t.mode = vibespacesModeAgentView
		return t, nil
	case "j", "down":
		fwds := t.currentAgentForwards()
		if len(fwds) > 0 {
			t.fwdManagerCursor = min(t.fwdManagerCursor+1, len(fwds)-1)
		}
	case "k", "up":
		if t.fwdManagerCursor > 0 {
			t.fwdManagerCursor--
		}
	case "a":
		t.fwdManagerAdding = true
		t.fwdManagerAddRemote = ""
		t.fwdManagerAddLocal = ""
		t.fwdManagerAddDNS = false
		t.fwdManagerAddDNSName = ""
		t.fwdManagerAddField = fwdManagerAddFieldRemote
		t.err = ""
	case "d":
		fwds := t.currentAgentForwards()
		if t.fwdManagerCursor < len(fwds) {
			fwd := fwds[t.fwdManagerCursor]
			return t, t.submitRemoveForward(fwd.RemotePort)
		}
	case "n":
		fwds := t.currentAgentForwards()
		if t.fwdManagerCursor < len(fwds) {
			fwd := fwds[t.fwdManagerCursor]
			return t, t.submitToggleDNS(fwd.RemotePort, fwd.DNSName)
		}
	case "r":
		if t.selectedVS != nil {
			return t, t.loadForwards(t.selectedVS.ID, t.selectedVS.Name)
		}
	}
	return t, nil
}

func (t *VibespacesTab) handleFwdManagerAddKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	switch {
	case k == "esc":
		t.fwdManagerAdding = false
		return t, nil

	case k == "ctrl+s":
		return t, t.submitAddForward()

	case k == "enter", k == "tab":
		// On the DNS toggle field, Enter/Tab advances without toggling
		if t.fwdManagerAddField == fwdManagerAddFieldDNS {
			t.fwdManagerAddField++
			// Skip DNS name field if DNS is disabled
			if !t.fwdManagerAddDNS && t.fwdManagerAddField == fwdManagerAddFieldDNSName {
				return t, t.submitAddForward()
			}
			return t, nil
		}
		t.fwdManagerAddField++
		// Skip DNS name field if DNS is disabled
		if t.fwdManagerAddField == fwdManagerAddFieldDNSName && !t.fwdManagerAddDNS {
			t.fwdManagerAddField++
		}
		if t.fwdManagerAddField >= fwdManagerAddFieldCount {
			return t, t.submitAddForward()
		}
		return t, nil

	case k == "down":
		if t.fwdManagerAddField < fwdManagerAddFieldCount-1 {
			t.fwdManagerAddField++
			// Skip DNS name if DNS disabled
			if t.fwdManagerAddField == fwdManagerAddFieldDNSName && !t.fwdManagerAddDNS {
				t.fwdManagerAddField--
			}
		}
		return t, nil

	case k == "up":
		if t.fwdManagerAddField > 0 {
			t.fwdManagerAddField--
			// Skip DNS name if DNS disabled
			if t.fwdManagerAddField == fwdManagerAddFieldDNSName && !t.fwdManagerAddDNS {
				t.fwdManagerAddField--
			}
		}
		return t, nil

	case k == " ":
		// Space toggles the DNS bool field
		if t.fwdManagerAddField == fwdManagerAddFieldDNS {
			t.fwdManagerAddDNS = !t.fwdManagerAddDNS
			return t, nil
		}
		return t, nil

	case k == "backspace":
		switch t.fwdManagerAddField {
		case fwdManagerAddFieldRemote:
			if len(t.fwdManagerAddRemote) > 0 {
				t.fwdManagerAddRemote = t.fwdManagerAddRemote[:len(t.fwdManagerAddRemote)-1]
			}
		case fwdManagerAddFieldLocal:
			if len(t.fwdManagerAddLocal) > 0 {
				t.fwdManagerAddLocal = t.fwdManagerAddLocal[:len(t.fwdManagerAddLocal)-1]
			}
		case fwdManagerAddFieldDNSName:
			if len(t.fwdManagerAddDNSName) > 0 {
				t.fwdManagerAddDNSName = t.fwdManagerAddDNSName[:len(t.fwdManagerAddDNSName)-1]
			}
		}
		return t, nil

	default:
		if msg.Type == tea.KeyRunes {
			text := string(msg.Runes)
			switch t.fwdManagerAddField {
			case fwdManagerAddFieldRemote:
				t.fwdManagerAddRemote += text
			case fwdManagerAddFieldLocal:
				t.fwdManagerAddLocal += text
			case fwdManagerAddFieldDNSName:
				t.fwdManagerAddDNSName += text
			}
		}
		return t, nil
	}
}

func (t *VibespacesTab) currentAgentForwards() []daemon.ForwardInfo {
	if t.agentCursor >= len(t.viewAgents) {
		return nil
	}
	return t.forwardsForAgent(t.viewAgents[t.agentCursor].AgentName)
}

func (t *VibespacesTab) viewCreateForm() string {
	dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDim)
	labelStyle := lipgloss.NewStyle().Foreground(ui.ColorMuted)
	activeStyle := lipgloss.NewStyle().Foreground(ui.ColorText)
	mutedLine := lipgloss.NewStyle().Foreground(ui.ColorMuted).
		Render(strings.Repeat("─", t.width-4))

	header := lipgloss.NewStyle().Italic(true).Foreground(ui.Orange).
		Render("Create vibespace")

	boolStr := func(v bool) string {
		if v {
			return "yes"
		}
		return "no"
	}

	type formField struct {
		label    string
		field    createFormField
		value    string
		isSelect bool
	}

	fields := []formField{
		{"Name", createFieldName, t.createName, false},
		{"Agent type", createFieldAgentType, string(t.createAgentType), true},
		{"Repo", createFieldRepo, t.createRepo, false},
	}
	// Only show worktree when repo is set
	if t.createRepo != "" {
		fields = append(fields, formField{"Worktree", createFieldWorktree, boolStr(t.createWorktree), true})
	}
	// Only show branch when worktree is enabled
	if t.createWorktree {
		fields = append(fields, formField{"Branch", createFieldBranch, t.createBranch, false})
	}
	fields = append(fields,
		formField{"CPU", createFieldCPU, t.createCPU, false},
		formField{"Memory", createFieldMemory, t.createMemory, false},
		formField{"Storage", createFieldStorage, t.createStorage, false},
	)

	var lines []string
	for _, f := range fields {
		label := fmt.Sprintf("%-12s", f.label)
		isActive := f.field == t.createField

		var val string
		if isActive {
			if f.isSelect {
				val = activeStyle.Render("["+f.value+"]") + "  " + dimStyle.Render("j/k to change")
			} else {
				val = activeStyle.Render(f.value + "█")
			}
		} else {
			if f.value == "" && !f.isSelect {
				val = dimStyle.Render("(optional)")
			} else {
				val = dimStyle.Render(f.value)
			}
		}

		lines = append(lines, fmt.Sprintf("  %s %s", labelStyle.Render(label), val))
	}

	if t.err != "" {
		lines = append(lines, "", lipgloss.NewStyle().Foreground(ui.ColorError).Render("  "+t.err))
	}

	fullBlock := header + "\n" + mutedLine + "\n" + strings.Join(lines, "\n") + "\n" + mutedLine
	return lipgloss.NewStyle().Padding(0, 2).Render(fullBlock)
}

func (t *VibespacesTab) viewGithubAuth() string {
	dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDim)
	activeStyle := lipgloss.NewStyle().Foreground(ui.ColorText)
	mutedLine := lipgloss.NewStyle().Foreground(ui.ColorMuted).
		Render(strings.Repeat("─", t.width-4))

	header := lipgloss.NewStyle().Italic(true).Foreground(ui.Orange).
		Render("Authorize GitHub Access")

	var lines []string
	lines = append(lines, fmt.Sprintf("  %-12s %s", dimStyle.Render("Open:"), activeStyle.Render(t.githubVerifyURI)))
	lines = append(lines, fmt.Sprintf("  %-12s %s", dimStyle.Render("Code:"), lipgloss.NewStyle().Bold(true).Foreground(ui.Orange).Render(t.githubUserCode)))
	lines = append(lines, "")
	lines = append(lines, "  "+dimStyle.Render("Waiting for authorization..."))
	lines = append(lines, "")
	lines = append(lines, "  "+dimStyle.Render("[Esc] Cancel"))

	fullBlock := header + "\n" + mutedLine + "\n" + strings.Join(lines, "\n") + "\n" + mutedLine
	return lipgloss.NewStyle().Padding(0, 2).Render(fullBlock)
}

func (t *VibespacesTab) viewDeleteConfirm() string {
	dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDim)
	activeStyle := lipgloss.NewStyle().Foreground(ui.ColorText)
	mutedLine := lipgloss.NewStyle().Foreground(ui.ColorMuted).
		Render(strings.Repeat("─", t.width-4))

	header := lipgloss.NewStyle().Italic(true).Foreground(ui.ColorError).
		Render(fmt.Sprintf("Delete \"%s\"?", t.deleteName))

	prompt := fmt.Sprintf("  Type %s to confirm: %s",
		dimStyle.Render(t.deleteName),
		activeStyle.Render(t.deleteInput+"█"))

	var errLine string
	if t.err != "" {
		errLine = "\n" + lipgloss.NewStyle().Foreground(ui.ColorError).Render("  "+t.err)
	}

	fullBlock := header + "\n" + mutedLine + "\n" + prompt + errLine + "\n" + mutedLine
	return lipgloss.NewStyle().Padding(0, 2).Render(fullBlock)
}

func (t *VibespacesTab) viewDeleteAgentConfirm() string {
	dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDim)
	activeStyle := lipgloss.NewStyle().Foreground(ui.ColorText)
	mutedLine := lipgloss.NewStyle().Foreground(ui.ColorMuted).
		Render(strings.Repeat("─", t.width-4))

	header := lipgloss.NewStyle().Italic(true).Foreground(ui.ColorError).
		Render(fmt.Sprintf("Delete agent \"%s\"?", t.deleteAgentName))

	prompt := fmt.Sprintf("  Type %s to confirm: %s",
		dimStyle.Render(t.deleteAgentName),
		activeStyle.Render(t.deleteAgentInput+"█"))

	var errLine string
	if t.err != "" {
		errLine = "\n" + lipgloss.NewStyle().Foreground(ui.ColorError).Render("  "+t.err)
	}

	fullBlock := header + "\n" + mutedLine + "\n" + prompt + errLine + "\n" + mutedLine
	return lipgloss.NewStyle().Padding(0, 2).Render(fullBlock)
}

func (t *VibespacesTab) viewAddAgentForm() string {
	dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDim)
	labelStyle := lipgloss.NewStyle().Foreground(ui.ColorMuted)
	activeStyle := lipgloss.NewStyle().Foreground(ui.ColorText)
	mutedLine := lipgloss.NewStyle().Foreground(ui.ColorMuted).
		Render(strings.Repeat("─", t.width-4))

	header := lipgloss.NewStyle().Italic(true).Foreground(ui.Orange).
		Render("Add agent")

	boolStr := func(v bool) string {
		if v {
			return "yes"
		}
		return "no"
	}

	// Collect selected tools as comma-separated summary
	selectedTools := func(set map[string]bool) string {
		if len(set) == 0 {
			return ""
		}
		var names []string
		for _, tool := range t.addAgentToolsList {
			if set[tool] {
				names = append(names, tool)
			}
		}
		return strings.Join(names, ", ")
	}

	type formEntry struct {
		label    string
		field    addAgentFormField
		value    string
		hint     string
		isSelect bool // j/k toggle (single value)
		isMulti  bool // multi-select (j/k + space)
		multiSet map[string]bool
		isEmpty  func() bool
		emptyVal string
	}

	allowedSummary := selectedTools(t.addAgentAllowedSet)
	disallowedSummary := selectedTools(t.addAgentDisallowedSet)

	entries := []formEntry{
		{"Agent type", addAgentFieldType, string(t.addAgentType), "j/k to change", true, false, nil, nil, ""},
		{"Name", addAgentFieldName, t.addAgentName, "optional, auto-generated if empty", false, false, nil,
			func() bool { return t.addAgentName == "" }, "(auto)"},
	}
	// Only show branch field when the vibespace has worktree enabled
	if t.selectedVS != nil && t.selectedVS.Worktree {
		entries = append(entries, formEntry{"Branch", addAgentFieldBranch, t.addAgentBranch, "git branch (default: agent name)", false, false, nil,
			func() bool { return t.addAgentBranch == "" }, "(agent name)"})
	}
	entries = append(entries,
		formEntry{"Model", addAgentFieldModel, t.addAgentModel, "e.g. opus, sonnet", false, false, nil,
			func() bool { return t.addAgentModel == "" }, "(default)"},
		formEntry{"Max turns", addAgentFieldMaxTurns, t.addAgentMaxTurns, "0 = unlimited", false, false, nil,
			func() bool { return t.addAgentMaxTurns == "" }, "(unlimited)"},
		formEntry{"Share creds", addAgentFieldShareCreds, boolStr(t.addAgentShareCreds), "j/k to toggle", true, false, nil, nil, ""},
		formEntry{"Skip perms", addAgentFieldSkipPerms, boolStr(t.addAgentSkipPerms), "j/k to toggle", true, false, nil, nil, ""},
		formEntry{"Allowed tools", addAgentFieldAllowedTools, allowedSummary, "j/k navigate, space toggle", false, true, t.addAgentAllowedSet,
			func() bool { return len(t.addAgentAllowedSet) == 0 }, "(default)"},
		formEntry{"Disallow tools", addAgentFieldDisallowedTools, disallowedSummary, "j/k navigate, space toggle", false, true, t.addAgentDisallowedSet,
			func() bool { return len(t.addAgentDisallowedSet) == 0 }, "(none)"},
	)

	var lines []string
	for _, e := range entries {
		label := fmt.Sprintf("%-15s", e.label)
		isActive := e.field == t.addAgentField

		if isActive && e.isMulti {
			// Render inline multi-select: label + hint, then tool checkboxes
			lines = append(lines, fmt.Sprintf("  %s %s", labelStyle.Render(label), dimStyle.Render(e.hint)))
			for i, tool := range t.addAgentToolsList {
				check := "[ ]"
				if e.multiSet[tool] {
					check = "[x]"
				}
				prefix := "    "
				if i == t.addAgentToolsCursor {
					lines = append(lines, prefix+activeStyle.Render(check+" "+tool))
				} else {
					lines = append(lines, prefix+dimStyle.Render(check+" "+tool))
				}
			}
			continue
		}

		var val string
		if isActive {
			if e.isSelect {
				val = activeStyle.Render("["+e.value+"]") + "  " + dimStyle.Render(e.hint)
			} else {
				val = activeStyle.Render(e.value + "█")
				if e.isEmpty != nil && e.isEmpty() {
					val += "  " + dimStyle.Render(e.hint)
				}
			}
		} else {
			display := e.value
			if e.isEmpty != nil && e.isEmpty() {
				display = e.emptyVal
			}
			val = dimStyle.Render(display)
		}

		lines = append(lines, fmt.Sprintf("  %s %s", labelStyle.Render(label), val))
	}

	if t.err != "" {
		lines = append(lines, "", lipgloss.NewStyle().Foreground(ui.ColorError).Render("  "+t.err))
	}

	fullBlock := header + "\n" + mutedLine + "\n" + strings.Join(lines, "\n") + "\n" + mutedLine
	return lipgloss.NewStyle().Padding(0, 2).Render(fullBlock)
}

func (t *VibespacesTab) viewEditConfigForm() string {
	dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDim)
	labelStyle := lipgloss.NewStyle().Foreground(ui.ColorMuted)
	activeStyle := lipgloss.NewStyle().Foreground(ui.ColorText)
	mutedLine := lipgloss.NewStyle().Foreground(ui.ColorMuted).
		Render(strings.Repeat("─", t.width-4))

	header := lipgloss.NewStyle().Italic(true).Foreground(ui.Orange).
		Render(fmt.Sprintf("Edit config: %s", t.editConfigAgentName))

	boolStr := func(v bool) string {
		if v {
			return "yes"
		}
		return "no"
	}

	// Collect selected tools as comma-separated summary
	selectedTools := func(set map[string]bool) string {
		if len(set) == 0 {
			return ""
		}
		var names []string
		for _, tool := range t.editConfigToolsList {
			if set[tool] {
				names = append(names, tool)
			}
		}
		return strings.Join(names, ", ")
	}

	type formEntry struct {
		label    string
		field    editConfigFormField
		value    string
		hint     string
		isSelect bool
		isMulti  bool
		multiSet map[string]bool
		isEmpty  func() bool
		emptyVal string
	}

	allowedSummary := selectedTools(t.editConfigAllowedSet)
	disallowedSummary := selectedTools(t.editConfigDisallowedSet)

	entries := []formEntry{
		{"Model", editConfigFieldModel, t.editConfigModel, "e.g. opus, sonnet", false, false, nil,
			func() bool { return t.editConfigModel == "" }, "(default)"},
		{"Max turns", editConfigFieldMaxTurns, t.editConfigMaxTurns, "0 = unlimited", false, false, nil,
			func() bool { return t.editConfigMaxTurns == "" }, "(unlimited)"},
		{"Skip perms", editConfigFieldSkipPerms, boolStr(t.editConfigSkipPerms), "j/k to toggle", true, false, nil, nil, ""},
		{"Allowed tools", editConfigFieldAllowedTools, allowedSummary, "j/k navigate, space toggle", false, true, t.editConfigAllowedSet,
			func() bool { return len(t.editConfigAllowedSet) == 0 }, "(default)"},
		{"Disallow tools", editConfigFieldDisallowedTools, disallowedSummary, "j/k navigate, space toggle", false, true, t.editConfigDisallowedSet,
			func() bool { return len(t.editConfigDisallowedSet) == 0 }, "(none)"},
	}

	var lines []string
	for _, e := range entries {
		label := fmt.Sprintf("%-15s", e.label)
		isActive := e.field == t.editConfigField

		if isActive && e.isMulti {
			lines = append(lines, fmt.Sprintf("  %s %s", labelStyle.Render(label), dimStyle.Render(e.hint)))
			for i, tool := range t.editConfigToolsList {
				check := "[ ]"
				if e.multiSet[tool] {
					check = "[x]"
				}
				prefix := "    "
				if i == t.editConfigToolsCursor {
					lines = append(lines, prefix+activeStyle.Render(check+" "+tool))
				} else {
					lines = append(lines, prefix+dimStyle.Render(check+" "+tool))
				}
			}
			continue
		}

		var val string
		if isActive {
			if e.isSelect {
				val = activeStyle.Render("["+e.value+"]") + "  " + dimStyle.Render(e.hint)
			} else {
				val = activeStyle.Render(e.value + "█")
				if e.isEmpty != nil && e.isEmpty() {
					val += "  " + dimStyle.Render(e.hint)
				}
			}
		} else {
			display := e.value
			if e.isEmpty != nil && e.isEmpty() {
				display = e.emptyVal
			}
			val = dimStyle.Render(display)
		}

		lines = append(lines, fmt.Sprintf("  %s %s", labelStyle.Render(label), val))
	}

	if t.err != "" {
		lines = append(lines, "", lipgloss.NewStyle().Foreground(ui.ColorError).Render("  "+t.err))
	}

	fullBlock := header + "\n" + mutedLine + "\n" + strings.Join(lines, "\n") + "\n" + mutedLine
	return lipgloss.NewStyle().Padding(0, 2).Render(fullBlock)
}

func (t *VibespacesTab) viewForwardManager() string {
	dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDim)
	labelStyle := lipgloss.NewStyle().Foreground(ui.ColorMuted)
	activeStyle := lipgloss.NewStyle().Foreground(ui.ColorText)
	mutedLine := lipgloss.NewStyle().Foreground(ui.ColorMuted).
		Render(strings.Repeat("─", t.width-4))

	agentName := ""
	if t.agentCursor < len(t.viewAgents) {
		agentName = t.viewAgents[t.agentCursor].AgentName
	}

	header := lipgloss.NewStyle().Italic(true).Foreground(ui.Orange).
		Render(fmt.Sprintf("Forwards: %s", agentName))

	fwds := t.currentAgentForwards()

	var lines []string

	if len(fwds) == 0 {
		lines = append(lines, "  "+dimStyle.Render("No forwards."))
	} else {
		// Compute column widths for alignment
		var maxRemote, maxLocal, maxType int
		for _, fwd := range fwds {
			if r := len(fmt.Sprintf(":%d", fwd.RemotePort)); r > maxRemote {
				maxRemote = r
			}
			if l := len(fmt.Sprintf(":%d", fwd.LocalPort)); l > maxLocal {
				maxLocal = l
			}
			if len(fwd.Type) > maxType {
				maxType = len(fwd.Type)
			}
		}

		for i, fwd := range fwds {
			remote := fmt.Sprintf(":%d", fwd.RemotePort)
			local := fmt.Sprintf(":%d", fwd.LocalPort)
			line := fmt.Sprintf("%-*s  → %-*s  %-*s  %s",
				maxRemote, remote,
				maxLocal, local,
				maxType, fwd.Type,
				fwd.Status)

			// Build clickable DNS hyperlink (OSC 8)
			var dnsSuffix string
			if fwd.DNSName != "" {
				host := fwd.DNSName + ".vibespace.internal"
				display := fmt.Sprintf("%s:%d", host, fwd.LocalPort)
				url := "http://" + display
				dnsSuffix = "  " + fmt.Sprintf("\x1b]8;;%s\x07%s\x1b]8;;\x07", url, display)
			}

			if i == t.fwdManagerCursor {
				lines = append(lines, "  "+activeStyle.Render("› "+line)+dnsSuffix)
			} else {
				lines = append(lines, "  "+dimStyle.Render("  "+line)+dnsSuffix)
			}
		}
	}

	// Add forward sub-form
	if t.fwdManagerAdding {
		lines = append(lines, "")
		addHeader := lipgloss.NewStyle().Italic(true).Foreground(ui.ColorMuted).
			Render("  Add forward")
		lines = append(lines, addHeader)

		remoteLabel := fmt.Sprintf("  %-14s", "Remote port")
		localLabel := fmt.Sprintf("  %-14s", "Local port")

		if t.fwdManagerAddField == fwdManagerAddFieldRemote {
			lines = append(lines, fmt.Sprintf("  %s %s",
				labelStyle.Render(remoteLabel),
				activeStyle.Render(t.fwdManagerAddRemote+"█")))
		} else {
			lines = append(lines, fmt.Sprintf("  %s %s",
				labelStyle.Render(remoteLabel),
				dimStyle.Render(t.fwdManagerAddRemote)))
		}

		if t.fwdManagerAddField == fwdManagerAddFieldLocal {
			lines = append(lines, fmt.Sprintf("  %s %s  %s",
				labelStyle.Render(localLabel),
				activeStyle.Render(t.fwdManagerAddLocal+"█"),
				dimStyle.Render("0 = auto")))
		} else {
			display := t.fwdManagerAddLocal
			if display == "" {
				display = "(auto)"
			}
			lines = append(lines, fmt.Sprintf("  %s %s",
				labelStyle.Render(localLabel),
				dimStyle.Render(display)))
		}

		dnsLabel := fmt.Sprintf("  %-14s", "DNS")
		dnsNameLabel := fmt.Sprintf("  %-14s", "DNS name")
		dnsToggle := "[ ]"
		if t.fwdManagerAddDNS {
			dnsToggle = "[x]"
		}
		if t.fwdManagerAddField == fwdManagerAddFieldDNS {
			lines = append(lines, fmt.Sprintf("  %s %s  %s",
				labelStyle.Render(dnsLabel),
				activeStyle.Render(dnsToggle),
				dimStyle.Render("Space to toggle")))
		} else {
			lines = append(lines, fmt.Sprintf("  %s %s",
				labelStyle.Render(dnsLabel),
				dimStyle.Render(dnsToggle)))
		}

		if t.fwdManagerAddDNS {
			if t.fwdManagerAddField == fwdManagerAddFieldDNSName {
				lines = append(lines, fmt.Sprintf("  %s %s  %s",
					labelStyle.Render(dnsNameLabel),
					activeStyle.Render(t.fwdManagerAddDNSName+"█"),
					dimStyle.Render("blank = agent.vibespace")))
			} else {
				display := t.fwdManagerAddDNSName
				if display == "" {
					display = "(default)"
				}
				lines = append(lines, fmt.Sprintf("  %s %s",
					labelStyle.Render(dnsNameLabel),
					dimStyle.Render(display)))
			}
		}
	}

	// Sudo password prompt for DNS host entry
	if t.sudoPromptActive {
		warnStyle := lipgloss.NewStyle().Foreground(ui.ColorWarning).Bold(true)
		lines = append(lines, "")
		lines = append(lines, "  "+warnStyle.Render("sudo required")+" "+dimStyle.Render("for /etc/hosts DNS entry"))
		mask := strings.Repeat("•", len(t.sudoInput))
		lines = append(lines, "  "+dimStyle.Render("Password:")+" "+activeStyle.Render(mask+"█"))
		lines = append(lines, "  "+dimStyle.Render("enter submit  esc skip"))
	}

	if t.err != "" {
		lines = append(lines, "", lipgloss.NewStyle().Foreground(ui.ColorError).Render("  "+t.err))
	}

	fullBlock := header + "\n" + mutedLine + "\n" + strings.Join(lines, "\n") + "\n" + mutedLine
	return lipgloss.NewStyle().Padding(0, 2).Render(fullBlock)
}
