package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/yagizdagabak/vibespace/pkg/session"

	"github.com/spf13/cobra"
)

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Manage multi-agent sessions",
	Long: `Manage multi-agent sessions for working with multiple Claude agents across vibespaces.

Use 'vibespace multi' to create and launch sessions interactively.
Use these commands to list, show details, or delete existing sessions.`,
	Example: `  vibespace session list           # List all sessions
  vibespace session show mywork     # Show session details
  vibespace session delete mywork   # Delete a session`,
}

var sessionListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all sessions",
	Example: `  vibespace session list`,
	Args:    cobra.NoArgs,
	RunE:    runSessionList,
}

var sessionDeleteCmd = &cobra.Command{
	Use:     "delete <name>",
	Short:   "Delete a session",
	Example: `  vibespace session delete mysession`,
	Args:    cobra.ExactArgs(1),
	RunE:    runSessionDelete,
}

var sessionShowCmd = &cobra.Command{
	Use:     "show <name>",
	Short:   "Show session details",
	Example: `  vibespace session show mysession`,
	Args:    cobra.ExactArgs(1),
	RunE:    runSessionShow,
}

func init() {
	sessionCmd.AddCommand(sessionListCmd)
	sessionCmd.AddCommand(sessionDeleteCmd)
	sessionCmd.AddCommand(sessionShowCmd)
}

func runSessionList(cmd *cobra.Command, args []string) error {
	out := getOutput()
	store, err := session.NewStore()
	if err != nil {
		return err
	}

	sessions, err := store.List()
	if err != nil {
		return err
	}

	// JSON output mode
	if out.IsJSONMode() {
		items := make([]SessionListItem, len(sessions))
		for i, sess := range sessions {
			items[i] = SessionListItem{
				Name:       sess.Name,
				Vibespaces: len(sess.Vibespaces),
				LastUsed:   sess.LastUsed,
			}
		}
		return out.JSON(NewJSONOutput(true, SessionListOutput{
			Sessions: items,
			Count:    len(items),
		}, nil))
	}

	if len(sessions) == 0 {
		// Plain mode - no output for empty result
		if out.IsPlainMode() {
			return nil
		}
		fmt.Println("No sessions found.")
		fmt.Println()
		fmt.Println("Create a new session with:")
		fmt.Println("  vibespace multi --vibespaces <name>")
		fmt.Println()
		fmt.Println("Or start an empty session and add vibespaces interactively:")
		fmt.Println("  vibespace multi")
		return nil
	}

	// Build table rows
	headers := []string{"NAME", "VIBESPACES", "AGENTS", "LAST USED"}
	rows := make([][]string, len(sessions))
	for i, sess := range sessions {
		vsNames, agentCount := formatSessionInfo(sess)
		lastUsed := formatRelativeTime(sess.LastUsed)
		rows[i] = []string{sess.Name, vsNames, agentCount, lastUsed}
	}

	out.Table(headers, rows)

	// Don't print footer in plain mode
	if !out.IsPlainMode() {
		fmt.Println()
		fmt.Printf("Resume a session: %s\n", out.Dim("vibespace multi -r"))
	}

	return nil
}

func runSessionDelete(cmd *cobra.Command, args []string) error {
	name := args[0]
	out := getOutput()

	store, err := session.NewStore()
	if err != nil {
		return err
	}

	if err := store.Delete(name); err != nil {
		return err
	}

	// JSON output
	if out.IsJSONMode() {
		return out.JSON(NewJSONOutput(true, SessionDeleteOutput{
			Name: name,
		}, nil))
	}

	printSuccess("Deleted session '%s'", name)
	return nil
}

func runSessionShow(cmd *cobra.Command, args []string) error {
	name := args[0]
	out := getOutput()

	store, err := session.NewStore()
	if err != nil {
		return err
	}

	sess, err := store.Get(name)
	if err != nil {
		return err
	}

	// JSON output mode
	if out.IsJSONMode() {
		jsonOut := SessionShowOutput{
			Name:      sess.Name,
			CreatedAt: sess.CreatedAt,
			LastUsed:  sess.LastUsed,
			Layout:    string(sess.Layout.Mode),
		}
		for _, vs := range sess.Vibespaces {
			jsonOut.Vibespaces = append(jsonOut.Vibespaces, SessionVibespace{
				Name:   vs.Name,
				Agents: vs.Agents,
			})
		}
		return out.JSON(NewJSONOutput(true, jsonOut, nil))
	}

	fmt.Printf("Session: %s\n", out.Bold(sess.Name))
	fmt.Printf("Created: %s\n", sess.CreatedAt.Format("2006-01-02 15:04"))
	fmt.Printf("Last used: %s\n", formatRelativeTime(sess.LastUsed))
	fmt.Printf("Layout: %s\n", sess.Layout.Mode)

	if len(sess.Vibespaces) == 0 {
		fmt.Println()
		fmt.Println("No vibespaces in this session.")
		fmt.Println("Add vibespaces with /add inside the TUI")
		return nil
	}

	fmt.Println()
	fmt.Println("Vibespaces:")
	for _, vs := range sess.Vibespaces {
		fmt.Printf("  %s\n", out.Bold(vs.Name))
		if len(vs.Agents) > 0 {
			fmt.Printf("    Agents: %s\n", joinStrings(vs.Agents, ", "))
		} else {
			fmt.Printf("    Agents: (all)\n")
		}
	}

	return nil
}

// truncateStr truncates a string to maxLen with ellipsis
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// formatRelativeTime formats a time as a relative string
func formatRelativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case d < 24*time.Hour:
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case d < 7*24*time.Hour:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "yesterday"
		}
		return fmt.Sprintf("%d days ago", days)
	default:
		return t.Format("2006-01-02")
	}
}

// joinStrings joins strings with a separator
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

// formatSessionInfo returns vibespace names and agent names for display
func formatSessionInfo(sess session.Session) (vibespaces string, agents string) {
	if len(sess.Vibespaces) == 0 {
		return "(empty)", "-"
	}

	vsNames := make([]string, len(sess.Vibespaces))
	var agentNames []string

	for i, vs := range sess.Vibespaces {
		vsNames[i] = vs.Name
		if len(vs.Agents) > 0 {
			// Specific agents selected
			for _, a := range vs.Agents {
				agentNames = append(agentNames, a+"@"+vs.Name)
			}
		}
	}

	vibespaces = strings.Join(vsNames, ", ")

	if len(agentNames) > 0 {
		agents = strings.Join(agentNames, ", ")
	} else {
		agents = "all"
	}

	return vibespaces, agents
}
