//go:build e2e

package e2e

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
	cmd.Env = append(os.Environ(), "NO_COLOR=1", "VIBESPACE_DEBUG=1")

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
