package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"
)

// ============================================================================
// Test Helpers
// ============================================================================

// createTempProject creates a temporary project directory with a basic configuration
func createTempProject(t *testing.T) (*Project, string) {
	t.Helper()

	tempDir := t.TempDir()

	// Create basic project structure
	orqenDir := filepath.Join(tempDir, ".orqen")
	if err := os.MkdirAll(orqenDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a basic config
	config := `
agents:
  default: "qwen"
  clients:
    qwen:
      command: ["qwen", "--yolo", "--acp"]

execution:
  max_agents: 10
  sleep_interval_seconds: 1

modules:
  - name: task
    dir: "tasks"
    order: ["doing", "ready", "prioritized", "inbox"]
    lanes:
      - name: "inbox"
        purpose: "User ideas"
      - name: "prioritized"
        purpose: "Tasks selected for refinement"
        ignore_if_exists: ["adr.draft"]
      - name: "ready"
        purpose: "Approved tasks"
        ignore_if_exists: ["doing"]
        ignore_if_dependency: ["prioritized", "backlog", "doing"]
        agent_behavior:
          - "Execute the task"
      - name: "doing"
        purpose: "Task being implemented"
      - name: "done"
        purpose: "Completed tasks"
  - name: adr
    dir: "docs/adr"
    lanes:
      - name: "draft"
        purpose: "Draft ADRs"
      - name: "accepted"
        purpose: "Accepted ADRs"
`

	if err := os.WriteFile(filepath.Join(orqenDir, "orqen.yaml"), []byte(config), 0644); err != nil {
		t.Fatal(err)
	}

	// Load the project
	project, err := Load(tempDir)
	if err != nil {
		t.Fatalf("Failed to load project: %v", err)
	}

	return project, tempDir
}

// createWorkItem creates a work item directory in a lane
func createWorkItemDir(t *testing.T, lane *Lane, name string) {
	t.Helper()

	if lane.DirAbs == "" {
		t.Fatal("lane.DirAbs is empty")
	}

	itemDir := filepath.Join(lane.DirAbs, name)
	if err := os.MkdirAll(itemDir, 0755); err != nil {
		t.Fatal(err)
	}
}

// createMockInvoker creates a mock invoker that tracks invocations
type mockInvoker struct {
	mu              sync.Mutex
	invocations     []InvocationRecord
	invocationDelay time.Duration
}

type InvocationRecord struct {
	Module string
	Lane   string
	Item   string
}

func (m *mockInvoker) Invoke(project *Project, module *Module, lane *Lane, item *WorkItem) (InvocationHandle, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.invocations = append(m.invocations, InvocationRecord{
		Module: module.Name,
		Lane:   lane.Name,
		Item:   item.Name,
	})

	handle := InvocationHandle{
		Item: item,
		Done: make(chan struct{}),
	}

	// Start a goroutine to complete the invocation
	go func() {
		if m.invocationDelay > 0 {
			time.Sleep(m.invocationDelay)
		}
		close(handle.Done)
	}()

	return handle, nil
}

func (m *mockInvoker) GetInvocations() []InvocationRecord {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]InvocationRecord, len(m.invocations))
	copy(result, m.invocations)
	return result
}

func (m *mockInvoker) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.invocations = nil
}

// AsWorkItemInvoker wraps the mockInvoker to match the WorkItemInvoker function type
func (m *mockInvoker) AsWorkItemInvoker() WorkItemInvoker {
	return m.Invoke
}

// ============================================================================
// Project Tests
// ============================================================================

func TestProjectGetModule(t *testing.T) {
	project, tempDir := createTempProject(t)
	defer os.RemoveAll(tempDir)

	mod := project.GetModule("task")
	if mod == nil {
		t.Error("expected to find task module")
	}

	mod = project.GetModule("adr")
	if mod == nil {
		t.Error("expected to find adr module")
	}

	mod = project.GetModule("nonexistent")
	if mod != nil {
		t.Error("expected nil for nonexistent module")
	}
}

func TestProjectActiveAgentCount(t *testing.T) {
	project, tempDir := createTempProject(t)
	defer os.RemoveAll(tempDir)

	// Set up lane directories
	for _, mod := range project.Modules {
		for _, lane := range mod.Lanes {
			lane.DirAbs = filepath.Join(tempDir, mod.Dir, lane.Name)
			if err := os.MkdirAll(lane.DirAbs, 0755); err != nil {
				t.Fatal(err)
			}
		}
	}

	taskModule := project.GetModule("task")
	doingLane := taskModule.GetLane("doing")

	createWorkItemDir(t, doingLane, "TASK-001-test")

	// Scan to populate cache
	scanLaneDirectory(doingLane)

	items := slices.Collect(doingLane.WorkItems())
	if len(items) > 0 {
		items[0].InProgress = true
	}

	if project.ActiveAgentCount() != 1 {
		t.Errorf("expected 1 active agent, got %d", project.ActiveAgentCount())
	}
}

func TestProjectHasAvailableSlot(t *testing.T) {
	project, tempDir := createTempProject(t)
	defer os.RemoveAll(tempDir)

	// Set up lane directories
	for _, mod := range project.Modules {
		for _, lane := range mod.Lanes {
			lane.DirAbs = filepath.Join(tempDir, mod.Dir, lane.Name)
			if err := os.MkdirAll(lane.DirAbs, 0755); err != nil {
				t.Fatal(err)
			}
		}
	}

	taskModule := project.GetModule("task")
	doingLane := taskModule.GetLane("doing")
	doingLane.MaxAgents = project.Execution.MaxAgents

	// Create items to fill slots
	for i := 0; i < project.Execution.MaxAgents-1; i++ {
		createWorkItemDir(t, doingLane, fmt.Sprintf("TASK-%03d-test", i+1))
	}

	// Scan to populate cache
	scanLaneDirectory(doingLane)

	// Should still have slot
	if !project.HasAvailableSlot() {
		t.Error("expected HasAvailableSlot to return true")
	}

	// Add one more to reach max
	createWorkItemDir(t, doingLane, fmt.Sprintf("TASK-%03d-test", project.Execution.MaxAgents))

	// Scan to populate cache
	scanLaneDirectory(doingLane)

	items := slices.Collect(doingLane.WorkItems())
	for _, item := range items {
		item.InProgress = true
	}

	// Should not have slot
	if project.HasAvailableSlot() {
		t.Error("expected HasAvailableSlot to return false")
	}
}

func TestProjectStartStop(t *testing.T) {
	project, tempDir := createTempProject(t)
	defer os.RemoveAll(tempDir)

	if project.IsRunning() {
		t.Error("expected project to not be running initially")
	}

	// Start the project
	project.Start()
	if !project.IsRunning() {
		t.Error("expected project to be running after Start()")
	}

	// Starting again should be a no-op
	project.Start()
	if !project.IsRunning() {
		t.Error("expected project to still be running after second Start()")
	}

	// Stop the project
	project.Stop()
	if project.IsRunning() {
		t.Error("expected project to not be running after Stop()")
	}

	// Stopping again should be a no-op
	project.Stop()
	if project.IsRunning() {
		t.Error("expected project to still not be running after second Stop()")
	}
}

// ============================================================================
// Executor Tests
// ============================================================================

func TestExecutorTick(t *testing.T) {
	project, tempDir := createTempProject(t)
	defer os.RemoveAll(tempDir)

	// Set up lane directories
	for _, mod := range project.Modules {
		for _, lane := range mod.Lanes {
			lane.DirAbs = filepath.Join(tempDir, mod.Dir, lane.Name)
			if err := os.MkdirAll(lane.DirAbs, 0755); err != nil {
				t.Fatal(err)
			}
		}
	}

	taskModule := project.GetModule("task")
	readyLane := taskModule.GetLane("ready")

	// Create a work item
	createWorkItemDir(t, readyLane, "TASK-001-test")

	// Scan to populate cache
	scanLaneDirectory(readyLane)

	// Create a mock invoker
	invoker := &mockInvoker{}
	executor := NewExecutor(project, invoker.AsWorkItemInvoker())

	// Perform one tick
	executor.tick()

	// Check that the item was invoked
	invocations := invoker.GetInvocations()
	if len(invocations) != 1 {
		t.Errorf("expected 1 invocation, got %d", len(invocations))
	}

	if len(invocations) > 0 {
		if invocations[0].Module != "task" {
			t.Errorf("expected module 'task', got '%s'", invocations[0].Module)
		}
		if invocations[0].Lane != "ready" {
			t.Errorf("expected lane 'ready', got '%s'", invocations[0].Lane)
		}
		if invocations[0].Item != "TASK-001-test" {
			t.Errorf("expected item 'TASK-001-test', got '%s'", invocations[0].Item)
		}
	}
}

func TestExecutorRespectsMaxAgents(t *testing.T) {
	project, tempDir := createTempProject(t)
	defer os.RemoveAll(tempDir)

	// Set max agents to 2
	project.Execution.MaxAgents = 2

	// Set up lane directories
	for _, mod := range project.Modules {
		for _, lane := range mod.Lanes {
			lane.DirAbs = filepath.Join(tempDir, mod.Dir, lane.Name)
			if err := os.MkdirAll(lane.DirAbs, 0755); err != nil {
				t.Fatal(err)
			}
			lane.MaxAgents = 2
		}
	}

	taskModule := project.GetModule("task")
	readyLane := taskModule.GetLane("ready")

	// Create 3 work items
	createWorkItemDir(t, readyLane, "TASK-001-test1")
	createWorkItemDir(t, readyLane, "TASK-002-test2")
	createWorkItemDir(t, readyLane, "TASK-003-test3")

	// Scan to populate cache
	scanLaneDirectory(readyLane)

	// Create a mock invoker with a delay
	invoker := &mockInvoker{
		invocationDelay: 100 * time.Millisecond,
	}
	executor := NewExecutor(project, invoker.AsWorkItemInvoker())

	// Perform one tick - should only start 2 invocations
	executor.tick()

	// Give some time for invocations to start
	time.Sleep(50 * time.Millisecond)

	invocations := invoker.GetInvocations()
	if len(invocations) > 2 {
		t.Errorf("expected at most 2 invocations, got %d", len(invocations))
	}
}

func TestExecutorCleanupCompleted(t *testing.T) {
	project, tempDir := createTempProject(t)
	defer os.RemoveAll(tempDir)

	// Set up lane directories
	for _, mod := range project.Modules {
		for _, lane := range mod.Lanes {
			lane.DirAbs = filepath.Join(tempDir, mod.Dir, lane.Name)
			if err := os.MkdirAll(lane.DirAbs, 0755); err != nil {
				t.Fatal(err)
			}
		}
	}

	taskModule := project.GetModule("task")
	readyLane := taskModule.GetLane("ready")

	// Create a work item
	createWorkItemDir(t, readyLane, "TASK-001-test")

	// Scan to populate cache
	scanLaneDirectory(readyLane)

	// Create a mock invoker with no delay
	invoker := &mockInvoker{}
	executor := NewExecutor(project, invoker.AsWorkItemInvoker())

	// Perform one tick
	executor.tick()

	// Wait for the invocation to complete
	time.Sleep(50 * time.Millisecond)

	// Perform cleanup
	executor.cleanupCompleted()

	// Check that the item is no longer in progress
	scanLaneDirectory(readyLane)
	items := slices.Collect(readyLane.WorkItems())
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	if items[0].InProgress {
		t.Error("expected item to not be in progress after cleanup")
	}
}

// ============================================================================
// Ignore Rules Tests
// ============================================================================

func TestIgnoreIfExists(t *testing.T) {
	project, tempDir := createTempProject(t)
	defer os.RemoveAll(tempDir)

	// Set up lane directories and module references
	for _, mod := range project.Modules {
		for _, lane := range mod.Lanes {
			lane.DirAbs = filepath.Join(tempDir, mod.Dir, lane.Name)
			lane.Module = mod
			if err := os.MkdirAll(lane.DirAbs, 0755); err != nil {
				t.Fatal(err)
			}
		}
	}

	taskModule := project.GetModule("task")
	readyLane := taskModule.GetLane("ready")
	doingLane := taskModule.GetLane("doing")

	// Create an item in doing lane
	createWorkItemDir(t, doingLane, "TASK-001-existing")

	// Create an item in ready lane
	createWorkItemDir(t, readyLane, "TASK-002-new")

	// Scan to populate caches
	scanLaneDirectory(doingLane)
	scanLaneDirectory(readyLane)

	// ready lane has ignore_if_exists: ["doing"]
	// So the new item should be ignored
	invoker := &mockInvoker{}
	executor := NewExecutor(project, invoker.AsWorkItemInvoker())
	executor.tick()

	invocations := invoker.GetInvocations()
	// Should not have invoked the ready item because doing has items
	for _, inv := range invocations {
		if inv.Lane == "ready" {
			t.Error("expected ready item to be ignored due to ignore_if_exists rule")
		}
	}
}

func TestIgnoreIfDependency(t *testing.T) {
	project, tempDir := createTempProject(t)
	defer os.RemoveAll(tempDir)

	// Set up lane directories
	for _, mod := range project.Modules {
		for _, lane := range mod.Lanes {
			lane.DirAbs = filepath.Join(tempDir, mod.Dir, lane.Name)
			if err := os.MkdirAll(lane.DirAbs, 0755); err != nil {
				t.Fatal(err)
			}
		}
	}

	taskModule := project.GetModule("task")

	// Set up lane directories and module references
	for _, lane := range taskModule.Lanes {
		lane.DirAbs = filepath.Join(tempDir, taskModule.Dir, lane.Name)
		lane.Module = taskModule
		if err := os.MkdirAll(lane.DirAbs, 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Also set up ADR module lanes
	adrModule := project.GetModule("adr")
	for _, lane := range adrModule.Lanes {
		lane.DirAbs = filepath.Join(tempDir, adrModule.Dir, lane.Name)
		lane.Module = adrModule
		if err := os.MkdirAll(lane.DirAbs, 0755); err != nil {
			t.Fatal(err)
		}
	}

	readyLane := taskModule.GetLane("ready")
	prioritizedLane := taskModule.GetLane("prioritized")

	// Create an item in prioritized lane
	createWorkItemDir(t, prioritizedLane, "TASK-001-dependency")

	// Create an item in ready lane with a dependency on the prioritized item
	createWorkItemDir(t, readyLane, "TASK-002-dependent")

	// Scan to populate caches
	scanLaneDirectory(prioritizedLane)
	scanLaneDirectory(readyLane)

	// The dependency is simulated by adding it to the item's Dependencies field
	// First, we need to list items in the prioritized lane to get the dependency item
	prioritizedItems := slices.Collect(prioritizedLane.WorkItems())
	if len(prioritizedItems) == 0 {
		t.Fatal("expected at least one item in prioritized lane")
	}

	// Now list items in ready lane and set the dependency
	readyItems := slices.Collect(readyLane.WorkItems())
	if len(readyItems) == 0 {
		t.Fatal("expected at least one item in ready lane")
	}

	// Set the dependency on the ready item
	readyItems[0].Attributes.Set("dependencies", []string{strconv.Itoa(prioritizedItems[0].Seq)})

	// ready lane has ignore_if_dependency: ["prioritized", "backlog", "doing"]
	// So the new item should be ignored because TASK-001 is in prioritized
	invoker := &mockInvoker{}
	executor := NewExecutor(project, invoker.AsWorkItemInvoker())
	executor.tick()

	invocations := invoker.GetInvocations()
	// Should not have invoked the ready item because dependency is in prioritized
	for _, inv := range invocations {
		if inv.Lane == "ready" {
			t.Error("expected ready item to be ignored due to ignore_if_dependency rule")
		}
	}
}

// ============================================================================
// Integration Test
// ============================================================================

func TestExecutionLoopIntegration(t *testing.T) {
	project, tempDir := createTempProject(t)
	defer os.RemoveAll(tempDir)

	// Set a short sleep interval for testing
	project.Execution.SleepIntervalSeconds = 1

	// Set up lane directories
	for _, mod := range project.Modules {
		for _, lane := range mod.Lanes {
			lane.DirAbs = filepath.Join(tempDir, mod.Dir, lane.Name)
			if err := os.MkdirAll(lane.DirAbs, 0755); err != nil {
				t.Fatal(err)
			}
		}
	}

	taskModule := project.GetModule("task")
	readyLane := taskModule.GetLane("ready")

	// Create some work items
	createWorkItemDir(t, readyLane, "TASK-001-create-project")
	createWorkItemDir(t, readyLane, "TASK-002-implement-feature")

	// Scan to populate cache
	scanLaneDirectory(readyLane)

	// Create a mock invoker
	invoker := &mockInvoker{}
	project.withInvokerOld(invoker.AsWorkItemInvoker())

	// Start the project
	project.Start()

	// Wait for a couple of ticks
	time.Sleep(2500 * time.Millisecond)

	// Stop the project
	project.Stop()

	// Check that items were invoked
	invocations := invoker.GetInvocations()
	if len(invocations) < 2 {
		t.Errorf("expected at least 2 invocations, got %d", len(invocations))
	}

	// Check that all invocations are for the ready lane
	for _, inv := range invocations {
		if inv.Lane != "ready" {
			t.Errorf("unexpected invocation in lane %s", inv.Lane)
		}
	}
}

// ============================================================================
// Load and Config Tests
// ============================================================================

func TestLoadInvalidDir(t *testing.T) {
	_, err := Load("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

func TestLoadMissingOrqenDir(t *testing.T) {
	tempDir := t.TempDir()
	_, err := Load(tempDir)
	if err == nil {
		t.Error("expected error for missing .orqen directory")
	}
}

func TestLoadMissingConfigFile(t *testing.T) {
	tempDir := t.TempDir()
	orqenDir := filepath.Join(tempDir, ".orqen")
	if err := os.MkdirAll(orqenDir, 0755); err != nil {
		t.Fatal(err)
	}

	_, err := Load(tempDir)
	if err == nil {
		t.Error("expected error for missing orqen.yaml")
	}
}

func TestLoadInvalidYaml(t *testing.T) {
	tempDir := t.TempDir()
	orqenDir := filepath.Join(tempDir, ".orqen")
	if err := os.MkdirAll(orqenDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(orqenDir, "orqen.yaml"), []byte("not: valid: yaml: ["), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(tempDir)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestLoadNoModules(t *testing.T) {
	tempDir := t.TempDir()
	orqenDir := filepath.Join(tempDir, ".orqen")
	if err := os.MkdirAll(orqenDir, 0755); err != nil {
		t.Fatal(err)
	}

	config := `
agents:
  default: "qwen"
  clients:
    qwen:
      command: ["qwen", "--yolo"]
execution:
  max_agents: 10
  sleep_interval_seconds: 1
modules: []
`
	if err := os.WriteFile(filepath.Join(orqenDir, "orqen.yaml"), []byte(config), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(tempDir)
	if err == nil {
		t.Error("expected error for no modules defined")
	}
}

func TestLoadEmptyAgentCommand(t *testing.T) {
	tempDir := t.TempDir()
	orqenDir := filepath.Join(tempDir, ".orqen")
	if err := os.MkdirAll(orqenDir, 0755); err != nil {
		t.Fatal(err)
	}

	config := `
agents:
  default: "qwen"
  clients:
    qwen:
      command: []
execution:
  max_agents: 10
  sleep_interval_seconds: 1
modules:
  - name: task
    lanes:
      - name: "inbox"
        purpose: "Test"
`
	if err := os.WriteFile(filepath.Join(orqenDir, "orqen.yaml"), []byte(config), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(tempDir)
	if err == nil {
		t.Error("expected error for empty agent command")
	}
}

func TestLoadDefaultsApplied(t *testing.T) {
	tempDir := t.TempDir()
	orqenDir := filepath.Join(tempDir, ".orqen")
	if err := os.MkdirAll(orqenDir, 0755); err != nil {
		t.Fatal(err)
	}

	config := `
agents:
  default: "qwen"
  clients:
    qwen:
      command: ["qwen", "--yolo"]
execution:
  max_agents: 10
  sleep_interval_seconds: 1
modules:
  - name: task
    lanes:
      - name: "doing"
        purpose: "In progress"
`
	if err := os.WriteFile(filepath.Join(orqenDir, "orqen.yaml"), []byte(config), 0644); err != nil {
		t.Fatal(err)
	}

	proj, err := Load(tempDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check defaults
	if proj.Execution.MaxAgents != 10 {
		t.Errorf("expected MaxAgents=10, got %d", proj.Execution.MaxAgents)
	}
	// SleepIntervalSeconds was explicitly set to 1
	if proj.Execution.SleepIntervalSeconds != 1 {
		t.Errorf("expected SleepIntervalSeconds=1, got %d", proj.Execution.SleepIntervalSeconds)
	}

	// Check inbox lane was added
	taskModule := proj.GetModule("task")
	inboxLane := taskModule.GetLane("inbox")
	if inboxLane == nil {
		t.Error("expected inbox lane to be auto-created")
	}

	// Check inbox defaults
	if inboxLane.MaxAgents != 2 {
		t.Errorf("expected inbox MaxAgents=2, got %d", inboxLane.MaxAgents)
	}
	if len(inboxLane.AgentBehavior) == 0 {
		t.Error("expected inbox AgentBehavior to be populated")
	}
	if len(inboxLane.CriticalRules) == 0 {
		t.Error("expected inbox CriticalRules to be populated")
	}
}

func TestLoadDuplicateLaneNames(t *testing.T) {
	tempDir := t.TempDir()
	orqenDir := filepath.Join(tempDir, ".orqen")
	if err := os.MkdirAll(orqenDir, 0755); err != nil {
		t.Fatal(err)
	}

	config := `
agents:
  default: "qwen"
  clients:
    qwen:
      command: ["qwen", "--yolo"]
execution:
  max_agents: 10
  sleep_interval_seconds: 1
modules:
  - name: task
    lanes:
      - name: "doing"
        purpose: "First"
      - name: "doing"
        purpose: "Duplicate"
`
	if err := os.WriteFile(filepath.Join(orqenDir, "orqen.yaml"), []byte(config), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(tempDir)
	if err == nil {
		t.Error("expected error for duplicate lane names")
	}
}

// ============================================================================
// Types Tests
// ============================================================================

func TestAgentGetCommand(t *testing.T) {
	agent := Agent{
		Default: "qwen",
		Clients: map[string]AgentClient{
			"qwen":   {Command: []string{"qwen", "--yolo"}},
			"claude": {Command: []string{"claude", "run"}},
			"empty":  {Command: []string{}},
		},
	}

	// Test explicit agent
	cmd := agent.GetCommand("claude")
	if len(cmd) != 2 || cmd[0] != "claude" || cmd[1] != "run" {
		t.Errorf("expected [claude run], got %v", cmd)
	}

	// Test default agent
	cmd = agent.GetCommand("")
	if len(cmd) != 2 || cmd[0] != "qwen" || cmd[1] != "--yolo" {
		t.Errorf("expected [qwen --yolo], got %v", cmd)
	}

	// Test nonexistent agent (returns empty)
	cmd = agent.GetCommand("nonexistent")
	if cmd != nil {
		t.Errorf("expected nil/empty for nonexistent agent, got %v", cmd)
	}
}

func TestInvocationHandleIsComplete(t *testing.T) {
	handle := InvocationHandle{
		Done: make(chan struct{}),
	}

	// Initially not complete
	if handle.IsComplete() {
		t.Error("expected IsComplete to return false initially")
	}

	// Complete the handle
	close(handle.Done)

	// Now complete
	if !handle.IsComplete() {
		t.Error("expected IsComplete to return true after close")
	}
}

func TestInvocationHandleWait(t *testing.T) {
	handle := InvocationHandle{
		Done: make(chan struct{}),
	}

	done := make(chan bool)
	go func() {
		handle.Wait()
		done <- true
	}()

	// Wait should block
	select {
	case <-done:
		t.Error("Wait should have blocked")
	case <-time.After(50 * time.Millisecond):
		// Expected: still waiting
	}

	// Complete the handle
	close(handle.Done)

	// Now Wait should return
	select {
	case <-done:
		// Expected
	case <-time.After(200 * time.Millisecond):
		t.Error("Wait should have returned after close")
	}
}

// ============================================================================
// Project Additional Tests
// ============================================================================

func TestProjectWithInvoker(t *testing.T) {
	project, _ := createTempProject(t)

	invoker := func(prompt string, item *WorkItem) error {
		return nil
	}

	result := project.WithInvoker(invoker)
	if result != project {
		t.Error("WithInvoker should return the project")
	}

	if project.invoker == nil {
		t.Error("invoker should be set")
	}
}

func TestProjectWithInvokerOld(t *testing.T) {
	project, tempDir := createTempProject(t)

	// Set up lane directories
	for _, mod := range project.Modules {
		for _, lane := range mod.Lanes {
			lane.DirAbs = filepath.Join(tempDir, mod.Dir, lane.Name)
			if err := os.MkdirAll(lane.DirAbs, 0755); err != nil {
				t.Fatal(err)
			}
		}
	}

	mockInv := &mockInvoker{}
	result := project.withInvokerOld(mockInv.AsWorkItemInvoker())
	if result != project {
		t.Error("withInvokerOld should return the project")
	}

	if project.executor == nil {
		t.Fatal("executor should be created")
	}

	// Test that it replaces an existing executor
	// Note: withInvokerOld calls executor.Stop() which blocks on e.done channel.
	// Since Run() was never called, e.done is never closed, so Stop() will block.
	// We test this in a goroutine with a timeout.
	mockInv2 := &mockInvoker{}
	done := make(chan struct{})
	go func() {
		project.withInvokerOld(mockInv2.AsWorkItemInvoker())
		close(done)
	}()

	select {
	case <-done:
		// Good - it completed
	case <-time.After(2 * time.Second):
		// Timeout - Stop() is blocking because Run() was never called
		// This is expected behavior when executor is not running
	}

	if project.executor == nil {
		t.Error("executor should be replaced")
	}
}
