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

	"github.com/vibespacehq/vibespace/pkg/jsonapi"
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
func runJSON(t *testing.T, args ...string) jsonapi.RawJSONOutput {
	t.Helper()
	args = append(args, "--json")
	r := run(t, args...)

	var out jsonapi.RawJSONOutput
	if err := json.Unmarshal([]byte(r.Stdout), &out); err != nil {
		t.Fatalf("failed to parse JSON output from %v:\nstdout: %s\nstderr: %s\nerror: %v",
			args, r.Stdout, r.Stderr, err)
	}
	return out
}

// mustSucceed runs a JSON command and asserts success=true.
func mustSucceed(t *testing.T, args ...string) jsonapi.RawJSONOutput {
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

// parseData unmarshals RawJSONOutput.Data into the given type.
func parseData[T any](t *testing.T, out jsonapi.RawJSONOutput) T {
	t.Helper()
	v, err := jsonapi.ParseData[T](out.Data)
	if err != nil {
		t.Fatalf("failed to unmarshal data into %T: %v\nraw: %s", *new(T), err, string(out.Data))
	}
	return v
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
			data := parseData[jsonapi.ListOutput](t, out)
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
			data := parseData[jsonapi.ForwardsOutput](t, out)
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

// mustSucceedDefault runs a command without --json or --plain (default output mode)
// and asserts exit code 0 with non-empty stdout.
func mustSucceedDefault(t *testing.T, args ...string) string {
	t.Helper()
	r := run(t, args...)
	if r.ExitCode != 0 {
		t.Fatalf("expected exit code 0 for %v (default mode) but got %d\nstdout: %s\nstderr: %s",
			args, r.ExitCode, r.Stdout, r.Stderr)
	}
	if strings.TrimSpace(r.Stdout) == "" {
		t.Fatalf("expected non-empty stdout for %v (default mode)\nstderr: %s",
			args, r.Stderr)
	}
	return r.Stdout
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
		data := parseData[jsonapi.InfoOutput](t, out)

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
		data := parseData[jsonapi.ConfigShowAllOutput](t, out)

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
		data := parseData[jsonapi.ConfigShowOutput](t, out)

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
		data := parseData[jsonapi.ConfigSetOutput](t, out)

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
		data := parseData[jsonapi.ConfigShowOutput](t, out)

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
		data := parseData[jsonapi.SessionListOutput](t, out)
		t.Logf("sessions: count=%d", data.Count)
	})

	// === Wait for pods to become Running ===

	t.Run("wait-for-ready", func(t *testing.T) {
		waitForReady(t, vsName)
	})

	// === Post-ready tests (needs Running pods) ===
	// NOTE: Forward/multi tests run BEFORE agent CRUD to avoid daemon stale-state
	// issues. On colima, deleting claude-2 leaves the daemon with stale SSH tunnels
	// and the portforward manager may not re-register claude-1 before forward-add
	// runs (~30s reconciliation interval). Running forward/multi first ensures the
	// daemon has clean state with only claude-1 registered.

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
		data := parseData[jsonapi.ExecOutput](t, out)

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
		data := parseData[jsonapi.ForwardsOutput](t, out)

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
		data := parseData[jsonapi.ForwardAddOutput](t, out)

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
		data := parseData[jsonapi.ForwardsOutput](t, out)

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
		data := parseData[jsonapi.ForwardRemoveOutput](t, out)

		if data.RemotePort != 8080 {
			t.Errorf("expected remote_port=8080, got %d", data.RemotePort)
		}
	})

	// --- multi list-sessions ---
	t.Run("multi-list-sessions", func(t *testing.T) {
		out := mustSucceed(t, "multi", "--list-sessions")
		data := parseData[jsonapi.MultiListSessionsOutput](t, out)

		// May be 0 sessions — just verify valid JSON structure
		t.Logf("multi list-sessions: count=%d", data.Count)
	})

	// --- multi list-agents ---
	t.Run("multi-list-agents", func(t *testing.T) {
		out := mustSucceed(t, "multi", "--vibespaces", vsName, "--list-agents")
		data := parseData[jsonapi.MultiListAgentsOutput](t, out)

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

	// === Agent CRUD tests ===
	// These run after forward/multi tests to avoid daemon stale-state issues.

	// --- agent create (second agent) ---
	t.Run("agent-create", func(t *testing.T) {
		out := mustSucceed(t, vsName, "agent", "create", "-t", "claude-code")
		data := parseData[jsonapi.AgentCreateOutput](t, out)

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
		data := parseData[jsonapi.AgentsOutput](t, out)

		if data.Count != 2 {
			t.Errorf("expected 2 agents, got %d", data.Count)
		}
	})

	// --- agent delete (remove claude-2) ---
	t.Run("agent-delete", func(t *testing.T) {
		out := mustSucceed(t, vsName, "agent", "delete", "claude-2")
		data := parseData[jsonapi.AgentDeleteOutput](t, out)

		if data.Agent != "claude-2" {
			t.Errorf("expected agent=claude-2, got %s", data.Agent)
		}
	})

	// --- agent list (verify back to 1) ---
	t.Run("agent-list-one", func(t *testing.T) {
		out := mustSucceed(t, vsName, "agent")
		data := parseData[jsonapi.AgentsOutput](t, out)

		if data.Count != 1 {
			t.Errorf("expected 1 agent, got %d", data.Count)
		}
	})

	// === Plain mode subtests (run while pods are still Running, before stop/start) ===

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
		t.Logf("plain forward list: %d bytes", len(out))
	})

	t.Run("plain/multi-list-sessions", func(t *testing.T) {
		// May be empty — just verify exit code 0
		mustSucceedPlain(t, "multi", "--list-sessions")
	})

	t.Run("plain/multi-list-agents", func(t *testing.T) {
		// Just verify exit code 0 — agent count validated by JSON multi-list-agents test.
		// After agent-delete the daemon may have stale state, so agent list can be empty.
		mustSucceedPlain(t, "multi", "--vibespaces", vsName, "--list-agents")
	})

	// === Default output mode subtests (no --json, no --plain) ===
	// Exercises the colored table/spinner code paths that are otherwise untested.

	t.Run("default/list", func(t *testing.T) {
		mustSucceedDefault(t, "list")
	})

	t.Run("default/info", func(t *testing.T) {
		mustSucceedDefault(t, vsName, "info")
	})

	t.Run("default/agents", func(t *testing.T) {
		mustSucceedDefault(t, vsName, "agent", "list")
	})

	t.Run("default/config-show-all", func(t *testing.T) {
		mustSucceedDefault(t, vsName, "config", "show")
	})

	t.Run("default/config-show", func(t *testing.T) {
		mustSucceedDefault(t, vsName, "config", "show", "claude-1")
	})

	t.Run("default/session-list", func(t *testing.T) {
		// May have 0 sessions — just verify exit code 0, don't require non-empty stdout.
		r := run(t, "session", "list")
		if r.ExitCode != 0 {
			t.Fatalf("expected exit code 0 but got %d\nstdout: %s\nstderr: %s",
				r.ExitCode, r.Stdout, r.Stderr)
		}
	})

	t.Run("default/forward-list", func(t *testing.T) {
		mustSucceedDefault(t, vsName, "forward", "list")
	})

	t.Run("default/multi-list-sessions", func(t *testing.T) {
		// May have 0 sessions — just verify exit code 0.
		r := run(t, "multi", "--list-sessions")
		if r.ExitCode != 0 {
			t.Fatalf("expected exit code 0 but got %d\nstdout: %s\nstderr: %s",
				r.ExitCode, r.Stdout, r.Stderr)
		}
	})

	t.Run("default/multi-list-agents", func(t *testing.T) {
		// After agent-delete the daemon may have stale state, so just verify exit code 0.
		r := run(t, "multi", "--vibespaces", vsName, "--list-agents")
		if r.ExitCode != 0 {
			t.Fatalf("expected exit code 0 but got %d\nstdout: %s\nstderr: %s",
				r.ExitCode, r.Stdout, r.Stderr)
		}
	})

	t.Run("default/status", func(t *testing.T) {
		mustSucceedDefault(t, "status")
	})

	// --- stop ---
	t.Run("stop", func(t *testing.T) {
		out := mustSucceed(t, vsName, "stop")
		data := parseData[jsonapi.StopOutput](t, out)

		if !data.Stopped {
			t.Error("expected stopped=true")
		}
	})

	// --- start ---
	t.Run("start", func(t *testing.T) {
		out := mustSucceed(t, vsName, "start")
		data := parseData[jsonapi.StartOutput](t, out)

		if data.Vibespace != vsName {
			t.Errorf("expected vibespace=%s, got %s", vsName, data.Vibespace)
		}
	})
}
