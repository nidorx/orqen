package engine

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// concurrencyTracker provides thread-safe tracking of peak concurrent invocations.
type concurrencyTracker struct {
	current atomic.Int32
	max     atomic.Int32
}

func (t *concurrencyTracker) Start() {
	n := t.current.Add(1)
	for {
		old := t.max.Load()
		if n <= old {
			break
		}
		if t.max.CompareAndSwap(old, n) {
			break
		}
	}
}

func (t *concurrencyTracker) End() {
	t.current.Add(-1)
}

func (t *concurrencyTracker) Peak() int32 {
	return t.max.Load()
}

func (t *concurrencyTracker) Current() int32 {
	return t.current.Load()
}

// makeTrackingInvoker creates a WorkItemInvoker that tracks concurrency and simulates work.
func makeTrackingInvoker(tracker *concurrencyTracker, workDuration time.Duration) WorkItemInvoker {
	return func(project *Project, mod *Module, lane *Lane, item *WorkItem) (InvocationHandle, error) {
		handle := InvocationHandle{Item: item, Done: make(chan struct{})}
		tracker.Start()
		go func() {
			time.Sleep(workDuration)
			tracker.End()
			close(handle.Done)
		}()
		return handle, nil
	}
}

// setupLaneDir creates a lane's absolute directory path.
func setupLaneDir(t *testing.T, lane *Lane, tempDir, modDir, laneName string) {
	t.Helper()
	lane.DirAbs = filepath.Join(tempDir, modDir, laneName)
	if err := os.MkdirAll(lane.DirAbs, 0o755); err != nil {
		t.Fatal(err)
	}
}

// waitExecutorWaitGroup waits for the executor's wait group to drain.
func waitExecutorWaitGroup(t *testing.T, executor *Executor) {
	t.Helper()
	executor.wg.Wait()
}

// ============================================================================
// Concurrency Integration Tests
// ============================================================================

// TestLaneMaxAgentsOne_SerialExecution verifies that a lane with max_agents: 1
// executes only one item at a time.
// INVARIANT: at no point should concurrent invocations exceed lane max_agents.
func TestLaneMaxAgentsOne_SerialExecution(t *testing.T) {
	proj, tempDir := createTempProject(t)
	defer os.RemoveAll(tempDir)

	lane := proj.Modules[0].GetLane("inbox")
	lane.MaxAgents = 1
	lane.AgentBehavior = []string{"do something"}
	setupLaneDir(t, lane, tempDir, proj.Modules[0].Dir, "inbox")

	createWorkItemDir(t, lane, "TASK-001-test-item-one")
	createWorkItemDir(t, lane, "TASK-002-test-item-two")
	createWorkItemDir(t, lane, "TASK-003-test-item-three")

	scanLaneDirectory(lane)

	tracker := &concurrencyTracker{}
	invoker := makeTrackingInvoker(tracker, 100*time.Millisecond)

	executor := NewExecutor(proj, invoker)
	executor.tick()
	waitExecutorWaitGroup(t, executor)

	// Verify peak concurrent was exactly 1
	if peak := tracker.Peak(); peak != 1 {
		t.Errorf("expected peak concurrent = 1, got %d", peak)
	}
}

// TestLaneMaxAgentsN_EnforcesLimit verifies that a lane with max_agents: N
// executes at most N items concurrently.
// INVARIANT: peak concurrent invocations must never exceed lane max_agents.
func TestLaneMaxAgentsN_EnforcesLimit(t *testing.T) {
	proj, tempDir := createTempProject(t)
	defer os.RemoveAll(tempDir)

	lane := proj.Modules[0].GetLane("inbox")
	lane.MaxAgents = 2
	lane.AgentBehavior = []string{"do something"}
	setupLaneDir(t, lane, tempDir, proj.Modules[0].Dir, "inbox")

	// Create 5 work items with unique names
	createWorkItemDir(t, lane, "TASK-001-test-item-one")
	createWorkItemDir(t, lane, "TASK-002-test-item-two")
	createWorkItemDir(t, lane, "TASK-003-test-item-three")
	createWorkItemDir(t, lane, "TASK-004-test-item-four")
	createWorkItemDir(t, lane, "TASK-005-test-item-five")

	scanLaneDirectory(lane)

	tracker := &concurrencyTracker{}
	invoker := makeTrackingInvoker(tracker, 100*time.Millisecond)

	executor := NewExecutor(proj, invoker)
	executor.tick()
	waitExecutorWaitGroup(t, executor)

	// With max_agents=2, first tick starts 2 items, peak should be 2
	if peak := tracker.Peak(); peak != 2 {
		t.Errorf("expected peak concurrent = 2, got %d", peak)
	}
}

// TestProjectMaxAgents_LimitsAcrossLanes verifies that project-level max_agents
// limits execution across multiple lanes, even when each lane allows more.
// INVARIANT: total concurrent invocations across all lanes must not exceed project max_agents.
func TestProjectMaxAgents_LimitsAcrossLanes(t *testing.T) {
	proj, tempDir := createTempProject(t)
	defer os.RemoveAll(tempDir)

	// Set project-level limit to 2
	proj.Execution.MaxAgents = 2

	// Configure two lanes, each allowing up to 5 agents
	lane1 := proj.Modules[0].GetLane("inbox")
	lane1.MaxAgents = 5
	lane1.AgentBehavior = []string{"do something"}
	setupLaneDir(t, lane1, tempDir, proj.Modules[0].Dir, "inbox")

	lane2 := proj.Modules[0].GetLane("ready")
	lane2.MaxAgents = 5
	lane2.AgentBehavior = []string{"do something"}
	setupLaneDir(t, lane2, tempDir, proj.Modules[0].Dir, "ready")

	// 3 items in each lane (6 total) — names must be unique across ALL lanes
	// because hash is computed from seq+name only
	createWorkItemDir(t, lane1, "TASK-001-laneA-item-one")
	createWorkItemDir(t, lane1, "TASK-002-laneA-item-two")
	createWorkItemDir(t, lane1, "TASK-003-laneA-item-three")
	createWorkItemDir(t, lane2, "TASK-004-laneB-item-one")
	createWorkItemDir(t, lane2, "TASK-005-laneB-item-two")
	createWorkItemDir(t, lane2, "TASK-006-laneB-item-three")

	scanLaneDirectory(lane1)
	scanLaneDirectory(lane2)

	tracker := &concurrencyTracker{}
	invoker := makeTrackingInvoker(tracker, 100*time.Millisecond)

	executor := NewExecutor(proj, invoker)
	executor.tick()
	waitExecutorWaitGroup(t, executor)

	// Project max_agents=2 should limit total to 2, even though lanes allow 5
	if peak := tracker.Peak(); peak != 2 {
		t.Errorf("expected peak concurrent = 2 (project limit), got %d", peak)
	}
}

// TestLaneAndProjectLimit_Interaction verifies that both lane-level and
// project-level limits are enforced simultaneously.
// INVARIANT: lane1 never exceeds 1 concurrent, lane2 never exceeds 3 concurrent,
// and total never exceeds 5 (project limit).
func TestLaneAndProjectLimit_Interaction(t *testing.T) {
	proj, tempDir := createTempProject(t)
	defer os.RemoveAll(tempDir)

	// Set project-level limit to 5
	proj.Execution.MaxAgents = 5

	// Lane1 allows only 1 concurrent
	lane1 := proj.Modules[0].GetLane("inbox")
	lane1.MaxAgents = 1
	lane1.AgentBehavior = []string{"do something"}
	setupLaneDir(t, lane1, tempDir, proj.Modules[0].Dir, "inbox")

	// Lane2 allows up to 3 concurrent
	lane2 := proj.Modules[0].GetLane("ready")
	lane2.MaxAgents = 3
	lane2.AgentBehavior = []string{"do something"}
	setupLaneDir(t, lane2, tempDir, proj.Modules[0].Dir, "ready")

	// 3 items in each lane — names must be unique across ALL lanes
	// because hash is computed from seq+name only
	createWorkItemDir(t, lane1, "TASK-001-laneA-item-one")
	createWorkItemDir(t, lane1, "TASK-002-laneA-item-two")
	createWorkItemDir(t, lane1, "TASK-003-laneA-item-three")
	createWorkItemDir(t, lane2, "TASK-004-laneB-item-one")
	createWorkItemDir(t, lane2, "TASK-005-laneB-item-two")
	createWorkItemDir(t, lane2, "TASK-006-laneB-item-three")

	scanLaneDirectory(lane1)
	scanLaneDirectory(lane2)

	tracker := &concurrencyTracker{}
	invoker := makeTrackingInvoker(tracker, 100*time.Millisecond)

	executor := NewExecutor(proj, invoker)
	executor.tick()
	waitExecutorWaitGroup(t, executor)

	// lane1 max_agents=1 should limit to 1, lane2 max_agents=3 should limit to 3
	// Project allows 5, so total = lane1(1) + lane2(3) = 4
	if peak := tracker.Peak(); peak != 4 {
		t.Errorf("expected peak concurrent = 4 (lane1=1 + lane2=3), got %d", peak)
	}
}

// TestItemsQueueWhenSlotsExhausted verifies that when slots are exhausted,
// items are queued (not dropped) and processed over multiple ticks.
// INVARIANT: with max_agents=1, only one item starts per tick; items remain
// in the lane waiting for their turn.
func TestItemsQueueWhenSlotsExhausted(t *testing.T) {
	proj, tempDir := createTempProject(t)
	defer os.RemoveAll(tempDir)

	lane := proj.Modules[0].GetLane("inbox")
	lane.MaxAgents = 1
	lane.AgentBehavior = []string{"do something"}
	setupLaneDir(t, lane, tempDir, proj.Modules[0].Dir, "inbox")

	// Create 3 items
	createWorkItemDir(t, lane, "TASK-001-first-item")
	createWorkItemDir(t, lane, "TASK-002-second-item")
	createWorkItemDir(t, lane, "TASK-003-third-item")

	scanLaneDirectory(lane)

	var mu sync.Mutex
	var invokedOrder []string
	var completeSignal = make(chan struct{}, 3)

	invoker := func(project *Project, mod *Module, lane *Lane, item *WorkItem) (InvocationHandle, error) {
		mu.Lock()
		invokedOrder = append(invokedOrder, item.Name)
		mu.Unlock()

		handle := InvocationHandle{Item: item, Done: make(chan struct{})}
		go func() {
			<-completeSignal // wait for test to signal completion
			close(handle.Done)
		}()
		return handle, nil
	}

	executor := NewExecutor(proj, invoker)

	// First tick: only 1 item should start (max_agents=1)
	executor.tick()
	time.Sleep(20 * time.Millisecond)

	mu.Lock()
	count1 := len(invokedOrder)
	mu.Unlock()
	if count1 != 1 {
		t.Fatalf("expected 1 invocation after first tick, got %d", count1)
	}

	// Verify that 2 items are still waiting (InProgress=false)
	waitingCount := 0
	for item := range lane.WorkItems() {
		if !item.InProgress {
			waitingCount++
		}
	}
	if waitingCount != 2 {
		t.Errorf("expected 2 items waiting (InProgress=false), got %d", waitingCount)
	}

	// Signal completion
	completeSignal <- struct{}{}
	executor.wg.Wait()

	// After completion, the item's InProgress should be false (cleanup)
	// and the next item should be eligible
	executor.tick()
	time.Sleep(20 * time.Millisecond)

	mu.Lock()
	count2 := len(invokedOrder)
	mu.Unlock()
	// May be 1 or 2 depending on whether the same item was re-invoked
	// The key invariant is: at most 1 new invocation per tick with max_agents=1
	if count2 > count1+1 {
		t.Errorf("expected at most %d invocations after second tick, got %d", count1+1, count2)
	}

	// Signal remaining completions
	for i := 0; i < 3; i++ {
		completeSignal <- struct{}{}
	}
	executor.wg.Wait()
}

// TestConcurrencyTests_Determinism verifies that concurrency tests produce
// consistent results across multiple runs.
// INVARIANT: running the same test setup 10 times should always yield peak concurrent = 1.
func TestConcurrencyTests_Determinism(t *testing.T) {
	for i := 0; i < 10; i++ {
		t.Run("run", func(t *testing.T) {
			proj, tempDir := createTempProject(t)
			defer os.RemoveAll(tempDir)

			lane := proj.Modules[0].GetLane("inbox")
			lane.MaxAgents = 1
			lane.AgentBehavior = []string{"do something"}
			setupLaneDir(t, lane, tempDir, proj.Modules[0].Dir, "inbox")

			createWorkItemDir(t, lane, "TASK-001-test-item-one")
			createWorkItemDir(t, lane, "TASK-002-test-item-two")
			createWorkItemDir(t, lane, "TASK-003-test-item-three")

			scanLaneDirectory(lane)

			tracker := &concurrencyTracker{}
			invoker := makeTrackingInvoker(tracker, 100*time.Millisecond)

			executor := NewExecutor(proj, invoker)
			executor.tick()
			waitExecutorWaitGroup(t, executor)

			if peak := tracker.Peak(); peak != 1 {
				t.Errorf("run %d: expected peak concurrent = 1, got %d", i+1, peak)
			}
		})
	}
}

// TestProjectMaxAgents_LimitsSingleLane verifies that project-level max_agents
// limits execution even within a single lane.
// INVARIANT: peak concurrent invocations must not exceed project max_agents.
func TestProjectMaxAgents_LimitsSingleLane(t *testing.T) {
	proj, tempDir := createTempProject(t)
	defer os.RemoveAll(tempDir)

	// Set project-level limit to 3
	proj.Execution.MaxAgents = 3

	// Lane allows unlimited agents
	lane := proj.Modules[0].GetLane("inbox")
	lane.MaxAgents = 0
	lane.AgentBehavior = []string{"do something"}
	setupLaneDir(t, lane, tempDir, proj.Modules[0].Dir, "inbox")

	// Create 6 items with unique names (more than project limit)
	createWorkItemDir(t, lane, "TASK-001-test-item-one")
	createWorkItemDir(t, lane, "TASK-002-test-item-two")
	createWorkItemDir(t, lane, "TASK-003-test-item-three")
	createWorkItemDir(t, lane, "TASK-004-test-item-four")
	createWorkItemDir(t, lane, "TASK-005-test-item-five")
	createWorkItemDir(t, lane, "TASK-006-test-item-six")

	scanLaneDirectory(lane)

	tracker := &concurrencyTracker{}
	invoker := makeTrackingInvoker(tracker, 100*time.Millisecond)

	executor := NewExecutor(proj, invoker)
	executor.tick()
	waitExecutorWaitGroup(t, executor)

	// Project max_agents=3 should limit to 3, even though lane allows unlimited
	if peak := tracker.Peak(); peak != 3 {
		t.Errorf("expected peak concurrent = 3 (project limit), got %d", peak)
	}
}

// TestMultiLane_FIFOPerLane verifies that within each lane, items are processed
// respecting per-lane concurrency limits, and that multi-lane execution respects
// both lane limits simultaneously.
func TestMultiLane_FIFOPerLane(t *testing.T) {
	proj, tempDir := createTempProject(t)
	defer os.RemoveAll(tempDir)

	// Allow enough project-level slots for both lanes
	proj.Execution.MaxAgents = 10

	// Lane1 allows 2 concurrent
	lane1 := proj.Modules[0].GetLane("inbox")
	lane1.MaxAgents = 2
	lane1.AgentBehavior = []string{"do something"}
	setupLaneDir(t, lane1, tempDir, proj.Modules[0].Dir, "inbox")

	// Lane2 allows 2 concurrent
	lane2 := proj.Modules[0].GetLane("ready")
	lane2.MaxAgents = 2
	lane2.AgentBehavior = []string{"do something"}
	setupLaneDir(t, lane2, tempDir, proj.Modules[0].Dir, "ready")

	// 3 items in each lane with unique names — names must be unique across ALL lanes
	// because hash is computed from seq+name only
	createWorkItemDir(t, lane1, "TASK-001-laneA-item-one")
	createWorkItemDir(t, lane1, "TASK-002-laneA-item-two")
	createWorkItemDir(t, lane1, "TASK-003-laneA-item-three")
	createWorkItemDir(t, lane2, "TASK-004-laneB-item-one")
	createWorkItemDir(t, lane2, "TASK-005-laneB-item-two")
	createWorkItemDir(t, lane2, "TASK-006-laneB-item-three")

	scanLaneDirectory(lane1)
	scanLaneDirectory(lane2)

	tracker := &concurrencyTracker{}
	invoker := makeTrackingInvoker(tracker, 100*time.Millisecond)

	executor := NewExecutor(proj, invoker)
	executor.tick()
	waitExecutorWaitGroup(t, executor)

	// Each lane allows 2, project allows 10, so first tick should start 2 per lane = 4 total
	if peak := tracker.Peak(); peak != 4 {
		t.Errorf("expected peak concurrent = 4 (lane1=2 + lane2=2), got %d", peak)
	}
}

// TestNoAgentBehavior_SkipsLane verifies that lanes without agent_behavior
// are skipped during execution.
func TestNoAgentBehavior_SkipsLane(t *testing.T) {
	proj, tempDir := createTempProject(t)
	defer os.RemoveAll(tempDir)

	// Lane with no agent behavior
	lane := proj.Modules[0].GetLane("inbox")
	lane.MaxAgents = 5
	lane.AgentBehavior = nil // no agent behavior
	setupLaneDir(t, lane, tempDir, proj.Modules[0].Dir, "inbox")

	createWorkItemDir(t, lane, "TASK-001-test-item")

	scanLaneDirectory(lane)

	tracker := &concurrencyTracker{}
	invoker := makeTrackingInvoker(tracker, 100*time.Millisecond)

	executor := NewExecutor(proj, invoker)
	executor.tick()
	waitExecutorWaitGroup(t, executor)

	// Should have invoked nothing because lane has no agent behavior
	if peak := tracker.Peak(); peak != 0 {
		t.Errorf("expected peak concurrent = 0 (no agent behavior), got %d", peak)
	}
}