//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vibespacehq/vibespace/pkg/agent"
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

// mustFail runs a JSON command and asserts success=false with an error containing expectedErr.
func mustFail(t *testing.T, expectedErr string, args ...string) jsonapi.RawJSONOutput {
	t.Helper()
	out := runJSON(t, args...)
	if out.Success {
		t.Fatalf("expected failure for %v but got success", args)
	}
	if out.Error == nil {
		t.Fatalf("expected error field in JSON output for %v", args)
	}
	if expectedErr != "" && !strings.Contains(out.Error.Message, expectedErr) {
		t.Errorf("expected error containing %q, got: %s", expectedErr, out.Error.Message)
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
func waitForReady(t *testing.T, vsName string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Minute)
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

// waitForDaemonReady polls `vibespace forward list --vibespace <name> --json` until the daemon
// returns at least 1 agent with an active SSH forward.
func waitForDaemonReady(t *testing.T, vsName string) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for daemon to have agent with active SSH forward for '%s'", vsName)
		}

		out := runJSON(t, "forward", "list", "--vibespace", vsName)
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

// runSubtests runs all E2E subtests between the initial "agents" check and the
// final "delete" step. Called from all 3 platform test files.
func runSubtests(t *testing.T, vsName string) {
	// === Pre-ready tests (k8s metadata only, pods may still be starting) ===

	// --- info ---
	t.Run("info", func(t *testing.T) {
		out := mustSucceed(t, "info", "--vibespace", vsName)
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
		out := mustSucceed(t, "config", "--vibespace", vsName)
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
		out := mustSucceed(t, "config", "show", "claude-1", "--vibespace", vsName)
		data := parseData[jsonapi.ConfigShowOutput](t, out)

		if data.Agent != "claude-1" {
			t.Errorf("expected agent=claude-1, got %s", data.Agent)
		}
		if data.Type != "claude-code" {
			t.Errorf("expected type=claude-code, got %s", data.Type)
		}
	})

	// --- config set (claude-code: --skip-permissions, --model) ---
	t.Run("config-set/claude-code", func(t *testing.T) {
		out := mustSucceed(t, "config", "set", "claude-1", "--model", "opus", "--skip-permissions", "--vibespace", vsName)
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
	t.Run("config-verify/claude-code", func(t *testing.T) {
		out := mustSucceed(t, "config", "show", "claude-1", "--vibespace", vsName)
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
		data := parseData[jsonapi.SessionListOutput](t, out)
		t.Logf("sessions: count=%d", data.Count)
	})

	// === Error tests (don't need running pods) ===

	t.Run("error/delete-nonexistent", func(t *testing.T) {
		mustFail(t, "not found", "delete", "nonexistent-vs", "-f")
	})

	t.Run("error/create-duplicate", func(t *testing.T) {
		mustFail(t, "already exists", "create", vsName, "-t", "claude-code")
	})

	t.Run("error/create-invalid-mount", func(t *testing.T) {
		mustFail(t, "invalid mount", "create", "tempvs", "-t", "claude-code", "--mount", "badformat")
	})

	t.Run("error/info-nonexistent", func(t *testing.T) {
		mustFail(t, "not found", "info", "--vibespace", "nonexistent")
	})

	t.Run("error/agent-delete-nonexistent", func(t *testing.T) {
		mustFail(t, "not found", "agent", "delete", "fake", "--vibespace", vsName)
	})

	t.Run("error/config-show-nonexistent-agent", func(t *testing.T) {
		mustFail(t, "not found", "config", "show", "fake", "--vibespace", vsName)
	})

	t.Run("error/config-set-nonexistent-agent", func(t *testing.T) {
		mustFail(t, "not found", "config", "set", "fake", "--model", "opus", "--vibespace", vsName)
	})

	t.Run("error/config-set-reasoning-claude", func(t *testing.T) {
		mustFail(t, "only supported for Codex", "config", "set", "claude-1", "--reasoning-effort", "high", "--vibespace", vsName)
	})

	t.Run("error/forward-add-invalid-port", func(t *testing.T) {
		mustFail(t, "invalid", "forward", "add", "abc", "--vibespace", vsName)
	})

	t.Run("error/forward-remove-invalid-port", func(t *testing.T) {
		mustFail(t, "invalid", "forward", "remove", "abc", "--vibespace", vsName)
	})

	// === Wait for pods to become Running ===

	t.Run("wait-for-ready", func(t *testing.T) {
		waitForReady(t, vsName)
	})

	// === Post-ready tests (needs Running pods) ===

	// --- wait for daemon (SSH tunnel ready) ---
	t.Run("wait-for-daemon", func(t *testing.T) {
		waitForDaemonReady(t, vsName)
	})

	// --- exec ---
	// Note: exec uses SetInterspersed(false), so --json must come before the
	// command args. We can't use mustSucceed/runJSON which append --json at the end.
	t.Run("exec", func(t *testing.T) {
		r := run(t, "exec", "--vibespace", vsName, "--json", "whoami")
		var out jsonapi.RawJSONOutput
		if err := json.Unmarshal([]byte(r.Stdout), &out); err != nil {
			t.Fatalf("failed to parse JSON output:\nstdout: %s\nstderr: %s\nerror: %v",
				r.Stdout, r.Stderr, err)
		}
		if !out.Success {
			t.Fatalf("expected success but got error: %+v", out.Error)
		}
		data := parseData[jsonapi.ExecOutput](t, out)

		if data.Vibespace != vsName {
			t.Errorf("expected vibespace=%s, got %s", vsName, data.Vibespace)
		}
		if strings.TrimSpace(data.Stdout) == "" {
			t.Error("expected non-empty stdout from whoami")
		}
		t.Logf("exec whoami: stdout=%q", strings.TrimSpace(data.Stdout))
	})

	// --- exec error tests (need daemon) ---
	t.Run("error/exec-nonexistent-agent", func(t *testing.T) {
		r := run(t, "exec", "--vibespace", vsName, "--json", "nonexistent", "whoami")
		var out jsonapi.RawJSONOutput
		if err := json.Unmarshal([]byte(r.Stdout), &out); err != nil {
			t.Fatalf("failed to parse JSON output:\nstdout: %s\nstderr: %s\nerror: %v",
				r.Stdout, r.Stderr, err)
		}
		if out.Success {
			t.Fatal("expected failure but got success")
		}
		if out.Error == nil || !strings.Contains(out.Error.Message, "not found") {
			t.Errorf("expected 'not found' error, got: %+v", out.Error)
		}
	})

	// --- forward list (with default SSH/ttyd forwards) ---
	t.Run("forward-list-default", func(t *testing.T) {
		out := mustSucceed(t, "forward", "list", "--vibespace", vsName)
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
		out := mustSucceed(t, "forward", "add", "8080", "--vibespace", vsName)
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
		out := mustSucceed(t, "forward", "list", "--vibespace", vsName)
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
		out := mustSucceed(t, "forward", "remove", "8080", "--vibespace", vsName)
		data := parseData[jsonapi.ForwardRemoveOutput](t, out)

		if data.RemotePort != 8080 {
			t.Errorf("expected remote_port=8080, got %d", data.RemotePort)
		}
	})

	// --- multi list-sessions ---
	t.Run("multi-list-sessions", func(t *testing.T) {
		out := mustSucceed(t, "multi", "--list-sessions")
		data := parseData[jsonapi.MultiListSessionsOutput](t, out)

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
		out := runJSON(t, "multi", "--vibespaces", vsName, "hello")
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

	// === Agent CRUD + config set tests (parameterized over all agent types) ===

	for _, agentType := range agent.AllTypes() {
		agentType := agentType // capture
		typeName := agentType.String()
		testAgentName := fmt.Sprintf("%s-test", typeName)

		t.Run(fmt.Sprintf("agent-crud/%s", typeName), func(t *testing.T) {
			// create
			t.Run("create", func(t *testing.T) {
				out := mustSucceed(t, "agent", "create", "--vibespace", vsName,
					"--agent-type", typeName, "--name", testAgentName)
				data := parseData[jsonapi.AgentCreateOutput](t, out)

				if data.Vibespace != vsName {
					t.Errorf("expected vibespace=%s, got %s", vsName, data.Vibespace)
				}
				if data.Agent != testAgentName {
					t.Errorf("expected agent=%s, got %s", testAgentName, data.Agent)
				}
				if data.Type != typeName {
					t.Errorf("expected type=%s, got %s", typeName, data.Type)
				}
				t.Logf("created agent: %s (type=%s)", data.Agent, data.Type)
			})

			// list — verify agent appears with correct type
			t.Run("list", func(t *testing.T) {
				out := mustSucceed(t, "agent", "list", "--vibespace", vsName)
				data := parseData[jsonapi.AgentsOutput](t, out)

				found := false
				for _, a := range data.Agents {
					if a.Name == testAgentName && a.Type == typeName {
						found = true
					}
				}
				if !found {
					t.Errorf("expected %s with type=%s in list, got: %+v", testAgentName, typeName, data.Agents)
				}
			})

			// config set — type-specific flags
			t.Run("config-set", func(t *testing.T) {
				var setArgs []string
				switch agentType {
				case agent.TypeClaudeCode:
					setArgs = []string{"config", "set", testAgentName,
						"--model", "sonnet", "--skip-permissions", "--max-turns", "5",
						"--vibespace", vsName}
				case agent.TypeCodex:
					setArgs = []string{"config", "set", testAgentName,
						"--model", "gpt-5.2-codex", "--reasoning-effort", "high", "--max-turns", "8",
						"--vibespace", vsName}
				}

				out := mustSucceed(t, setArgs...)
				data := parseData[jsonapi.ConfigSetOutput](t, out)

				if data.Agent != testAgentName {
					t.Errorf("expected agent=%s, got %s", testAgentName, data.Agent)
				}
			})

			// config verify — check persisted values
			t.Run("config-verify", func(t *testing.T) {
				out := mustSucceed(t, "config", "show", testAgentName, "--vibespace", vsName)
				data := parseData[jsonapi.ConfigShowOutput](t, out)

				switch agentType {
				case agent.TypeClaudeCode:
					if data.Config.Model != "sonnet" {
						t.Errorf("expected model=sonnet, got %s", data.Config.Model)
					}
					if !data.Config.SkipPermissions {
						t.Error("expected skip_permissions=true")
					}
					if data.Config.MaxTurns != 5 {
						t.Errorf("expected max_turns=5, got %d", data.Config.MaxTurns)
					}
				case agent.TypeCodex:
					if data.Config.Model != "gpt-5.2-codex" {
						t.Errorf("expected model=gpt-5.2-codex, got %s", data.Config.Model)
					}
					if data.Config.ReasoningEffort != "high" {
						t.Errorf("expected reasoning_effort=high, got %s", data.Config.ReasoningEffort)
					}
					if data.Config.MaxTurns != 8 {
						t.Errorf("expected max_turns=8, got %d", data.Config.MaxTurns)
					}
				}
			})

			// config error tests — type-specific restrictions
			if agentType == agent.TypeCodex {
				t.Run("error/config-set-skip-perms", func(t *testing.T) {
					mustFail(t, "not supported for Codex", "config", "set", testAgentName, "--skip-permissions", "--vibespace", vsName)
				})
				t.Run("error/config-set-no-skip-perms", func(t *testing.T) {
					mustFail(t, "not supported for Codex", "config", "set", testAgentName, "--no-skip-permissions", "--vibespace", vsName)
				})
				t.Run("error/config-set-allowed-tools", func(t *testing.T) {
					mustFail(t, "not supported for Codex", "config", "set", testAgentName, "--allowed-tools", "Bash", "--vibespace", vsName)
				})
				t.Run("error/config-set-disallowed-tools", func(t *testing.T) {
					mustFail(t, "not supported for Codex", "config", "set", testAgentName, "--disallowed-tools", "Bash", "--vibespace", vsName)
				})
				t.Run("error/config-set-bad-reasoning", func(t *testing.T) {
					mustFail(t, "invalid reasoning effort", "config", "set", testAgentName, "--reasoning-effort", "banana", "--vibespace", vsName)
				})
			}

			// delete
			t.Run("delete", func(t *testing.T) {
				out := mustSucceed(t, "agent", "delete", testAgentName, "--vibespace", vsName)
				data := parseData[jsonapi.AgentDeleteOutput](t, out)

				if data.Agent != testAgentName {
					t.Errorf("expected agent=%s, got %s", testAgentName, data.Agent)
				}
			})
		})
	}

	// --- verify back to 1 agent after type tests ---
	t.Run("agent-list-after-crud", func(t *testing.T) {
		out := mustSucceed(t, "agent", "list", "--vibespace", vsName)
		data := parseData[jsonapi.AgentsOutput](t, out)

		if data.Count != 1 {
			t.Errorf("expected 1 agent, got %d", data.Count)
		}
	})

	// === Agent create with config flags ===

	t.Run("agent-create-with-flags", func(t *testing.T) {
		out := mustSucceed(t, "agent", "create", "--vibespace", vsName,
			"--agent-type", "claude-code",
			"--name", "flagtest",
			"--skip-permissions",
			"--model", "sonnet",
			"--max-turns", "10",
		)
		data := parseData[jsonapi.AgentCreateOutput](t, out)

		if data.Agent != "flagtest" {
			t.Errorf("expected agent=flagtest, got %s", data.Agent)
		}
	})

	t.Run("agent-verify-create-flags", func(t *testing.T) {
		out := mustSucceed(t, "config", "show", "flagtest", "--vibespace", vsName)
		data := parseData[jsonapi.ConfigShowOutput](t, out)

		if data.Config.Model != "sonnet" {
			t.Errorf("expected model=sonnet, got %s", data.Config.Model)
		}
		if !data.Config.SkipPermissions {
			t.Error("expected skip_permissions=true")
		}
		if data.Config.MaxTurns != 10 {
			t.Errorf("expected max_turns=10, got %d", data.Config.MaxTurns)
		}
	})

	t.Run("agent-delete-flagtest", func(t *testing.T) {
		mustSucceed(t, "agent", "delete", "flagtest", "--vibespace", vsName)
	})

	// === Plain mode subtests ===

	t.Run("plain/list", func(t *testing.T) {
		out := mustSucceedPlain(t, "list")
		if strings.TrimSpace(out) == "" {
			t.Error("expected non-empty plain output")
		}
		t.Logf("plain list: %s", strings.Split(out, "\n")[0])
	})

	t.Run("plain/info", func(t *testing.T) {
		out := mustSucceedPlain(t, "info", "--vibespace", vsName)
		if strings.TrimSpace(out) == "" {
			t.Error("expected non-empty plain output")
		}
	})

	t.Run("plain/agents", func(t *testing.T) {
		out := mustSucceedPlain(t, "agent", "list", "--vibespace", vsName)
		if strings.TrimSpace(out) == "" {
			t.Error("expected non-empty plain output")
		}
	})

	t.Run("plain/config-show-all", func(t *testing.T) {
		out := mustSucceedPlain(t, "config", "show", "--vibespace", vsName)
		if strings.TrimSpace(out) == "" {
			t.Error("expected non-empty plain output")
		}
	})

	t.Run("plain/config-show", func(t *testing.T) {
		out := mustSucceedPlain(t, "config", "show", "claude-1", "--vibespace", vsName)
		if strings.TrimSpace(out) == "" {
			t.Error("expected non-empty plain output")
		}
	})

	t.Run("plain/session-list", func(t *testing.T) {
		mustSucceedPlain(t, "session", "list")
	})

	t.Run("plain/forward-list", func(t *testing.T) {
		out := mustSucceedPlain(t, "forward", "list", "--vibespace", vsName)
		t.Logf("plain forward list: %d bytes", len(out))
	})

	t.Run("plain/multi-list-sessions", func(t *testing.T) {
		mustSucceedPlain(t, "multi", "--list-sessions")
	})

	t.Run("plain/multi-list-agents", func(t *testing.T) {
		mustSucceedPlain(t, "multi", "--vibespaces", vsName, "--list-agents")
	})

	// === Default output mode subtests ===

	t.Run("default/list", func(t *testing.T) {
		mustSucceedDefault(t, "list")
	})

	t.Run("default/info", func(t *testing.T) {
		mustSucceedDefault(t, "info", "--vibespace", vsName)
	})

	t.Run("default/agents", func(t *testing.T) {
		mustSucceedDefault(t, "agent", "list", "--vibespace", vsName)
	})

	t.Run("default/config-show-all", func(t *testing.T) {
		mustSucceedDefault(t, "config", "show", "--vibespace", vsName)
	})

	t.Run("default/config-show", func(t *testing.T) {
		mustSucceedDefault(t, "config", "show", "claude-1", "--vibespace", vsName)
	})

	t.Run("default/session-list", func(t *testing.T) {
		r := run(t, "session", "list")
		if r.ExitCode != 0 {
			t.Fatalf("expected exit code 0 but got %d\nstdout: %s\nstderr: %s",
				r.ExitCode, r.Stdout, r.Stderr)
		}
	})

	t.Run("default/forward-list", func(t *testing.T) {
		mustSucceedDefault(t, "forward", "list", "--vibespace", vsName)
	})

	t.Run("default/multi-list-sessions", func(t *testing.T) {
		r := run(t, "multi", "--list-sessions")
		if r.ExitCode != 0 {
			t.Fatalf("expected exit code 0 but got %d\nstdout: %s\nstderr: %s",
				r.ExitCode, r.Stdout, r.Stderr)
		}
	})

	t.Run("default/multi-list-agents", func(t *testing.T) {
		r := run(t, "multi", "--vibespaces", vsName, "--list-agents")
		if r.ExitCode != 0 {
			t.Fatalf("expected exit code 0 but got %d\nstdout: %s\nstderr: %s",
				r.ExitCode, r.Stdout, r.Stderr)
		}
	})

	t.Run("default/status", func(t *testing.T) {
		mustSucceedDefault(t, "status")
	})

	// === Agent start/stop specific agent (last — restarts pods) ===

	t.Run("agent-stop-specific", func(t *testing.T) {
		out := mustSucceed(t, "agent", "stop", "claude-1", "--vibespace", vsName)
		data := parseData[jsonapi.StopOutput](t, out)

		if !data.Stopped {
			t.Error("expected stopped=true")
		}
		if data.Target != "claude-1" {
			t.Errorf("expected target=claude-1, got %s", data.Target)
		}
	})

	t.Run("agent-start-specific", func(t *testing.T) {
		out := mustSucceed(t, "agent", "start", "claude-1", "--vibespace", vsName)
		data := parseData[jsonapi.StartOutput](t, out)

		if data.Agent != "claude-1" {
			t.Errorf("expected agent=claude-1, got %s", data.Agent)
		}
	})

	// --- stop all ---
	t.Run("stop", func(t *testing.T) {
		out := mustSucceed(t, "agent", "stop", "--vibespace", vsName)
		data := parseData[jsonapi.StopOutput](t, out)

		if !data.Stopped {
			t.Error("expected stopped=true")
		}
	})

	// --- error: agent create on stopped vibespace ---
	t.Run("error/agent-create-stopped-vs", func(t *testing.T) {
		mustFail(t, "stopped", "agent", "create", "--vibespace", vsName, "-t", "claude-code")
	})

	// --- start all ---
	t.Run("start", func(t *testing.T) {
		out := mustSucceed(t, "agent", "start", "--vibespace", vsName)
		data := parseData[jsonapi.StartOutput](t, out)

		if data.Vibespace != vsName {
			t.Errorf("expected vibespace=%s, got %s", vsName, data.Vibespace)
		}
	})
}
