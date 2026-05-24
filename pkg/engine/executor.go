package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// Executor manages the execution loop for a project
type Executor struct {
	project      *Project
	invoker      WorkItemInvoker
	hookExecutor *HookExecutor
	mu           sync.Mutex
	active       map[string]InvocationHandle // track active invocations by handle ID
	done         chan struct{}
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

// NewExecutor creates a new executor for the given project
func NewExecutor(project *Project, invoker WorkItemInvoker) *Executor {
	ctx, cancel := context.WithCancel(context.Background())
	return &Executor{
		project:      project,
		invoker:      invoker,
		hookExecutor: NewHookExecutor(project.DirAbs),
		active:       make(map[string]InvocationHandle),
		done:         make(chan struct{}),
		ctx:          ctx,
		cancel:       cancel,
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

			// Try to execute each work item
			for item := range lane.WorkItems() {
				if item.InProgress {
					continue
				}

				// Check if this item should be ignored
				if item.shouldIgnore() {
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

// invokeItem starts execution of a work item with pre/post hooks
func (e *Executor) invokeItem(module *Module, lane *Lane, item *WorkItem) error {
	// Mark item as in progress
	item.InProgress = true

	// Resolve hooks for this module/lane
	preHooks, postHooks := e.project.ResolveHooksForLane(module, lane)

	// Build wildcard variables
	itemJSON, _ := json.Marshal(item.Alias())
	
	wildcardVars := map[string]string{
		"WI":          item.RelativePath(),
		"MODULE":      module.Name,
		"LANE":        lane.Name,
		"PROJECT_DIR": e.project.DirAbs,
		"WI_JSON":     string(itemJSON),
	}

	// Execute pre-hooks
	for _, hook := range preHooks {
		result := e.hookExecutor.Execute(hook.Name, hook.Definition, wildcardVars)
		if result.ExitCode != 0 {
			// Pre-hook failed → abort, create HOOK-FAIL artifact
			log.Printf("ERROR: pre-hook %s failed with exit code %d: %s", hook.Name, result.ExitCode, result.Stderr)
			
			// Create FAIL artifact
			if err := CreateHookFailArtifact(item, hook, result); err != nil {
				log.Printf("WARNING: failed to create HOOK-FAIL artifact: %v", err)
			}

			item.InProgress = false
			return fmt.Errorf("pre-hook %s failed with exit code %d", hook.Name, result.ExitCode)
		}
	}

	// Invoke the agent
	handle, err := e.invoker(e.project, module, lane, item)
	if err != nil {
		item.InProgress = false
		// fmt.Errorf("failed to invoke agent for item %s: %w", item.Name, err)
		return err
	}

	// Track the invocation
	e.mu.Lock()
	e.active[item.ID] = handle
	e.mu.Unlock()

	// Start a goroutine to handle completion (agent + post-hooks)
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()

		// Wait for agent completion
		_ = handle.Wait()

		// Execute post-hooks
		for _, hook := range postHooks {
			result := e.hookExecutor.Execute(hook.Name, hook.Definition, wildcardVars)
			if result.ExitCode != 0 {
				// Post-hook failed → log warning, continue
				log.Printf("WARNING: post-hook %s failed with exit code %d: %s", hook.Name, result.ExitCode, result.Stderr)
			}
		}

		// Clean up
		e.mu.Lock()
		delete(e.active, item.ID)
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