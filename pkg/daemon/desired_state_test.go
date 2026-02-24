package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func newTestDesiredStateManager(t *testing.T) *DesiredStateManager {
	t.Helper()
	dir := t.TempDir()
	return &DesiredStateManager{
		dir:    dir,
		states: make(map[string]*DesiredState),
	}
}

func TestDesiredStateAddForward(t *testing.T) {
	state := &DesiredState{Agents: make(map[string][]DesiredForward)}
	state.AddForward("claude-1", DesiredForward{ContainerPort: 8080, LocalPort: 3000})

	forwards := state.GetAgentForwards("claude-1")
	if len(forwards) != 1 {
		t.Fatalf("expected 1 forward, got %d", len(forwards))
	}
	if forwards[0].ContainerPort != 8080 {
		t.Errorf("ContainerPort = %d, want 8080", forwards[0].ContainerPort)
	}
	if forwards[0].LocalPort != 3000 {
		t.Errorf("LocalPort = %d, want 3000", forwards[0].LocalPort)
	}
}

func TestDesiredStateAddForwardDuplicate(t *testing.T) {
	state := &DesiredState{Agents: make(map[string][]DesiredForward)}
	state.AddForward("claude-1", DesiredForward{ContainerPort: 8080, LocalPort: 3000})
	state.AddForward("claude-1", DesiredForward{ContainerPort: 8080, LocalPort: 4000}) // duplicate port

	forwards := state.GetAgentForwards("claude-1")
	if len(forwards) != 1 {
		t.Errorf("duplicate forward should be ignored: got %d forwards", len(forwards))
	}
}

func TestDesiredStateAddForwardMultiple(t *testing.T) {
	state := &DesiredState{Agents: make(map[string][]DesiredForward)}
	state.AddForward("claude-1", DesiredForward{ContainerPort: 8080, LocalPort: 3000})
	state.AddForward("claude-1", DesiredForward{ContainerPort: 22, LocalPort: 3001})
	state.AddForward("claude-2", DesiredForward{ContainerPort: 8080, LocalPort: 3002})

	if len(state.GetAgentForwards("claude-1")) != 2 {
		t.Errorf("claude-1 should have 2 forwards")
	}
	if len(state.GetAgentForwards("claude-2")) != 1 {
		t.Errorf("claude-2 should have 1 forward")
	}
}

func TestDesiredStateRemoveForward(t *testing.T) {
	state := &DesiredState{Agents: make(map[string][]DesiredForward)}
	state.AddForward("claude-1", DesiredForward{ContainerPort: 8080, LocalPort: 3000})
	state.AddForward("claude-1", DesiredForward{ContainerPort: 22, LocalPort: 3001})

	state.RemoveForward("claude-1", 8080)

	forwards := state.GetAgentForwards("claude-1")
	if len(forwards) != 1 {
		t.Fatalf("expected 1 forward after remove, got %d", len(forwards))
	}
	if forwards[0].ContainerPort != 22 {
		t.Errorf("remaining forward ContainerPort = %d, want 22", forwards[0].ContainerPort)
	}
}

func TestDesiredStateRemoveForwardNonexistent(t *testing.T) {
	state := &DesiredState{Agents: make(map[string][]DesiredForward)}
	state.AddForward("claude-1", DesiredForward{ContainerPort: 8080, LocalPort: 3000})

	// Removing nonexistent port should not panic
	state.RemoveForward("claude-1", 9999)
	state.RemoveForward("nonexistent-agent", 8080)

	if len(state.GetAgentForwards("claude-1")) != 1 {
		t.Error("forward should still exist")
	}
}

func TestDesiredStateGetAgentForwardsCopy(t *testing.T) {
	state := &DesiredState{Agents: make(map[string][]DesiredForward)}
	state.AddForward("claude-1", DesiredForward{ContainerPort: 8080, LocalPort: 3000})

	forwards := state.GetAgentForwards("claude-1")
	forwards[0].ContainerPort = 9999 // modify the copy

	original := state.GetAgentForwards("claude-1")
	if original[0].ContainerPort != 8080 {
		t.Error("GetAgentForwards should return a copy, not a reference")
	}
}

func TestDesiredStateGetAgentForwardsEmpty(t *testing.T) {
	state := &DesiredState{Agents: make(map[string][]DesiredForward)}
	forwards := state.GetAgentForwards("nonexistent")
	if len(forwards) != 0 {
		t.Errorf("expected 0 forwards for nonexistent agent, got %d", len(forwards))
	}
}

func TestDesiredStateGetAllAgents(t *testing.T) {
	state := &DesiredState{Agents: make(map[string][]DesiredForward)}
	state.AddForward("claude-1", DesiredForward{ContainerPort: 8080, LocalPort: 3000})
	state.AddForward("claude-2", DesiredForward{ContainerPort: 22, LocalPort: 3001})

	agents := state.GetAllAgents()
	if len(agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(agents))
	}

	agentSet := map[string]bool{}
	for _, a := range agents {
		agentSet[a] = true
	}
	if !agentSet["claude-1"] || !agentSet["claude-2"] {
		t.Errorf("expected claude-1 and claude-2, got %v", agents)
	}
}

func TestDesiredStateGetAllAgentsEmpty(t *testing.T) {
	state := &DesiredState{Agents: make(map[string][]DesiredForward)}
	agents := state.GetAllAgents()
	if len(agents) != 0 {
		t.Errorf("expected 0 agents, got %d", len(agents))
	}
}

func TestDesiredStateConcurrent(t *testing.T) {
	state := &DesiredState{Agents: make(map[string][]DesiredForward)}
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			state.AddForward("claude-1", DesiredForward{ContainerPort: 8080 + n, LocalPort: 3000 + n})
		}(i)
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			state.GetAgentForwards("claude-1")
			state.GetAllAgents()
		}()
	}

	wg.Wait()
}

func TestManagerGetOrCreate(t *testing.T) {
	mgr := newTestDesiredStateManager(t)

	state := mgr.GetOrCreate("project-a")
	if state == nil {
		t.Fatal("GetOrCreate returned nil")
	}
	if state.Agents == nil {
		t.Fatal("Agents map should be initialized")
	}

	// Second call returns same instance
	state2 := mgr.GetOrCreate("project-a")
	if state != state2 {
		t.Error("GetOrCreate should return same instance for same vibespace")
	}
}

func TestManagerGet(t *testing.T) {
	mgr := newTestDesiredStateManager(t)

	if got := mgr.Get("nonexistent"); got != nil {
		t.Error("Get nonexistent should return nil")
	}

	mgr.GetOrCreate("project-a")
	if got := mgr.Get("project-a"); got == nil {
		t.Error("Get should return state after GetOrCreate")
	}
}

func TestManagerSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	mgr := &DesiredStateManager{
		dir:    dir,
		states: make(map[string]*DesiredState),
	}

	state := mgr.GetOrCreate("test-vs")
	state.AddForward("claude-1", DesiredForward{ContainerPort: 8080, LocalPort: 3000})

	if err := mgr.Save("test-vs"); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify file exists
	path := filepath.Join(dir, "test-vs.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var loaded DesiredState
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(loaded.Agents["claude-1"]) != 1 {
		t.Errorf("loaded state should have 1 forward for claude-1")
	}
}

func TestManagerSaveNonexistent(t *testing.T) {
	mgr := newTestDesiredStateManager(t)
	// Saving a nonexistent vibespace should be a no-op
	if err := mgr.Save("nonexistent"); err != nil {
		t.Errorf("Save nonexistent should return nil, got: %v", err)
	}
}

func TestManagerRemove(t *testing.T) {
	dir := t.TempDir()
	mgr := &DesiredStateManager{
		dir:    dir,
		states: make(map[string]*DesiredState),
	}

	state := mgr.GetOrCreate("test-vs")
	state.AddForward("claude-1", DesiredForward{ContainerPort: 8080, LocalPort: 3000})
	mgr.Save("test-vs")

	if err := mgr.Remove("test-vs"); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if mgr.Get("test-vs") != nil {
		t.Error("state should be nil after Remove")
	}

	// File should be gone
	path := filepath.Join(dir, "test-vs.json")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file should be deleted after Remove")
	}
}

func TestManagerRemoveNonexistent(t *testing.T) {
	mgr := newTestDesiredStateManager(t)
	if err := mgr.Remove("nonexistent"); err != nil {
		t.Errorf("Remove nonexistent should return nil, got: %v", err)
	}
}

func TestManagerLoadAll(t *testing.T) {
	dir := t.TempDir()

	// Write a state file manually
	state := &DesiredState{
		Agents: map[string][]DesiredForward{
			"claude-1": {{ContainerPort: 8080, LocalPort: 3000}},
		},
	}
	data, _ := json.Marshal(state)
	os.WriteFile(filepath.Join(dir, "my-vs.json"), data, 0644)

	mgr := &DesiredStateManager{
		dir:    dir,
		states: make(map[string]*DesiredState),
	}
	if err := mgr.loadAll(); err != nil {
		t.Fatalf("loadAll: %v", err)
	}

	loaded := mgr.Get("my-vs")
	if loaded == nil {
		t.Fatal("my-vs should be loaded")
	}
	forwards := loaded.GetAgentForwards("claude-1")
	if len(forwards) != 1 {
		t.Errorf("expected 1 forward, got %d", len(forwards))
	}
}

func TestManagerLoadAllSkipsCorrupt(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "good.json"), []byte(`{"agents":{}}`), 0644)
	os.WriteFile(filepath.Join(dir, "bad.json"), []byte(`{invalid`), 0644)
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not json"), 0644)

	mgr := &DesiredStateManager{
		dir:    dir,
		states: make(map[string]*DesiredState),
	}
	if err := mgr.loadAll(); err != nil {
		t.Fatalf("loadAll: %v", err)
	}

	if mgr.Get("good") == nil {
		t.Error("good.json should be loaded")
	}
	if mgr.Get("bad") != nil {
		t.Error("bad.json should be skipped")
	}
	if mgr.Get("readme") != nil {
		t.Error("readme.txt should be skipped (not .json)")
	}
}

func TestManagerLoadAllEmptyDir(t *testing.T) {
	mgr := &DesiredStateManager{
		dir:    filepath.Join(t.TempDir(), "nonexistent"),
		states: make(map[string]*DesiredState),
	}
	if err := mgr.loadAll(); err != nil {
		t.Fatalf("loadAll on nonexistent dir should return nil, got: %v", err)
	}
}
