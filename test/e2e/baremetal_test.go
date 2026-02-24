//go:build e2e && linux

package e2e

import (
	"testing"

	"github.com/vibespacehq/vibespace/pkg/jsonapi"
)

// TestBareMetalLifecycle runs a full lifecycle test of the vibespace binary
// in bare metal mode: init → status → create → list → agents → delete → verify.
//
// This test requires a Linux environment (ubuntu-latest CI runner) and will
// install k3s directly on the host. It uses t.Cleanup to uninstall afterward.
func TestBareMetalLifecycle(t *testing.T) {
	// Always clean up — uninstall vibespace even if the test fails.
	// CI runners are ephemeral, but this keeps things tidy.
	t.Cleanup(func() {
		t.Log("cleanup: uninstalling vibespace")
		r := run(t, "uninstall", "--force")
		t.Logf("cleanup stdout: %s", r.Stdout)
		t.Logf("cleanup stderr: %s", r.Stderr)
	})

	// --- init ---
	t.Run("init", func(t *testing.T) {
		r := run(t, "init", "--bare-metal", "--cpu", "2", "--memory", "2", "--disk", "10")
		t.Logf("stdout: %s", r.Stdout)
		t.Logf("stderr: %s", r.Stderr)
		if r.ExitCode != 0 {
			t.Fatalf("init failed with exit code %d\nstderr: %s", r.ExitCode, r.Stderr)
		}
	})

	// --- status ---
	t.Run("status", func(t *testing.T) {
		out := mustSucceed(t, "status")
		data := parseData[jsonapi.StatusOutput](t, out)

		if !data.Cluster.Installed {
			t.Error("expected cluster.installed=true")
		}
		if !data.Cluster.Running {
			t.Error("expected cluster.running=true")
		}
		if data.Cluster.Platform != "linux" {
			t.Errorf("expected platform=linux, got %s", data.Cluster.Platform)
		}
	})

	// --- create ---
	var vibespaceID string
	t.Run("create", func(t *testing.T) {
		out := mustSucceed(t, "create", "e2e-test", "-t", "claude-code")
		data := parseData[jsonapi.CreateOutput](t, out)

		if data.Name != "e2e-test" {
			t.Errorf("expected name=e2e-test, got %s", data.Name)
		}
		if data.ID == "" {
			t.Error("expected non-empty id")
		}
		vibespaceID = data.ID
		t.Logf("created vibespace: name=%s id=%s", data.Name, data.ID)
	})

	if vibespaceID == "" {
		t.Fatal("create did not return a vibespace ID, cannot continue")
	}

	// --- list ---
	t.Run("list", func(t *testing.T) {
		out := mustSucceed(t, "list")
		data := parseData[jsonapi.ListOutput](t, out)

		found := false
		for _, vs := range data.Vibespaces {
			if vs.Name == "e2e-test" {
				found = true
				if vs.Agents < 1 {
					t.Errorf("expected at least 1 agent, got %d", vs.Agents)
				}
			}
		}
		if !found {
			t.Errorf("vibespace 'e2e-test' not found in list: %+v", data.Vibespaces)
		}
	})

	// --- agents ---
	t.Run("agents", func(t *testing.T) {
		out := mustSucceed(t, "e2e-test", "agent")
		data := parseData[jsonapi.AgentsOutput](t, out)

		if data.Count < 1 {
			t.Fatalf("expected at least 1 agent, got %d", data.Count)
		}

		foundClaudeCode := false
		for _, a := range data.Agents {
			if a.Type == "claude-code" {
				foundClaudeCode = true
			}
		}
		if !foundClaudeCode {
			t.Errorf("expected a claude-code agent, got: %+v", data.Agents)
		}
	})

	// --- expanded subtests (info, config, exec, forward, ports, multi, agent CRUD, plain, stop, start) ---
	runExpandedSubtests(t, "e2e-test")

	// --- delete ---
	t.Run("delete", func(t *testing.T) {
		out := mustSucceed(t, "delete", "e2e-test", "-f")
		data := parseData[jsonapi.DeleteOutput](t, out)

		if data.Name != "e2e-test" {
			t.Errorf("expected name=e2e-test, got %s", data.Name)
		}
	})

	// --- verify deletion ---
	t.Run("verify-deleted", func(t *testing.T) {
		out := mustSucceed(t, "list")
		data := parseData[jsonapi.ListOutput](t, out)

		for _, vs := range data.Vibespaces {
			if vs.Name == "e2e-test" {
				t.Errorf("vibespace 'e2e-test' still exists after delete: %+v", vs)
			}
		}
	})
}
