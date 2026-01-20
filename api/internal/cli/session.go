package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"vibespace/pkg/session"
	"vibespace/pkg/tui"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Manage multi-agent sessions",
	Long: `Manage multi-agent sessions for working with multiple Claude agents across vibespaces.

Sessions allow you to group agents from multiple vibespaces and interact with them
through a terminal UI.`,
}

var sessionCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new session",
	Args:  cobra.ExactArgs(1),
	RunE:  runSessionCreate,
}

var sessionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all sessions",
	Args:  cobra.NoArgs,
	RunE:  runSessionList,
}

var sessionDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a session",
	Args:  cobra.ExactArgs(1),
	RunE:  runSessionDelete,
}

var sessionAddCmd = &cobra.Command{
	Use:   "add <session> <vibespace> [agent]",
	Short: "Add a vibespace or agent to a session",
	Long: `Add a vibespace or specific agent to an existing session.

If no agent is specified, all agents from the vibespace will be included.`,
	Args: cobra.RangeArgs(2, 3),
	RunE: runSessionAdd,
}

var sessionRemoveCmd = &cobra.Command{
	Use:   "remove <session> <vibespace> [agent]",
	Short: "Remove a vibespace or agent from a session",
	Long: `Remove a vibespace or specific agent from a session.

If no agent is specified, the entire vibespace is removed from the session.`,
	Args: cobra.RangeArgs(2, 3),
	RunE: runSessionRemove,
}

var sessionStartCmd = &cobra.Command{
	Use:   "start <name>",
	Short: "Start a session (launch TUI)",
	Args:  cobra.ExactArgs(1),
	RunE:  runSessionStart,
}

var sessionShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show session details",
	Args:  cobra.ExactArgs(1),
	RunE:  runSessionShow,
}

func init() {
	sessionCmd.AddCommand(sessionCreateCmd)
	sessionCmd.AddCommand(sessionListCmd)
	sessionCmd.AddCommand(sessionDeleteCmd)
	sessionCmd.AddCommand(sessionAddCmd)
	sessionCmd.AddCommand(sessionRemoveCmd)
	sessionCmd.AddCommand(sessionStartCmd)
	sessionCmd.AddCommand(sessionShowCmd)
}

func runSessionCreate(cmd *cobra.Command, args []string) error {
	name := args[0]

	store, err := session.NewStore()
	if err != nil {
		return err
	}

	sess, err := store.Create(name)
	if err != nil {
		return err
	}

	printSuccess("Created session '%s'", sess.Name)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  vibespace session add %s <vibespace>    Add a vibespace to the session\n", name)
	fmt.Printf("  vibespace session start %s              Start the session TUI\n", name)

	return nil
}

func runSessionList(cmd *cobra.Command, args []string) error {
	store, err := session.NewStore()
	if err != nil {
		return err
	}

	sessions, err := store.List()
	if err != nil {
		return err
	}

	if len(sessions) == 0 {
		fmt.Println("No sessions found.")
		fmt.Println()
		fmt.Println("Create one with:")
		fmt.Println("  vibespace session create <name>")
		return nil
	}

	fmt.Println("Sessions:")
	fmt.Println()

	for _, sess := range sessions {
		// Format vibespaces count
		vsCount := len(sess.Vibespaces)
		vsLabel := "vibespace"
		if vsCount != 1 {
			vsLabel = "vibespaces"
		}

		// Format last used time
		lastUsed := formatRelativeTime(sess.LastUsed)

		fmt.Printf("  %s\n", color.New(color.Bold).Sprint(sess.Name))
		fmt.Printf("    %d %s, last used %s\n", vsCount, vsLabel, lastUsed)
	}

	return nil
}

func runSessionDelete(cmd *cobra.Command, args []string) error {
	name := args[0]

	store, err := session.NewStore()
	if err != nil {
		return err
	}

	if err := store.Delete(name); err != nil {
		return err
	}

	printSuccess("Deleted session '%s'", name)
	return nil
}

func runSessionAdd(cmd *cobra.Command, args []string) error {
	sessionName := args[0]
	vibespace := args[1]
	var agents []string
	if len(args) > 2 {
		agents = []string{args[2]}
	}

	// Verify vibespace exists
	ctx := context.Background()
	svc, err := getVibespaceService()
	if err != nil {
		return err
	}

	_, err = svc.Get(ctx, vibespace)
	if err != nil {
		return fmt.Errorf("vibespace '%s' not found", vibespace)
	}

	// Update session
	store, err := session.NewStore()
	if err != nil {
		return err
	}

	sess, err := store.Get(sessionName)
	if err != nil {
		return err
	}

	sess.AddVibespace(vibespace, agents)

	if err := store.Save(sess); err != nil {
		return err
	}

	if len(agents) > 0 {
		printSuccess("Added %s from %s to session '%s'", agents[0], vibespace, sessionName)
	} else {
		printSuccess("Added vibespace %s to session '%s'", vibespace, sessionName)
	}

	return nil
}

func runSessionRemove(cmd *cobra.Command, args []string) error {
	sessionName := args[0]
	vibespace := args[1]
	var agent string
	if len(args) > 2 {
		agent = args[2]
	}

	store, err := session.NewStore()
	if err != nil {
		return err
	}

	sess, err := store.Get(sessionName)
	if err != nil {
		return err
	}

	if agent != "" {
		sess.RemoveAgent(session.AgentAddress{Agent: agent, Vibespace: vibespace})
		printSuccess("Removed %s@%s from session '%s'", agent, vibespace, sessionName)
	} else {
		sess.RemoveVibespace(vibespace)
		printSuccess("Removed vibespace %s from session '%s'", vibespace, sessionName)
	}

	if err := store.Save(sess); err != nil {
		return err
	}

	return nil
}

func runSessionStart(cmd *cobra.Command, args []string) error {
	name := args[0]

	store, err := session.NewStore()
	if err != nil {
		return err
	}

	sess, err := store.Get(name)
	if err != nil {
		return err
	}

	if len(sess.Vibespaces) == 0 {
		return fmt.Errorf("session '%s' has no vibespaces. Add one with: vibespace session add %s <vibespace>", name, name)
	}

	// Update last used time
	sess.LastUsed = time.Now()
	_ = store.Save(sess)

	// Launch TUI
	return runTUI(sess, false)
}

func runSessionShow(cmd *cobra.Command, args []string) error {
	name := args[0]

	store, err := session.NewStore()
	if err != nil {
		return err
	}

	sess, err := store.Get(name)
	if err != nil {
		return err
	}

	bold := color.New(color.Bold)
	dim := color.New(color.Faint)

	fmt.Printf("Session: %s\n", bold.Sprint(sess.Name))
	fmt.Printf("Created: %s\n", sess.CreatedAt.Format("2006-01-02 15:04"))
	fmt.Printf("Last used: %s\n", formatRelativeTime(sess.LastUsed))
	fmt.Printf("Layout: %s\n", sess.Layout.Mode)

	if len(sess.Vibespaces) == 0 {
		fmt.Println()
		fmt.Println("No vibespaces in this session.")
		return nil
	}

	fmt.Println()
	fmt.Println("Vibespaces:")
	for _, vs := range sess.Vibespaces {
		fmt.Printf("  %s\n", bold.Sprint(vs.Name))
		if len(vs.Agents) > 0 {
			fmt.Printf("    Agents: %s\n", strings.Join(vs.Agents, ", "))
		} else {
			fmt.Printf("    Agents: %s\n", dim.Sprint("(all)"))
		}
	}

	return nil
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

// runTUI launches the terminal UI for a session
func runTUI(sess *session.Session, isAdHoc bool) error {
	return tui.Run(sess, isAdHoc)
}
