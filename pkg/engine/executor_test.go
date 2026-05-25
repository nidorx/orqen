package engine

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestLaneConcurrency_MaxAgentsOne verifies that when a lane has max_agents: 1,
// only ONE work item executes at a time.
func TestLaneConcurrency_MaxAgentsOne(t *testing.T) {
	proj, tempDir := createTempProject(t)
	defer os.RemoveAll(tempDir)

	// Configure lane for test
	lane := proj.Modules[0].GetLane("inbox")
	lane.MaxAgents = 1
	lane.AgentBehavior = []string{"do something"}
	lane.DirAbs = filepath.Join(tempDir, proj.Modules[0].Dir, "inbox")
	if err := os.MkdirAll(lane.DirAbs, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create two work item directories
	createWorkItemDir(t, lane, "TASK-001-test-item-one")
	createWorkItemDir(t, lane, "TASK-002-test-item-two")

	// Scan to populate cache
	scanLaneDirectory(lane)

	var maxConcurrent atomic.Int32
	var currentConcurrent atomic.Int32
	var wg sync.WaitGroup

	// Mock invoker that tracks concurrency
	invoker := func(project *Project, mod *Module, lane *Lane, item *WorkItem) (InvocationHandle, error) {
		handle := InvocationHandle{
			Item: item,
			Done: make(chan struct{}),
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			n := currentConcurrent.Add(1)
			for {
				old := maxConcurrent.Load()
				if n <= old {
					break
				}
				if maxConcurrent.CompareAndSwap(old, n) {
					break
				}
			}
			time.Sleep(100 * time.Millisecond)
			currentConcurrent.Add(-1)
			close(handle.Done)
		}()

		return handle, nil
	}

	executor := NewExecutor(proj, invoker)
	executor.tick()

	wg.Wait()

	// Verify max concurrent was 1
	if max := maxConcurrent.Load(); max != 1 {
		t.Errorf("expected max concurrent = 1, got %d", max)
	}
}

// TestLaneConcurrency_MaxAgentsMultiple verifies that lanes with max_agents > 1
// execute multiple items up to the limit.
func TestLaneConcurrency_MaxAgentsMultiple(t *testing.T) {
	proj, tempDir := createTempProject(t)
	defer os.RemoveAll(tempDir)

	lane := proj.Modules[0].GetLane("inbox")
	lane.MaxAgents = 2
	lane.AgentBehavior = []string{"do something"}
	lane.DirAbs = filepath.Join(tempDir, proj.Modules[0].Dir, "inbox")
	if err := os.MkdirAll(lane.DirAbs, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create three work item directories
	createWorkItemDir(t, lane, "TASK-001-test-item-one")
	createWorkItemDir(t, lane, "TASK-002-test-item-two")
	createWorkItemDir(t, lane, "TASK-003-test-item-three")

	// Scan to populate cache
	scanLaneDirectory(lane)

	var maxConcurrent atomic.Int32
	var currentConcurrent atomic.Int32
	var wg sync.WaitGroup

	invoker := func(project *Project, mod *Module, lane *Lane, item *WorkItem) (InvocationHandle, error) {
		handle := InvocationHandle{
			Item: item,
			Done: make(chan struct{}),
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			n := currentConcurrent.Add(1)
			for {
				old := maxConcurrent.Load()
				if n <= old {
					break
				}
				if maxConcurrent.CompareAndSwap(old, n) {
					break
				}
			}
			time.Sleep(100 * time.Millisecond)
			currentConcurrent.Add(-1)
			close(handle.Done)
		}()

		return handle, nil
	}

	executor := NewExecutor(proj, invoker)
	executor.tick()

	wg.Wait()

	// With max_agents=2, we should see at most 2 concurrent
	if max := maxConcurrent.Load(); max != 2 {
		t.Errorf("expected max concurrent = 2, got %d", max)
	}
}

// TestLaneConcurrency_UnlimitedAgents verifies that when max_agents is 0 or unset,
// all eligible items execute without limit.
func TestLaneConcurrency_UnlimitedAgents(t *testing.T) {
	proj, tempDir := createTempProject(t)
	defer os.RemoveAll(tempDir)

	lane := proj.Modules[0].GetLane("inbox")
	lane.MaxAgents = 0 // unlimited
	lane.AgentBehavior = []string{"do something"}
	lane.DirAbs = filepath.Join(tempDir, proj.Modules[0].Dir, "inbox")
	if err := os.MkdirAll(lane.DirAbs, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create three work item directories
	createWorkItemDir(t, lane, "TASK-001-test-item-one")
	createWorkItemDir(t, lane, "TASK-002-test-item-two")
	createWorkItemDir(t, lane, "TASK-003-test-item-three")

	// Scan to populate cache
	scanLaneDirectory(lane)

	var maxConcurrent atomic.Int32
	var currentConcurrent atomic.Int32
	var wg sync.WaitGroup

	invoker := func(project *Project, mod *Module, lane *Lane, item *WorkItem) (InvocationHandle, error) {
		handle := InvocationHandle{
			Item: item,
			Done: make(chan struct{}),
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			n := currentConcurrent.Add(1)
			for {
				old := maxConcurrent.Load()
				if n <= old {
					break
				}
				if maxConcurrent.CompareAndSwap(old, n) {
					break
				}
			}
			time.Sleep(100 * time.Millisecond)
			currentConcurrent.Add(-1)
			close(handle.Done)
		}()

		return handle, nil
	}

	executor := NewExecutor(proj, invoker)
	executor.tick()

	wg.Wait()

	// With unlimited agents, all 3 should run concurrently
	if max := maxConcurrent.Load(); max != 3 {
		t.Errorf("expected max concurrent = 3, got %d", max)
	}
}

// TestProjectLevelSlotLimit verifies that project-level max_agents limit is respected.
func TestProjectLevelSlotLimit(t *testing.T) {
	proj, tempDir := createTempProject(t)
	defer os.RemoveAll(tempDir)

	// Use two lanes with unlimited lane-level agents
	lane1 := proj.Modules[0].GetLane("inbox")
	lane1.MaxAgents = 0
	lane1.AgentBehavior = []string{"do something"}
	lane1.DirAbs = filepath.Join(tempDir, proj.Modules[0].Dir, "inbox")
	if err := os.MkdirAll(lane1.DirAbs, 0o755); err != nil {
		t.Fatal(err)
	}

	lane2 := proj.Modules[0].GetLane("ready")
	lane2.MaxAgents = 0
	lane2.AgentBehavior = []string{"do something"}
	lane2.DirAbs = filepath.Join(tempDir, proj.Modules[0].Dir, "ready")
	if err := os.MkdirAll(lane2.DirAbs, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create work items in two different lanes
	createWorkItemDir(t, lane1, "TASK-001-test-item-one")
	createWorkItemDir(t, lane2, "TASK-002-test-item-two")

	// Scan to populate cache
	scanLaneDirectory(lane1)
	scanLaneDirectory(lane2)

	var maxConcurrent atomic.Int32
	var currentConcurrent atomic.Int32
	var wg sync.WaitGroup

	invoker := func(project *Project, mod *Module, lane *Lane, item *WorkItem) (InvocationHandle, error) {
		handle := InvocationHandle{
			Item: item,
			Done: make(chan struct{}),
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			n := currentConcurrent.Add(1)
			for {
				old := maxConcurrent.Load()
				if n <= old {
					break
				}
				if maxConcurrent.CompareAndSwap(old, n) {
					break
				}
			}
			time.Sleep(100 * time.Millisecond)
			currentConcurrent.Add(-1)
			close(handle.Done)
		}()

		return handle, nil
	}

	executor := NewExecutor(proj, invoker)
	executor.tick()

	wg.Wait()

	// With project max_agents=10 (default), both items should run
	if max := maxConcurrent.Load(); max < 2 {
		t.Errorf("expected at least 2 concurrent, got %d", max)
	}
}

// TestAtomicSlotClaim verifies that tryClaimSlot is atomic and prevents race conditions.
func TestAtomicSlotClaim(t *testing.T) {
	proj, _ := createTempProject(t)

	lane := proj.Modules[0].GetLane("inbox")
	lane.MaxAgents = 1
	lane.AgentBehavior = []string{"do something"}

	// Create two work items
	item1, err := lane.CreateWorkItem("test-item-one")
	if err != nil {
		t.Fatalf("failed to create item1: %v", err)
	}
	item2, err := lane.CreateWorkItem("test-item-two")
	if err != nil {
		t.Fatalf("failed to create item2: %v", err)
	}

	executor := NewExecutor(proj, nil)

	// Simulate concurrent claims
	var claimed1, claimed2 atomic.Bool
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		claimed1.Store(executor.tryClaimSlot(item1))
	}()
	go func() {
		defer wg.Done()
		claimed2.Store(executor.tryClaimSlot(item2))
	}()

	wg.Wait()

	// Only one should be claimed
	c1 := claimed1.Load()
	c2 := claimed2.Load()

	if c1 && c2 {
		t.Error("both items were claimed, but max_agents=1 should allow only one")
	}
	if !c1 && !c2 {
		t.Error("neither item was claimed, but at least one should have been")
	}
}