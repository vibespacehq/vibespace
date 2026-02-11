//go:build e2e

package e2e

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- Binary runner ---

// RunResult holds the output of a command execution.
type RunResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// projectRoot walks up from the test directory to find the go.mod file.
func projectRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find project root (no go.mod)")
		}
		dir = parent
	}
}

// binaryPath returns the path to the pre-built vibespace binary at the project root.
func binaryPath(t *testing.T) string {
	t.Helper()
	p := filepath.Join(projectRoot(t), "vibespace")
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("binary not found at %s — run 'go build -o vibespace ./cmd/vibespace' first", p)
	}
	return p
}

// run executes the vibespace binary with the given arguments.
func run(t *testing.T, args ...string) RunResult {
	t.Helper()
	bin := binaryPath(t)
	cmd := exec.Command(bin, args...)
	env := append(os.Environ(), "NO_COLOR=1", "VIBESPACE_DEBUG=1")

	// Ensure GOCOVERDIR exists so coverage-instrumented binaries can write data.
	if dir := os.Getenv("GOCOVERDIR"); dir != "" {
		os.MkdirAll(dir, 0755)
	}

	cmd.Env = env

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := RunResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run %v: %v", args, err)
		}
	}

	return result
}

// runWithStdin executes the vibespace binary with the given stdin and arguments.
func runWithStdin(t *testing.T, stdin string, args ...string) RunResult {
	t.Helper()
	bin := binaryPath(t)
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), "NO_COLOR=1", "VIBESPACE_DEBUG=1")
	cmd.Stdin = strings.NewReader(stdin)

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := RunResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run %v: %v", args, err)
		}
	}

	return result
}

// runJSON executes the vibespace binary with --json appended and parses the output.
func runJSON(t *testing.T, args ...string) JSONOutput {
	t.Helper()
	args = append(args, "--json")
	r := run(t, args...)

	var out JSONOutput
	if err := json.Unmarshal([]byte(r.Stdout), &out); err != nil {
		t.Fatalf("failed to parse JSON output from %v:\nstdout: %s\nstderr: %s\nerror: %v",
			args, r.Stdout, r.Stderr, err)
	}
	return out
}

// mustSucceed runs a JSON command and asserts success=true.
func mustSucceed(t *testing.T, args ...string) JSONOutput {
	t.Helper()
	out := runJSON(t, args...)
	if !out.Success {
		errMsg := ""
		if out.Error != nil {
			errMsg = out.Error.Message
		}
		t.Fatalf("expected success for %v but got error: %s\nraw: %+v", args, errMsg, out)
	}
	return out
}

// parseData unmarshals JSONOutput.Data into the given type.
func parseData[T any](t *testing.T, out JSONOutput) T {
	t.Helper()
	raw, err := json.Marshal(out.Data)
	if err != nil {
		t.Fatalf("failed to re-marshal data: %v", err)
	}
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("failed to unmarshal data into %T: %v\nraw: %s", v, err, string(raw))
	}
	return v
}

// --- JSON types (mirrored from internal/cli, cannot import internal) ---

// JSONOutput is the standard wrapper for all JSON output.
type JSONOutput struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data,omitempty"`
	Error   *JSONError      `json:"error,omitempty"`
	Meta    JSONMeta        `json:"meta"`
}

// JSONError represents an error in JSON output.
type JSONError struct {
	Message  string `json:"message"`
	Code     string `json:"code,omitempty"`
	ExitCode int    `json:"exit_code,omitempty"`
	Hint     string `json:"hint,omitempty"`
}

// JSONMeta contains metadata about the CLI response.
type JSONMeta struct {
	SchemaVersion string `json:"schema_version"`
	CLIVersion    string `json:"cli_version"`
	Timestamp     string `json:"timestamp"`
}

// StatusData is the JSON data from `vibespace status --json`.
type StatusData struct {
	Cluster struct {
		Installed bool   `json:"installed"`
		Running   bool   `json:"running"`
		Platform  string `json:"platform"`
	} `json:"cluster"`
}

// CreateData is the JSON data from `vibespace create --json`.
type CreateData struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

// ListData is the JSON data from `vibespace list --json`.
type ListData struct {
	Vibespaces []VibespaceItem `json:"vibespaces"`
	Count      int             `json:"count"`
}

// VibespaceItem represents a vibespace in list output.
type VibespaceItem struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	Agents    int    `json:"agents"`
	CPU       string `json:"cpu"`
	Memory    string `json:"memory"`
	Storage   string `json:"storage"`
	CreatedAt string `json:"created_at"`
}

// AgentsData is the JSON data from `vibespace <name> agent --json`.
type AgentsData struct {
	Vibespace string      `json:"vibespace"`
	Agents    []AgentItem `json:"agents"`
	Count     int         `json:"count"`
}

// AgentItem represents an agent in the agents list.
type AgentItem struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Vibespace string `json:"vibespace"`
	Status    string `json:"status"`
}

// DeleteData is the JSON data from `vibespace delete --json`.
type DeleteData struct {
	Name     string `json:"name"`
	KeepData bool   `json:"keep_data"`
}

// InfoData is the JSON data from `vibespace <name> info --json`.
type InfoData struct {
	Name      string          `json:"name"`
	ID        string          `json:"id"`
	Status    string          `json:"status"`
	PVC       string          `json:"pvc"`
	CPU       string          `json:"cpu"`
	Memory    string          `json:"memory"`
	Storage   string          `json:"storage"`
	Mounts    []MountData     `json:"mounts,omitempty"`
	Agents    []InfoAgentData `json:"agents"`
	Forwards  []ForwardAgent  `json:"forwards,omitempty"`
	CreatedAt string          `json:"created_at"`
}

// MountData represents a mount in info output.
type MountData struct {
	HostPath      string `json:"host_path"`
	ContainerPath string `json:"container_path"`
	ReadOnly      bool   `json:"read_only"`
}

// InfoAgentData represents an agent with config in info output.
type InfoAgentData struct {
	Name   string          `json:"name"`
	Type   string          `json:"type"`
	Status string          `json:"status"`
	Config AgentConfigData `json:"config"`
}

// AgentConfigData represents agent configuration fields.
type AgentConfigData struct {
	SkipPermissions  bool     `json:"skip_permissions"`
	ShareCredentials bool     `json:"share_credentials"`
	AllowedTools     []string `json:"allowed_tools"`
	DisallowedTools  []string `json:"disallowed_tools"`
	Model            string   `json:"model"`
	MaxTurns         int      `json:"max_turns"`
	SystemPrompt     string   `json:"system_prompt"`
	ReasoningEffort  string   `json:"reasoning_effort,omitempty"`
}

// ConfigShowData is the JSON data from `vibespace <name> config show <agent> --json`.
type ConfigShowData struct {
	Vibespace string          `json:"vibespace"`
	Agent     string          `json:"agent"`
	Type      string          `json:"type"`
	Config    AgentConfigData `json:"config"`
}

// ConfigShowAllData is the JSON data from `vibespace <name> config show --json` (all agents).
type ConfigShowAllData struct {
	Vibespace string            `json:"vibespace"`
	Agents    []ConfigAgentItem `json:"agents"`
}

// ConfigAgentItem represents an agent with its config in config show all output.
type ConfigAgentItem struct {
	Agent  string          `json:"agent"`
	Type   string          `json:"type"`
	Config AgentConfigData `json:"config"`
}

// ConfigSetData is the JSON data from `vibespace <name> config set --json`.
type ConfigSetData struct {
	Vibespace string          `json:"vibespace"`
	Agent     string          `json:"agent"`
	Config    AgentConfigData `json:"config"`
}

// AgentCreateData is the JSON data from `vibespace <name> agent create --json`.
type AgentCreateData struct {
	Vibespace string `json:"vibespace"`
	Agent     string `json:"agent"`
	Type      string `json:"type"`
}

// AgentDeleteData is the JSON data from `vibespace <name> agent delete --json`.
type AgentDeleteData struct {
	Vibespace string `json:"vibespace"`
	Agent     string `json:"agent"`
}

// StartData is the JSON data from `vibespace <name> start --json`.
type StartData struct {
	Vibespace string `json:"vibespace"`
	Agent     string `json:"agent,omitempty"`
}

// StopData is the JSON data from `vibespace <name> stop --json`.
type StopData struct {
	Stopped bool   `json:"stopped"`
	Target  string `json:"target,omitempty"`
}

// SessionListData is the JSON data from `vibespace session list --json`.
type SessionListData struct {
	Sessions []SessionListItem `json:"sessions"`
	Count    int               `json:"count"`
}

// SessionListItem represents a session in session list output.
type SessionListItem struct {
	Name       string `json:"name"`
	Vibespaces int    `json:"vibespaces"`
	LastUsed   string `json:"last_used"`
}

// ExecData is the JSON data from `vibespace <name> exec --json`.
type ExecData struct {
	Vibespace string `json:"vibespace"`
	Agent     string `json:"agent"`
	Command   string `json:"command"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	ExitCode  int    `json:"exit_code"`
}

// ForwardsData is the JSON data from `vibespace <name> forward list --json`.
type ForwardsData struct {
	Vibespace string         `json:"vibespace"`
	Agents    []ForwardAgent `json:"agents"`
}

// ForwardAgent represents an agent with its forwards.
type ForwardAgent struct {
	Name     string        `json:"name"`
	PodName  string        `json:"pod_name,omitempty"`
	Forwards []ForwardInfo `json:"forwards"`
}

// ForwardInfo represents a port forward.
type ForwardInfo struct {
	LocalPort  int    `json:"local_port"`
	RemotePort int    `json:"remote_port"`
	Type       string `json:"type"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
	Reconnects int    `json:"reconnects,omitempty"`
}

// ForwardAddData is the JSON data from `vibespace <name> forward add --json`.
type ForwardAddData struct {
	Vibespace  string `json:"vibespace"`
	Agent      string `json:"agent"`
	LocalPort  int    `json:"local_port"`
	RemotePort int    `json:"remote_port"`
	DNSName    string `json:"dns_name,omitempty"`
}

// ForwardRemoveData is the JSON data from `vibespace <name> forward remove --json`.
type ForwardRemoveData struct {
	Vibespace  string `json:"vibespace"`
	Agent      string `json:"agent"`
	RemotePort int    `json:"remote_port"`
}

// PortsData is the JSON data from `vibespace <name> ports --json`.
type PortsData struct {
	Vibespace string         `json:"vibespace"`
	Ports     []DetectedPort `json:"ports"`
	Count     int            `json:"count"`
}

// DetectedPort represents a detected port.
type DetectedPort struct {
	Port       int    `json:"port"`
	Process    string `json:"process"`
	DetectedAt string `json:"detected_at"`
}

// MultiListSessionsData is the JSON data from `vibespace multi --list-sessions --json`.
type MultiListSessionsData struct {
	Sessions []MultiSessionItem `json:"sessions"`
	Count    int                `json:"count"`
}

// MultiSessionItem represents a session in multi list output.
type MultiSessionItem struct {
	Name       string   `json:"name"`
	Vibespaces []string `json:"vibespaces"`
	CreatedAt  string   `json:"created_at"`
	LastUsed   string   `json:"last_used"`
}

// MultiListAgentsData is the JSON data from `vibespace multi --list-agents --json`.
type MultiListAgentsData struct {
	Session string   `json:"session"`
	Agents  []string `json:"agents"`
	Count   int      `json:"count"`
}

// MultiMessageData is the JSON data from `vibespace multi --json "message"`.
type MultiMessageData struct {
	Session   string               `json:"session"`
	Request   MultiRequestInfo     `json:"request"`
	Responses []MultiAgentResponse `json:"responses"`
}

// MultiRequestInfo contains request details.
type MultiRequestInfo struct {
	Target  string `json:"target"`
	Message string `json:"message"`
}

// MultiAgentResponse represents a response from a single agent.
type MultiAgentResponse struct {
	Agent     string `json:"agent"`
	Timestamp string `json:"timestamp"`
	Content   string `json:"content"`
	Error     string `json:"error,omitempty"`
}

// --- Helpers ---

// waitForReady polls `vibespace list --json` until the vibespace status is "running".
// Polls every 5 seconds with a 3-minute timeout.
func waitForReady(t *testing.T, vsName string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Minute)
	for {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for vibespace '%s' to become running", vsName)
		}

		out := runJSON(t, "list")
		if out.Success {
			data := parseData[ListData](t, out)
			for _, vs := range data.Vibespaces {
				if vs.Name == vsName && vs.Status == "running" {
					t.Logf("vibespace '%s' is running", vsName)
					return
				}
			}
		}

		t.Logf("waiting for vibespace '%s' to become running...", vsName)
		time.Sleep(5 * time.Second)
	}
}

// waitForDaemonReady polls `vibespace <name> forward list --json` until the daemon
// returns at least 1 agent with an active SSH forward. The daemon auto-starts on
// first forward command but needs time to: spawn → reconcile pods → set up kubectl
// port-forwards for SSH. On slow CI runners this can take 10-20 seconds.
// Polls every 3 seconds with a 60-second timeout.
func waitForDaemonReady(t *testing.T, vsName string) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for daemon to have agent with active SSH forward for '%s'", vsName)
		}

		out := runJSON(t, vsName, "forward", "list")
		if out.Success {
			data := parseData[ForwardsData](t, out)
			for _, agent := range data.Agents {
				for _, fwd := range agent.Forwards {
					if fwd.Type == "ssh" && fwd.Status == "active" {
						t.Logf("daemon ready: agent=%s ssh_port=%d", agent.Name, fwd.LocalPort)
						return
					}
				}
			}
		}

		t.Logf("waiting for daemon to be ready for '%s'...", vsName)
		time.Sleep(3 * time.Second)
	}
}

// mustSucceedPlain runs a command with --plain and asserts exit code 0.
func mustSucceedPlain(t *testing.T, args ...string) string {
	t.Helper()
	args = append(args, "--plain")
	r := run(t, args...)
	if r.ExitCode != 0 {
		t.Fatalf("expected exit code 0 for %v --plain but got %d\nstdout: %s\nstderr: %s",
			args, r.ExitCode, r.Stdout, r.Stderr)
	}
	return r.Stdout
}

// --- Expanded subtests ---

// runExpandedSubtests runs all expanded E2E subtests between the initial "agents"
// check and the final "delete" step. Called from all 3 platform test files.
func runExpandedSubtests(t *testing.T, vsName string) {
	// === Pre-ready tests (k8s metadata only, pods may still be starting) ===

	// --- info ---
	t.Run("info", func(t *testing.T) {
		out := mustSucceed(t, vsName, "info")
		data := parseData[InfoData](t, out)

		if data.Name != vsName {
			t.Errorf("expected name=%s, got %s", vsName, data.Name)
		}
		if data.ID == "" {
			t.Error("expected non-empty id")
		}
		if len(data.Agents) < 1 {
			t.Errorf("expected at least 1 agent, got %d", len(data.Agents))
		}
	})

	// --- config show all ---
	t.Run("config-show-all", func(t *testing.T) {
		out := mustSucceed(t, vsName, "config")
		data := parseData[ConfigShowAllData](t, out)

		if data.Vibespace != vsName {
			t.Errorf("expected vibespace=%s, got %s", vsName, data.Vibespace)
		}
		if len(data.Agents) < 1 {
			t.Errorf("expected at least 1 agent in config, got %d", len(data.Agents))
		}
	})

	// --- config show (single agent) ---
	t.Run("config-show", func(t *testing.T) {
		out := mustSucceed(t, vsName, "config", "show", "claude-1")
		data := parseData[ConfigShowData](t, out)

		if data.Agent != "claude-1" {
			t.Errorf("expected agent=claude-1, got %s", data.Agent)
		}
		if data.Type != "claude-code" {
			t.Errorf("expected type=claude-code, got %s", data.Type)
		}
	})

	// --- config set ---
	t.Run("config-set", func(t *testing.T) {
		out := mustSucceed(t, vsName, "config", "set", "claude-1", "--model", "opus", "--skip-permissions")
		data := parseData[ConfigSetData](t, out)

		if data.Agent != "claude-1" {
			t.Errorf("expected agent=claude-1, got %s", data.Agent)
		}
		if data.Config.Model != "opus" {
			t.Errorf("expected model=opus, got %s", data.Config.Model)
		}
		if !data.Config.SkipPermissions {
			t.Error("expected skip_permissions=true")
		}
	})

	// --- config verify (re-show after set) ---
	t.Run("config-verify", func(t *testing.T) {
		out := mustSucceed(t, vsName, "config", "show", "claude-1")
		data := parseData[ConfigShowData](t, out)

		if data.Config.Model != "opus" {
			t.Errorf("expected persisted model=opus, got %s", data.Config.Model)
		}
		if !data.Config.SkipPermissions {
			t.Error("expected persisted skip_permissions=true")
		}
	})

	// --- session list ---
	t.Run("session-list", func(t *testing.T) {
		out := mustSucceed(t, "session", "list")
		// Just verify valid JSON with count field — may be 0 sessions
		data := parseData[SessionListData](t, out)
		t.Logf("sessions: count=%d", data.Count)
	})

	// === Wait for pods to become Running ===

	t.Run("wait-for-ready", func(t *testing.T) {
		waitForReady(t, vsName)
	})

	// === Post-ready tests (needs Running pods) ===

	// --- agent create (second agent) ---
	t.Run("agent-create", func(t *testing.T) {
		out := mustSucceed(t, vsName, "agent", "create", "-t", "claude-code")
		data := parseData[AgentCreateData](t, out)

		if data.Vibespace != vsName {
			t.Errorf("expected vibespace=%s, got %s", vsName, data.Vibespace)
		}
		if data.Agent != "claude-2" {
			t.Errorf("expected agent=claude-2, got %s", data.Agent)
		}
		t.Logf("created agent: %s (type=%s)", data.Agent, data.Type)
	})

	// --- agent list (verify 2 agents) ---
	t.Run("agent-list-two", func(t *testing.T) {
		out := mustSucceed(t, vsName, "agent")
		data := parseData[AgentsData](t, out)

		if data.Count != 2 {
			t.Errorf("expected 2 agents, got %d", data.Count)
		}
	})

	// --- agent delete (remove claude-2) ---
	t.Run("agent-delete", func(t *testing.T) {
		out := mustSucceed(t, vsName, "agent", "delete", "claude-2")
		data := parseData[AgentDeleteData](t, out)

		if data.Agent != "claude-2" {
			t.Errorf("expected agent=claude-2, got %s", data.Agent)
		}
	})

	// --- agent list (verify back to 1) ---
	t.Run("agent-list-one", func(t *testing.T) {
		out := mustSucceed(t, vsName, "agent")
		data := parseData[AgentsData](t, out)

		if data.Count != 1 {
			t.Errorf("expected 1 agent, got %d", data.Count)
		}
	})

	// --- wait for daemon (SSH tunnel ready) ---
	t.Run("wait-for-daemon", func(t *testing.T) {
		// The daemon auto-starts on first forward command. It needs time to:
		// spawn process → reconcile (discover pods) → create kubectl port-forward
		// for SSH (port 2222) → tunnel becomes active. On slow CI runners this
		// takes 10-20+ seconds. We poll until the SSH forward is active.
		waitForDaemonReady(t, vsName)
	})

	// --- exec ---
	t.Run("exec", func(t *testing.T) {
		out := mustSucceed(t, vsName, "exec", "whoami")
		data := parseData[ExecData](t, out)

		if data.Vibespace != vsName {
			t.Errorf("expected vibespace=%s, got %s", vsName, data.Vibespace)
		}
		if strings.TrimSpace(data.Stdout) == "" {
			t.Error("expected non-empty stdout from whoami")
		}
		t.Logf("exec whoami: stdout=%q", strings.TrimSpace(data.Stdout))
	})

	// --- forward list (with default SSH/ttyd forwards) ---
	t.Run("forward-list-default", func(t *testing.T) {
		out := mustSucceed(t, vsName, "forward", "list")
		data := parseData[ForwardsData](t, out)

		if data.Vibespace != vsName {
			t.Errorf("expected vibespace=%s, got %s", vsName, data.Vibespace)
		}
		if len(data.Agents) < 1 {
			t.Errorf("expected at least 1 agent in forward list, got %d", len(data.Agents))
		}
		t.Logf("forward list: %d agents", len(data.Agents))
	})

	// --- forward add ---
	t.Run("forward-add", func(t *testing.T) {
		out := mustSucceed(t, vsName, "forward", "add", "8080")
		data := parseData[ForwardAddData](t, out)

		if data.Vibespace != vsName {
			t.Errorf("expected vibespace=%s, got %s", vsName, data.Vibespace)
		}
		if data.RemotePort != 8080 {
			t.Errorf("expected remote_port=8080, got %d", data.RemotePort)
		}
		if data.LocalPort == 0 {
			t.Error("expected non-zero local_port")
		}
		t.Logf("forward add: local=%d remote=%d", data.LocalPort, data.RemotePort)
	})

	// --- forward list (active) ---
	t.Run("forward-list-active", func(t *testing.T) {
		out := mustSucceed(t, vsName, "forward", "list")
		data := parseData[ForwardsData](t, out)

		found := false
		for _, agent := range data.Agents {
			for _, fwd := range agent.Forwards {
				if fwd.RemotePort == 8080 {
					found = true
				}
			}
		}
		if !found {
			t.Error("expected forward with remote_port=8080 in list")
		}
	})

	// --- forward remove ---
	t.Run("forward-remove", func(t *testing.T) {
		out := mustSucceed(t, vsName, "forward", "remove", "8080")
		data := parseData[ForwardRemoveData](t, out)

		if data.RemotePort != 8080 {
			t.Errorf("expected remote_port=8080, got %d", data.RemotePort)
		}
	})

	// --- ports ---
	t.Run("ports", func(t *testing.T) {
		out := mustSucceed(t, vsName, "ports")
		data := parseData[PortsData](t, out)

		if data.Vibespace != vsName {
			t.Errorf("expected vibespace=%s, got %s", vsName, data.Vibespace)
		}
		// Count may be 0 (no dev server running) — just verify valid JSON
		t.Logf("ports: count=%d", data.Count)
	})

	// --- multi list-sessions ---
	t.Run("multi-list-sessions", func(t *testing.T) {
		out := mustSucceed(t, "multi", "--list-sessions")
		data := parseData[MultiListSessionsData](t, out)

		// May be 0 sessions — just verify valid JSON structure
		t.Logf("multi list-sessions: count=%d", data.Count)
	})

	// --- multi list-agents ---
	t.Run("multi-list-agents", func(t *testing.T) {
		out := mustSucceed(t, "multi", "--vibespaces", vsName, "--list-agents")
		data := parseData[MultiListAgentsData](t, out)

		if len(data.Agents) < 1 {
			t.Errorf("expected at least 1 agent, got %d", len(data.Agents))
		}
		t.Logf("multi list-agents: session=%s agents=%v", data.Session, data.Agents)
	})

	// --- multi message ---
	t.Run("multi-message", func(t *testing.T) {
		// Use runJSON (not mustSucceed) — response may have success=false
		// (e.g., auth error for Claude API) but valid JSON proves the pipeline works.
		out := runJSON(t, "multi", "--vibespaces", vsName, "hello")
		// Valid JSON was parsed — that's the assertion.
		// Log whether it succeeded or what error occurred.
		if out.Success {
			t.Log("multi message: success=true")
		} else {
			errMsg := ""
			if out.Error != nil {
				errMsg = out.Error.Message
			}
			t.Logf("multi message: success=false (expected — no API key): %s", errMsg)
		}
	})

	// --- stop ---
	t.Run("stop", func(t *testing.T) {
		out := mustSucceed(t, vsName, "stop")
		data := parseData[StopData](t, out)

		if !data.Stopped {
			t.Error("expected stopped=true")
		}
	})

	// --- start ---
	t.Run("start", func(t *testing.T) {
		out := mustSucceed(t, vsName, "start")
		data := parseData[StartData](t, out)

		if data.Vibespace != vsName {
			t.Errorf("expected vibespace=%s, got %s", vsName, data.Vibespace)
		}
	})
}

// --- Plain mode subtests ---

// runPlainModeSubtests re-runs all read-only commands with --plain and verifies
// they produce non-empty tab-separated output. Called after runExpandedSubtests
// so all state (vibespace, agents, daemon) already exists.
func runPlainModeSubtests(t *testing.T, vsName string) {
	t.Run("plain/list", func(t *testing.T) {
		out := mustSucceedPlain(t, "list")
		if strings.TrimSpace(out) == "" {
			t.Error("expected non-empty plain output")
		}
		t.Logf("plain list: %s", strings.Split(out, "\n")[0])
	})

	t.Run("plain/info", func(t *testing.T) {
		out := mustSucceedPlain(t, vsName, "info")
		if strings.TrimSpace(out) == "" {
			t.Error("expected non-empty plain output")
		}
	})

	t.Run("plain/agents", func(t *testing.T) {
		out := mustSucceedPlain(t, vsName, "agent", "list")
		if strings.TrimSpace(out) == "" {
			t.Error("expected non-empty plain output")
		}
	})

	t.Run("plain/config-show-all", func(t *testing.T) {
		out := mustSucceedPlain(t, vsName, "config", "show")
		if strings.TrimSpace(out) == "" {
			t.Error("expected non-empty plain output")
		}
	})

	t.Run("plain/config-show", func(t *testing.T) {
		out := mustSucceedPlain(t, vsName, "config", "show", "claude-1")
		if strings.TrimSpace(out) == "" {
			t.Error("expected non-empty plain output")
		}
	})

	t.Run("plain/session-list", func(t *testing.T) {
		// May be empty (no sessions) — just verify exit code 0
		mustSucceedPlain(t, "session", "list")
	})

	t.Run("plain/forward-list", func(t *testing.T) {
		out := mustSucceedPlain(t, vsName, "forward", "list")
		// May be empty if no user forwards active — daemon SSH forwards
		// are internal and may or may not appear in plain output
		t.Logf("plain forward list: %d bytes", len(out))
	})

	t.Run("plain/ports", func(t *testing.T) {
		// May be empty (no dev server running) — just verify exit code 0
		mustSucceedPlain(t, vsName, "ports")
	})

	t.Run("plain/multi-list-sessions", func(t *testing.T) {
		// May be empty — just verify exit code 0
		mustSucceedPlain(t, "multi", "--list-sessions")
	})

	t.Run("plain/multi-list-agents", func(t *testing.T) {
		out := mustSucceedPlain(t, "multi", "--vibespaces", vsName, "--list-agents")
		if strings.TrimSpace(out) == "" {
			t.Error("expected non-empty plain output (at least 1 agent)")
		}
	})
}
