package project

import (
	"fmt"
	"strings"
	"sync"

	"github.com/nidorx/orqen/pkg/utils"
)

// Project represents the top-level project configuration (.orqen/orqen.yaml).
type Project struct {
	Id        string     `yaml:"-"` // directory hash
	DirAbs    string     `yaml:"-"` // absolute directory path
	Agents    Agent      `yaml:"agents"`
	Execution *Execution `yaml:"execution"`
	Modules   []*Module  `yaml:"modules"`

	// Runtime state (not serialized)
	mu       sync.Mutex
	invoker  AgentInvoker
	executor *Executor
	running  bool
}

// GetModule returns a module by name, or nil if not found
func (p *Project) GetModule(name string) *Module {
	for _, mod := range p.Modules {
		if mod.Name == name {
			return mod
		}
	}
	return nil
}

// ActiveAgentCount returns the total number of active agents across all modules
func (p *Project) ActiveAgentCount() int {
	count := 0
	for _, mod := range p.Modules {
		count += mod.ActiveItemCount()
	}
	return count
}

// HasAvailableSlot checks if there's room for more agents at the project level
func (p *Project) HasAvailableSlot() bool {
	if p.Execution.MaxAgents <= 0 {
		return true
	}
	return p.ActiveAgentCount() < p.Execution.MaxAgents
}

// WithInvoker sets the agent invoker for the project
func (p *Project) WithInvoker(invoker AgentInvoker) *Project {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.invoker = invoker
	return p
}

// withInvokerOld sets the agent invoker for the project
func (p *Project) withInvokerOld(invoker WorkItemInvoker) *Project {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.executor != nil {
		p.executor.Stop()
	}
	p.executor = NewExecutor(p, invoker)
	return p
}

// Start inicia a execução do projeto
func (p *Project) Start() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return
	}

	if p.executor == nil {
		// Use a default no-op invoker if none is set
		p.executor = NewExecutor(p, agentInvoker)
	}

	p.running = true
	go p.executor.Run()
}

// Stop finaliza a execução do projeto
func (p *Project) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return
	}

	if p.executor != nil {
		p.executor.Stop()
	}
	p.running = false
}

// IsRunning returns true if the project is currently running
func (p *Project) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

// agentInvoker is a default invoker that does nothing
// Invoke implements AgentInvoker for the noop invoker
func agentInvoker(proj *Project, mod *Module, lane *Lane, item *WorkItem) (InvocationHandle, error) {
	handle := InvocationHandle{
		ID:   utils.HashXxh64([]byte(fmt.Sprintf("%d-%s", item.ID, item.Name))),
		Item: item,
		Done: make(chan struct{}),
	}

	item.JobID = handle.ID

	go func() {

		var prompt strings.Builder

		prompt.WriteString(mod.Prompt)
		prompt.WriteString("\n")
		prompt.WriteString(lane.Prompt)
		prompt.WriteString("\n\n")

		prompt.WriteString("# EXECUTION CONTEXT (Auto-Gathered)\n")
		prompt.WriteString("**REQUIRED ACTION:** Work on item bellow\n")
		prompt.WriteString(fmt.Sprintf("- lane_name: %s\n", lane.Name))
		prompt.WriteString(fmt.Sprintf("- lane_dir: %s\n", lane.Dir))
		if item.ID == 0 {
			prompt.WriteString("- item_id: NOT CREATED (0), see tool orqen_create_item from orqen MCP Server\n")
		} else {
			prompt.WriteString(fmt.Sprintf("- item_id: %d\n", item.ID))
		}
		prompt.WriteString(fmt.Sprintf("- item_name: %s\n", item.Name))
		prompt.WriteString(fmt.Sprintf("- item_last_update: %v\n", item.ModTime))

		prompt.WriteString("- item_files:\n")
		for _, v := range item.Files {
			prompt.WriteString(fmt.Sprintf("    - `%v`\n", v))
		}

		handle.err = proj.invoker(prompt.String(), item)

		close(handle.Done)
	}()
	return handle, nil
}
