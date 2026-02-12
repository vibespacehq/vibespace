package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/vibespacehq/vibespace/pkg/agent"
	"github.com/vibespacehq/vibespace/pkg/model"
	"github.com/vibespacehq/vibespace/pkg/ui"
	"github.com/vibespacehq/vibespace/pkg/vibespace"
)

// captureStdout captures stdout output from a function.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldStdout := os.Stdout
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = oldStdout

	var buf [64 * 1024]byte
	n, _ := r.Read(buf[:])
	r.Close()
	return string(buf[:n])
}

func TestColorStatus(t *testing.T) {
	out := NewOutput(OutputConfig{NoColor: true})

	tests := []struct {
		status string
		want   string
	}{
		{"running", "running"},
		{"stopped", "stopped"},
		{"creating", "creating"},
		{"error", "error"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := colorStatus(out, tt.status)
			if got != tt.want {
				t.Errorf("colorStatus(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestRenderInfoRich(t *testing.T) {
	initOutput(OutputConfig{NoColor: true})

	info := InfoOutput{
		Name:      "test",
		ID:        "e27ed7c0",
		Status:    "running",
		PVC:       "vibespace-e27ed7c0-pvc",
		CPU:       "250m",
		Memory:    "512Mi",
		Storage:   "10Gi",
		CreatedAt: "2025-01-15 10:30:00",
	}

	vs := &model.Vibespace{
		Name:   "test",
		ID:     "e27ed7c0",
		Status: "running",
	}

	agents := []vibespace.AgentInfo{
		{AgentName: "claude-1", AgentType: agent.TypeClaudeCode, Status: "running"},
	}

	agentConfigs := map[string]*agent.Config{
		"claude-1": {Model: "opus"},
	}

	got := captureStdout(t, func() {
		renderInfoRich(info, vs, agents, agentConfigs, nil, true)
	})
	stripped := ui.StripAnsi(got)

	// Check key fields are present
	for _, want := range []string{"test", "e27ed7c0", "running", "250m", "512Mi", "10Gi", "Created"} {
		if !strings.Contains(stripped, want) {
			t.Errorf("renderInfoRich output missing %q:\n%s", want, stripped)
		}
	}

	// Check labels
	for _, label := range []string{"ID", "Status", "PVC", "CPU", "Memory", "Storage"} {
		if !strings.Contains(stripped, label) {
			t.Errorf("renderInfoRich output missing label %q:\n%s", label, stripped)
		}
	}
}

func TestRenderInfoRichWithMounts(t *testing.T) {
	initOutput(OutputConfig{NoColor: true})

	info := InfoOutput{
		Name:    "test",
		ID:      "abc123",
		Status:  "running",
		PVC:     "vibespace-abc123-pvc",
		CPU:     "500m",
		Memory:  "1Gi",
		Storage: "20Gi",
		Mounts: []MountInfo{
			{HostPath: "/home/user/code", ContainerPath: "/workspace", ReadOnly: false},
			{HostPath: "/home/user/data", ContainerPath: "/data", ReadOnly: true},
		},
		CreatedAt: "2025-01-15",
	}

	vs := &model.Vibespace{Name: "test", ID: "abc123", Status: "running"}

	got := captureStdout(t, func() {
		renderInfoRich(info, vs, nil, nil, nil, true)
	})
	stripped := ui.StripAnsi(got)

	if !strings.Contains(stripped, "Mounts") {
		t.Error("output should contain Mounts section")
	}
	if !strings.Contains(stripped, "/workspace") {
		t.Error("output should contain mount container path")
	}
	if !strings.Contains(stripped, "(rw)") {
		t.Error("output should contain rw mode")
	}
	if !strings.Contains(stripped, "(ro)") {
		t.Error("output should contain ro mode")
	}
}

func TestRenderInfoRichWithForwards(t *testing.T) {
	initOutput(OutputConfig{NoColor: true})

	info := InfoOutput{
		Name:      "test",
		ID:        "abc123",
		Status:    "running",
		PVC:       "vibespace-abc123-pvc",
		CPU:       "250m",
		Memory:    "512Mi",
		Storage:   "10Gi",
		CreatedAt: "2025-01-15",
	}

	vs := &model.Vibespace{Name: "test", ID: "abc123", Status: "running"}

	forwards := []AgentForwardInfo{
		{
			Name: "claude-1",
			Forwards: []ForwardInfo{
				{LocalPort: 8080, RemotePort: 8080, Type: "manual", Status: "active"},
				{LocalPort: 3000, RemotePort: 3000, Type: "auto", Status: "active"},
			},
		},
	}

	got := captureStdout(t, func() {
		renderInfoRich(info, vs, nil, nil, forwards, true)
	})
	stripped := ui.StripAnsi(got)

	if !strings.Contains(stripped, "Forwards") {
		t.Error("output should contain Forwards section")
	}
	if !strings.Contains(stripped, ":8080") {
		t.Error("output should contain port 8080")
	}
	if !strings.Contains(stripped, "[manual]") {
		t.Error("output should contain forward type")
	}
	if !strings.Contains(stripped, "[auto]") {
		t.Error("output should contain auto forward type")
	}
}

func TestRenderInfoRichForwardWithError(t *testing.T) {
	initOutput(OutputConfig{NoColor: true})

	info := InfoOutput{
		Name: "test", ID: "abc", Status: "running",
		PVC: "pvc", CPU: "100m", Memory: "256Mi", Storage: "5Gi",
		CreatedAt: "2025-01-15",
	}
	vs := &model.Vibespace{Name: "test", ID: "abc", Status: "running"}

	forwards := []AgentForwardInfo{
		{
			Name: "claude-1",
			Forwards: []ForwardInfo{
				{LocalPort: 8080, RemotePort: 8080, Status: "error", Error: "connection refused"},
			},
		},
	}

	got := captureStdout(t, func() {
		renderInfoRich(info, vs, nil, nil, forwards, true)
	})

	if !strings.Contains(got, "connection refused") {
		t.Error("output should contain forward error message")
	}
}

func TestRenderInfoRichForwardDefaultType(t *testing.T) {
	initOutput(OutputConfig{NoColor: true})

	info := InfoOutput{
		Name: "test", ID: "abc", Status: "running",
		PVC: "pvc", CPU: "100m", Memory: "256Mi", Storage: "5Gi",
		CreatedAt: "2025-01-15",
	}
	vs := &model.Vibespace{Name: "test", ID: "abc", Status: "running"}

	forwards := []AgentForwardInfo{
		{
			Name: "claude-1",
			Forwards: []ForwardInfo{
				{LocalPort: 8080, RemotePort: 8080, Type: "", Status: "active"},
			},
		},
	}

	got := captureStdout(t, func() {
		renderInfoRich(info, vs, nil, nil, forwards, true)
	})

	if !strings.Contains(got, "[manual]") {
		t.Error("empty forward type should default to 'manual'")
	}
}

func TestPrintAgentConfigForInfo(t *testing.T) {
	out := NewOutput(OutputConfig{NoColor: true})

	tests := []struct {
		name      string
		agentName string
		agentType agent.Type
		status    string
		config    *agent.Config
		wantParts []string
	}{
		{
			name:      "claude running default",
			agentName: "claude-1",
			agentType: agent.TypeClaudeCode,
			status:    "running",
			config:    &agent.Config{},
			wantParts: []string{"claude-1", "running", "skip_permissions=false", "model=default"},
		},
		{
			name:      "claude with model",
			agentName: "claude-2",
			agentType: agent.TypeClaudeCode,
			status:    "creating",
			config:    &agent.Config{Model: "opus", SkipPermissions: true},
			wantParts: []string{"claude-2", "creating", "skip_permissions=true", "model=opus"},
		},
		{
			name:      "codex always skip",
			agentName: "codex-1",
			agentType: agent.TypeCodex,
			status:    "running",
			config:    &agent.Config{},
			wantParts: []string{"codex-1", "running", "skip_permissions=always", "model=default"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := captureStdout(t, func() {
				printAgentConfigForInfo(tt.agentName, tt.agentType, tt.status, tt.config, out)
			})
			for _, want := range tt.wantParts {
				if !strings.Contains(got, want) {
					t.Errorf("output missing %q:\n%s", want, got)
				}
			}
		})
	}
}

func TestRenderInfoPlain(t *testing.T) {
	info := InfoOutput{
		Name:      "test",
		ID:        "abc123",
		Status:    "running",
		PVC:       "vibespace-abc123-pvc",
		CPU:       "250m",
		Memory:    "512Mi",
		Storage:   "10Gi",
		CreatedAt: "2025-01-15",
		Agents: []AgentInfoOutput{
			{Name: "claude-1", Type: "claude-code", Status: "running", Config: AgentConfigOutput{Model: "opus"}},
		},
		Mounts: []MountInfo{
			{HostPath: "/home/user/code", ContainerPath: "/workspace"},
		},
		Forwards: []AgentForwardInfo{
			{Name: "claude-1", Forwards: []ForwardInfo{
				{LocalPort: 8080, RemotePort: 8080, Type: "manual", Status: "active"},
			}},
		},
	}

	// Without header
	got := captureStdout(t, func() {
		renderInfoPlain(info, false)
	})

	if strings.Contains(got, "KEY\tVALUE") {
		t.Error("should not have header when header=false")
	}
	if !strings.Contains(got, "name\ttest") {
		t.Error("should contain name field")
	}
	if !strings.Contains(got, "agent.claude-1.type\tclaude-code") {
		t.Error("should contain agent type")
	}

	// With header
	gotH := captureStdout(t, func() {
		renderInfoPlain(info, true)
	})
	if !strings.Contains(gotH, "KEY\tVALUE") {
		t.Error("should have header when header=true")
	}
}

func TestShortenPath(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		input string
		want  string
	}{
		{home + "/code", "~/code"},
		{home + "/Documents/project", "~/Documents/project"},
		{"/var/lib/data", "/var/lib/data"},
		{"/tmp/test", "/tmp/test"},
	}

	for _, tt := range tests {
		got := shortenPath(tt.input)
		if got != tt.want {
			t.Errorf("shortenPath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
