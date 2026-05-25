package engine

import (
	"fmt"
	"iter"
	"strings"
	"sync"
)

// Chat holds the top-level chat configuration, parsed from orqen.yaml
// under the "chat:" key.
type Chat struct {
	Agent    string   `yaml:"agent"`
	Telegram Telegram `yaml:"telegram"`
}

// Telegram holds the Telegram bot token configuration.
type Telegram struct {
	Token string `yaml:"token"`
}

// ToolDef represents a user-defined tool configuration from orqen.yaml.
type ToolDef struct {
	Command     []string            `yaml:"command,omitempty"`     // default command
	Timeout     int                 `yaml:"timeout,omitempty"`     // timeout in seconds (default 30)
	Description string              `yaml:"description,omitempty"` // tool description for MCP
	OSCommands  map[string][]string `yaml:",inline"`               // os-specific commands (windows, darwin, linux)
	Args        map[string]string   `yaml:"args,omitempty"`        // param_name -> description
}

// ValidOSKeys returns the set of valid OS keys for tool definitions.
var ValidOSKeys = map[string]bool{
	"windows": true,
	"darwin":  true,
	"linux":   true,
}

// GetCommandForOS returns the command array for the current OS, falling back to the default command.
func (t *ToolDef) GetCommandForOS(goos string) ([]string, error) {
	// Check OS-specific override
	if cmd, ok := t.OSCommands[goos]; ok {
		return cmd, nil
	}
	// Fall back to default
	if len(t.Command) > 0 {
		return t.Command, nil
	}
	return nil, fmt.Errorf("no command defined for tool (neither default nor %s-specific)", goos)
}

// Project represents the top-level project configuration (.orqen/orqen.yaml).
type Project struct {
	Id         string                     `yaml:"-"` // directory hash
	DirAbs     string                     `yaml:"-"` // absolute directory path
	Chat       *Chat                      `yaml:"chat"`
	Agents     Agent                      `yaml:"agents"`
	McpServers map[string]McpServerConfig `yaml:"mcpServers"`
	Execution  *Execution                 `yaml:"execution"`
	Modules    []*Module                  `yaml:"modules"`
	NamedHooks NamedHooks                 `yaml:"hooks,omitempty"` // named hook definitions
	Tools      map[string]ToolDef         `yaml:"tools,omitempty"` // user-defined dynamic tools

	// Runtime state (not serialized)
	mu       sync.Mutex
	fsys     *Fsys
	running  bool
	executor *Executor
	invoker  AgentInvoker
}

// ResolveHooksForLane resolves hooks for a given module/lane combination.
func (p *Project) ResolveHooksForLane(mod *Module, lane *Lane) (preHooks, postHooks []*ResolvedHook) {
	if p.NamedHooks == nil {
		return nil, nil
	}

	var modHooks, laneHooks *HookBindings
	if mod.Hooks != nil {
		modHooks = mod.Hooks
	}
	if lane.Hooks != nil {
		laneHooks = lane.Hooks
	}

	return ResolveHooks(modHooks, laneHooks, p.NamedHooks)
}

// GetModule returns a module by name, or nil if not found.
func (p *Project) GetModule(name string) *Module {
	for _, mod := range p.Modules {
		if strings.EqualFold(mod.Name, name) {
			return mod
		}
	}
	return nil
}

// findTargetModule scans all modules and lanes to find the module
// that contains a work item with the given ID.
func (p *Project) FindModule(module *string) (*Module, error) {
	var targetModule *Module
	if module != nil && *module != "" {
		if targetModule = p.GetModule(*module); targetModule == nil {
			return nil, fmt.Errorf("module not found: %s", *module)
		} else {
			return targetModule, nil
		}
	}

	if len(p.Modules) == 1 {
		return p.Modules[0], nil
	}

	return nil, nil
}

// WorkItems returns all work items in this project.
func (p *Project) WorkItems() iter.Seq[*WorkItem] {
	return func(yield func(*WorkItem) bool) {
		for _, mod := range p.Modules {
			for item := range mod.WorkItems() {
				if !yield(item) {
					// yield returns false if the loop should stop (e.g., 'break' was called)
					return
				}
			}
		}
	}
}

// ActiveAgentCount returns the total number of active agents across all modules.
func (p *Project) ActiveAgentCount() int {
	count := 0
	for _, mod := range p.Modules {
		count += mod.ActiveItemCount()
	}
	return count
}

// HasAvailableSlot checks if there's room for more agents at the project level.
func (p *Project) HasAvailableSlot() bool {
	if p.Execution.MaxAgents <= 0 {
		return true
	}
	return p.ActiveAgentCount() < p.Execution.MaxAgents
}

// WithInvoker sets the agent invoker for the project.
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

// Start begins project execution.
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

// Stop terminates project execution.
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

// IsRunning returns true if the project is currently running.
func (p *Project) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

// agentInvoker is a default invoker that wraps the project's configured invoker,
// building a prompt from module/lane context and work item metadata.
func agentInvoker(proj *Project, mod *Module, lane *Lane, item *WorkItem) (InvocationHandle, error) {
	handle := InvocationHandle{
		Item: item,
		Done: make(chan struct{}),
	}

	go func() {

		var prompt strings.Builder

		prompt.WriteString(mod.Prompt)
		prompt.WriteString("\n")
		prompt.WriteString(lane.Prompt)
		prompt.WriteString("\n\n")

		prompt.WriteString("# EXECUTION CONTEXT (Auto-Gathered)\n")
		prompt.WriteString("**REQUIRED ACTION:** Work on item bellow\n")
		fmt.Fprintf(&prompt, "- lane_name: %s\n", lane.Name)
		fmt.Fprintf(&prompt, "- lane_dir: %s\n", lane.Dir)
		fmt.Fprintf(&prompt, "- module_name: %s\n", mod.Name)
		if item.Seq == 0 {
			prompt.WriteString("- workitem_seq: NOT CREATED (0), see tool workitem_create from orqen MCP Server\n")
		} else {
			fmt.Fprintf(&prompt, "- workitem_seq: %d\n", item.Seq)
		}
		fmt.Fprintf(&prompt, "- workitem_name: %s\n", item.Name)
		fmt.Fprintf(&prompt, "- workitem_last_update: %v\n", item.ModTime)

		prompt.WriteString("- workitem_files:\n")
		for _, v := range item.Files {
			fmt.Fprintf(&prompt, "    - `%v`\n", v)
		}

		handle.err = proj.invoker(prompt.String(), item)

		close(handle.Done)
	}()
	return handle, nil
}
