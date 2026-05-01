package project

import (
	"encoding/base64"
	"fmt"
	"strings"
	"sync"

	"github.com/cespare/xxhash/v2"
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
func agentInvoker(prj *Project, mod *Module, lan *Lane, itm *WorkItem) (InvocationHandle, error) {
	handle := InvocationHandle{
		ID:   hashXxh64([]byte(fmt.Sprintf("%d-%s", itm.ID, itm.Name))),
		Item: itm,
		Done: make(chan struct{}),
	}

	itm.JobID = handle.ID

	go func() {

		var prompt strings.Builder

		prompt.WriteString(mod.Prompt)
		prompt.WriteString("\n")
		prompt.WriteString(lan.Prompt)
		prompt.WriteString("\n\n")

		prompt.WriteString("========================================\n")
		prompt.WriteString("PRE-EXECUTION CONTEXT (Auto-Gathered)\n")
		prompt.WriteString("========================================\n\n")

		prompt.WriteString("**RECOMMENDED ACTION (auto-determined):**\n")
		prompt.WriteString(fmt.Sprintf("   → Start %s from %s\n\n", itm.Name, lan.Dir))

		prompt.WriteString("**RELATED RESOURCES:**\n")
		for _, v := range itm.Files {
			prompt.WriteString(fmt.Sprintf("- `%v`\n", v))
		}
		prompt.WriteString("\n")

		prompt.WriteString("========================================\n")
		prompt.WriteString("END OF PRE-EXECUTION CONTEXT\n")
		prompt.WriteString("========================================\n")

		handle.err = prj.invoker(
			prompt.String(),
			itm,
		)

		close(handle.Done)
	}()
	return handle, nil
}

// Xxh64 return a base64-encoded checksum of a resource using Xxh64 algorithm
//
// Encoded using Base64 URLSafe
func hashXxh64(content []byte) string {
	h := xxhash.New()
	h.Write(content)
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}
