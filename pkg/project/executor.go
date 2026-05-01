package project

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Executor manages the execution loop for a project
type Executor struct {
	project *Project
	invoker WorkItemInvoker
	mu      sync.Mutex
	active  map[string]InvocationHandle // track active invocations by handle ID
	done    chan struct{}
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewExecutor creates a new executor for the given project
func NewExecutor(project *Project, invoker WorkItemInvoker) *Executor {
	ctx, cancel := context.WithCancel(context.Background())
	return &Executor{
		project: project,
		invoker: invoker,
		active:  make(map[string]InvocationHandle),
		done:    make(chan struct{}),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Run starts the execution loop
func (e *Executor) Run() {
	defer close(e.done)

	sleepInterval := time.Duration(e.project.Execution.SleepIntervalSeconds) * time.Second
	if sleepInterval <= 0 {
		sleepInterval = 60 * time.Second
	}

	ticker := time.NewTicker(sleepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			e.tick()
		}
	}
}

// tick performs one iteration of the execution loop
func (e *Executor) tick() {
	// Clean up completed invocations
	e.cleanupCompleted()

	// Check if we have available slots
	if !e.project.HasAvailableSlot() {
		return
	}

	// Try to find and execute work items
	e.processWorkItems()
}

// cleanupCompleted removes completed invocations from the active map
func (e *Executor) cleanupCompleted() {
	e.mu.Lock()
	defer e.mu.Unlock()

	for id, handle := range e.active {
		if handle.IsComplete() {
			// Mark the work item as no longer in progress
			handle.Item.JobID = ""
			handle.Item.InProgress = false
			delete(e.active, id)
		}
	}
}

// processWorkItems scans all lanes and starts execution for eligible work items
func (e *Executor) processWorkItems() {
	for _, mod := range e.project.Modules {
		// Get lanes in the configured order
		lanes := mod.GetLanesInOrder()

		for _, lane := range lanes {

			// no agent actions
			if len(lane.AgentBehavior) == 0 {
				continue
			}

			// Check if this lane has available slots
			if !lane.HasAvailableSlot() {
				continue
			}

			// Check if project has available slots
			if !e.project.HasAvailableSlot() {
				return
			}

			// Get work items from this lane
			items := lane.ListItems()
			if len(items) == 0 {
				continue
			}

			// Try to execute each work item
			for _, item := range items {
				if item.InProgress {
					continue
				}

				// Check if this item should be ignored
				if e.shouldIgnoreItem(mod, lane, item) {
					continue
				}

				// Start execution
				if err := e.invokeItem(mod, lane, item); err != nil {
					// Log error but continue with other items
					continue
				}

				// If we've reached max agents, stop processing
				if !e.project.HasAvailableSlot() {
					return
				}
			}
		}
	}
}

// shouldIgnoreItem checks if a work item should be skipped based on ignore rules
func (e *Executor) shouldIgnoreItem(module *Module, lane *Lane, item *WorkItem) bool {

	// Check ignore_if_exists
	if len(lane.IgnoreIfExists) > 0 {
		if HasItemsInReferencedLanes(e.project, module, lane.IgnoreIfExists) {
			return true
		}
	}

	// Check ignore_if_dependency
	if len(lane.IgnoreIfDependency) > 0 {
		if HasDependencyInReferencedLanes(e.project, module, item, lane.IgnoreIfDependency) {
			return true
		}
	}

	// ignore if recently updated
	if item.ModTime.After(time.Now().Add(-30 * time.Second)) {
		return true
	}

	return false
}

// invokeItem starts execution of a work item
func (e *Executor) invokeItem(module *Module, lane *Lane, item *WorkItem) error {
	// Mark item as in progress
	item.InProgress = true

	// Invoke the agent
	handle, err := e.invoker(e.project, module, lane, item)
	if err != nil {
		item.InProgress = false
		return fmt.Errorf("failed to invoke agent for item %s: %w", item.Name, err)
	}

	// Track the invocation
	e.mu.Lock()
	e.active[handle.ID] = handle
	e.mu.Unlock()

	// Start a goroutine to handle completion
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()

		// Wait for completion
		_ = handle.Wait()

		// Clean up
		e.mu.Lock()
		delete(e.active, handle.ID)
		e.mu.Unlock()

		// Mark item as no longer in progress
		item.InProgress = false
	}()

	return nil
}

// Stop signals the executor to stop and waits for active invocations to complete
func (e *Executor) Stop() {
	e.cancel()
	<-e.done
	e.wg.Wait()
}
