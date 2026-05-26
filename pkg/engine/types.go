package engine

import (
	"embed"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
)

//go:embed prompts
var embedPromptsFS embed.FS

const (

	// projectConfigDir is the name of the directory containing project configuration.
	projectConfigDir = ".orqen"
	// projectConfigFile is the name of the project configuration file.
	projectConfigFile = "orqen.yaml"
)

// Agent holds project-level agent settings.
//
// YAML structure:
//
//	agents:
//	  default: "qwen"              # default agent client name
//	  clients:
//	    qwen:
//	      command: ["qwen", "--yolo", "--acp"]
type Agent struct {
	Default string                 `yaml:"default"`
	Clients map[string]AgentClient `yaml:"clients"`
}

func (a *Agent) GetName(agent string) string {
	if agent == "" {
		agent = a.Default
	}

	return agent
}

func (a *Agent) GetCommand(agent string) []string {
	if agent == "" {
		agent = a.Default
	}

	client := a.Clients[agent]

	return client.Command
}

// AgentClient defines a single agent client configuration.
//
// YAML structure:
//
//	command: ["qwen", "--yolo", "--acp"]   # CLI command and arguments to invoke the agent
type AgentClient struct {
	Command []string `yaml:"command"`
}

// McpServerStdioConfig defines an MCP server transport via stdio.
//
// YAML structure:
//
//	stdio:
//	  command: "npx"
//	  args: ["-y", "@modelcontextprotocol/server-postgres", "postgresql://localhost/db"]
//	  env:
//	    - name: "API_KEY"
//	      value: "secret"
type McpServerStdioConfig struct {
	Command string      `yaml:"command"`
	Args    []string    `yaml:"args"`
	Env     []McpEnvVar `yaml:"env"`
}

// McpEnvVar defines an environment variable for an MCP server.
type McpEnvVar struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

// McpServerHttpConfig defines an MCP server transport via HTTP.
//
// YAML structure:
//
//	http:
//	  url: "https://search.example.com/mcp"
//	  headers:
//	    Authorization: "Bearer token"
type McpServerHttpConfig struct {
	Url     string            `yaml:"url"`
	Headers map[string]string `yaml:"headers"`
}

// McpServerConfig defines a named MCP server with either stdio or http transport.
//
// YAML structure:
//
//	mcpServers:
//	  database:
//	    stdio:
//	      command: "npx"
//	      args: ["-y", "@modelcontextprotocol/server-postgres", "postgresql://localhost/db"]
//	  search:
//	    http:
//	      url: "https://search.example.com/mcp"
//	      headers:
//	        Authorization: "Bearer token"
type McpServerConfig struct {
	Stdio *McpServerStdioConfig `yaml:"stdio"`
	Http  *McpServerHttpConfig  `yaml:"http"`
}

// Execution holds project-level execution settings.
//
// YAML structure:
//
//	execution:
//	  max_agents: 10                  # maximum concurrent agents across all modules (0 = unlimited)
//	  sleep_interval_seconds: 60      # seconds between work cycles
type Execution struct {
	MaxAgents            int `yaml:"max_agents"`
	SleepIntervalSeconds int `yaml:"sleep_interval_seconds"`
}

// AgentInvoker defines the function signature for invoking agent executions.
// It receives the synthesized prompt and the work item being processed.
type AgentInvoker func(prompt string, item *WorkItem) error

// WorkItemInvoker defines the function signature for invoking and managing agent executions.
// Returns an InvocationHandle that can be used to track progress.
type WorkItemInvoker func(project *Project, module *Module, lane *Lane, item *WorkItem) (InvocationHandle, error)

// InvocationHandle represents a running agent invocation.
type InvocationHandle struct {
	Item *WorkItem     // the work item being processed
	Done chan struct{} // closed when the invocation completes
	err  error         // error from the invocation (if any)
}

// Wait blocks until the invocation completes.
func (h InvocationHandle) Wait() error {
	<-h.Done
	return h.err
}

// IsComplete returns true if the invocation has completed.
func (h InvocationHandle) IsComplete() bool {
	select {
	case <-h.Done:
		return true
	default:
		return false
	}
}

// SchemaField describes an observed front matter field across a module.
type SchemaField struct {
	Field  string   `json:"field"`  // field name
	Types  []string `json:"types"`  // observed Go/YAML types (string, bool, int, list, map)
	Values []any    `json:"values"` // unique observed values (up to schemaMaxValues)
}

// ScheduleFrequency defines the scheduling frequency type.
type ScheduleFrequency string

const (
	ScheduleDaily   ScheduleFrequency = "daily"
	ScheduleWeekly  ScheduleFrequency = "weekly"
	ScheduleMonthly ScheduleFrequency = "monthly"
	ScheduleCron    ScheduleFrequency = "cron"
)

// LaneSchedule defines a schedule configuration for lane execution windows.
// Only one schedule mode is active at a time based on the Frequency field.
type LaneSchedule struct {
	Frequency      ScheduleFrequency `yaml:"frequency"`                // "daily", "weekly", "monthly", or "cron"
	Time           []string          `yaml:"time"`                     // HH:MM time(s) — required for daily/weekly/monthly
	DaysOfWeek     []string          `yaml:"daysOfWeek,omitempty"`     // Day names for weekly (Monday-Sunday)
	DaysOfMonth    []int             `yaml:"daysOfMonth,omitempty"`    // Day numbers for monthly (1-31)
	CronExpression string            `yaml:"cronExpression,omitempty"` // Standard cron expression for custom schedules
}

// IsDue returns true if the current time matches the schedule window.
// Returns true for nil receiver (no schedule → always eligible).
func (ls *LaneSchedule) IsDue(now time.Time) bool {
	if ls == nil {
		return true // no schedule → always eligible
	}
	switch ls.Frequency {
	case ScheduleDaily:
		return ls.matchesTime(now)
	case ScheduleWeekly:
		return ls.matchesDayOfWeek(now) && ls.matchesTime(now)
	case ScheduleMonthly:
		return ls.matchesDayOfMonth(now) && ls.matchesTime(now)
	case ScheduleCron:
		return ls.matchesCron(now)
	default:
		return true
	}
}

// scheduleTolerance defines the default window tolerance in minutes
// for time matching. If the executor tick falls within this window
// of a configured time, the lane is eligible.
const scheduleTolerance = 2 * time.Minute

// matchesTime checks if now is within tolerance of any configured Time entry.
func (ls *LaneSchedule) matchesTime(now time.Time) bool {
	currentMinutes := now.Hour()*60 + now.Minute()
	for _, t := range ls.Time {
		parts := strings.Split(t, ":")
		if len(parts) != 2 {
			continue
		}
		hour := parseIntSafe(parts[0])
		minute := parseIntSafe(parts[1])
		configuredMinutes := hour*60 + minute

		diff := currentMinutes - configuredMinutes
		if diff < 0 {
			diff = -diff
		}
		// Handle midnight crossing: e.g., 23:59 vs 00:01
		if diff > 720 { // 12 hours
			diff = 1440 - diff
		}
		if time.Duration(diff)*time.Minute <= scheduleTolerance {
			return true
		}
	}
	return false
}

// matchesDayOfWeek checks if now.Weekday() is in DaysOfWeek.
func (ls *LaneSchedule) matchesDayOfWeek(now time.Time) bool {
	if len(ls.DaysOfWeek) == 0 {
		return true
	}
	currentDay := strings.ToLower(now.Weekday().String())
	for _, day := range ls.DaysOfWeek {
		if strings.ToLower(day) == currentDay {
			return true
		}
	}
	return false
}

// matchesDayOfMonth checks if now.Day() is in DaysOfMonth.
func (ls *LaneSchedule) matchesDayOfMonth(now time.Time) bool {
	if len(ls.DaysOfMonth) == 0 {
		return true
	}
	currentDay := now.Day()
	for _, day := range ls.DaysOfMonth {
		if day == currentDay {
			return true
		}
	}
	return false
}

// matchesCron parses and evaluates a standard 5-field cron expression.
// Fields: minute hour day-of-month month day-of-week
// Supports: *, */N, N, N-M, N,M,O
func (ls *LaneSchedule) matchesCron(now time.Time) bool {
	if ls.CronExpression == "" {
		return false
	}
	fields := strings.Fields(ls.CronExpression)
	if len(fields) < 5 || len(fields) > 6 {
		return false
	}

	// Use first 5 fields (ignore year if 6-field)
	minuteField := fields[0]
	hourField := fields[1]
	domField := fields[2]
	monthField := fields[3]
	dowField := fields[4]

	return matchCronField(minuteField, now.Minute(), 0, 59) &&
		matchCronField(hourField, now.Hour(), 0, 23) &&
		matchCronField(domField, now.Day(), 1, 31) &&
		matchCronField(monthField, int(now.Month()), 1, 12) &&
		matchCronFieldDow(dowField, int(now.Weekday()), 0, 6)
}

// parseIntSafe parses a string to int, returns 0 on failure.
func parseIntSafe(s string) int {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// matchCronField checks if a value matches a single cron field expression.
// Supports: *, */N, N, N-M, N,M,O
func matchCronField(expr string, value, minVal, maxVal int) bool {
	if expr == "*" {
		return true
	}

	// Handle */N (step)
	if strings.HasPrefix(expr, "*/") {
		step := parseIntSafe(expr[2:])
		if step <= 0 {
			return false
		}
		return (value-minVal)%step == 0
	}

	// Handle comma-separated list
	parts := strings.Split(expr, ",")
	for _, part := range parts {
		// Handle range N-M
		if idx := strings.Index(part, "-"); idx != -1 {
			start := parseIntSafe(part[:idx])
			end := parseIntSafe(part[idx+1:])
			if value >= start && value <= end {
				return true
			}
		} else {
			// Exact value
			if parseIntSafe(part) == value {
				return true
			}
		}
	}
	return false
}

// matchCronFieldDow handles day-of-week matching with Sunday=0/7 convention.
// Go's time.Weekday(): Sunday=0, Monday=1, ..., Saturday=6
// Cron: Sunday=0 or 7, Monday=1, ..., Saturday=6
func matchCronFieldDow(expr string, value, minVal, maxVal int) bool {
	// Normalize: if value is 0 (Sunday), also match 7
	if expr == "*" {
		return true
	}

	// Handle */N (step)
	if strings.HasPrefix(expr, "*/") {
		step := parseIntSafe(expr[2:])
		if step <= 0 {
			return false
		}
		return (value-minVal)%step == 0
	}

	// Handle comma-separated list
	parts := strings.Split(expr, ",")
	for _, part := range parts {
		// Handle range N-M
		if idx := strings.Index(part, "-"); idx != -1 {
			start := parseIntSafe(part[:idx])
			end := parseIntSafe(part[idx+1:])
			// Sunday can be 0 or 7
			if (value >= start && value <= end) || (value == 0 && end == 7 && start <= 7) {
				return true
			}
		} else {
			// Exact value (match 0 or 7 for Sunday)
			n := parseIntSafe(part)
			if n == value || (value == 0 && n == 7) || (value == 7 && n == 0) {
				return true
			}
		}
	}
	return false
}

// laneScheduleYAML is used for custom YAML unmarshaling
type laneScheduleYAML struct {
	Frequency      ScheduleFrequency `yaml:"frequency"`
	Time           any               `yaml:"time"`
	DaysOfWeek     []string          `yaml:"daysOfWeek,omitempty"`
	DaysOfMonth    []int             `yaml:"daysOfMonth,omitempty"`
	CronExpression string            `yaml:"cronExpression,omitempty"`
}

// UnmarshalYAML implements custom YAML parsing for LaneSchedule.
// Supports both single string and array formats for the Time field:
//
//	time: "02:00"        → []string{"02:00"}
//	time: ["02:00"]      → []string{"02:00"}
//	time: ["02:00", "06:00"] → []string{"02:00", "06:00"}
func (ls *LaneSchedule) UnmarshalYAML(b []byte) error {
	var aux laneScheduleYAML
	if err := yaml.Unmarshal(b, &aux); err != nil {
		return err
	}

	ls.Frequency = aux.Frequency
	ls.DaysOfWeek = aux.DaysOfWeek
	ls.DaysOfMonth = aux.DaysOfMonth
	ls.CronExpression = aux.CronExpression

	// Normalize Time field from any to []string
	switch v := aux.Time.(type) {
	case string:
		ls.Time = []string{v}
	case []any:
		ls.Time = make([]string, len(v))
		for i, item := range v {
			if s, ok := item.(string); ok {
				ls.Time[i] = s
			} else {
				return fmt.Errorf("schedule.time[%d]: expected string, got %T", i, item)
			}
		}
	case nil:
		ls.Time = []string{}
	default:
		return fmt.Errorf("schedule.time: expected string or array, got %T", v)
	}

	return nil
}

// HookResult holds the outcome of a hook execution.
type HookResult struct {
	HookName string
	ExitCode int
	Stdout   string
	Stderr   string
	Duration time.Duration
	Err      error // non-nil if command failed
}

// HookTimeoutError represents a timeout during hook execution.
type HookTimeoutError struct {
	HookName string
	Timeout  time.Duration
}

func (e *HookTimeoutError) Error() string {
	return fmt.Sprintf("hook %s timed out after %v", e.HookName, e.Timeout)
}

// HookDefinition maps a hook name to its command array.
// OS-specific variants use suffix: .windows, .darwin, .linux
type HookDefinition struct {
	Command []string      `yaml:"-"`                 // base command (no OS suffix)
	Windows []string      `yaml:"-"`                 // .windows variant
	Darwin  []string      `yaml:"-"`                 // .darwin variant
	Linux   []string      `yaml:"-"`                 // .linux variant
	Timeout time.Duration `yaml:"timeout,omitempty"` // default 5m
}

// GetCommandForOS resolves the command array for the given OS.
// Falls back to base Command if no OS-specific variant exists.
func (h *HookDefinition) GetCommandForOS(goos string) []string {
	switch goos {
	case "windows":
		if len(h.Windows) > 0 {
			return h.Windows
		}
	case "darwin":
		if len(h.Darwin) > 0 {
			return h.Darwin
		}
	case "linux":
		if len(h.Linux) > 0 {
			return h.Linux
		}
	}
	return h.Command
}

// GetCurrentOSCommand resolves the command array for the current OS.
func (h *HookDefinition) GetCurrentOSCommand() []string {
	return h.GetCommandForOS(runtime.GOOS)
}

// HookBinding represents a single hook reference, optionally negated.
// Negated bindings (!hook_name) exclude a module-level hook at lane level.
type HookBinding struct {
	Name    string
	Negated bool // true if prefixed with "!"
}

// UnmarshalYAML implements custom YAML parsing for HookBinding.
// Handles the "!hook_name" negation syntax (string prefix, not YAML tag).
func (hb *HookBinding) UnmarshalYAML(b []byte) error {
	s := strings.TrimSpace(string(b))
	// Remove surrounding quotes if present
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	} else if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
		s = s[1 : len(s)-1]
	}

	if strings.HasPrefix(s, "!") {
		hb.Negated = true
		hb.Name = strings.TrimPrefix(s, "!")
	} else {
		hb.Name = s
	}

	if hb.Name == "" {
		return fmt.Errorf("hook binding name cannot be empty")
	}

	return nil
}

// HookBindings holds pre/post hook lists for a module or lane.
type HookBindings struct {
	Pre  []HookBinding `yaml:"pre,omitempty"`
	Post []HookBinding `yaml:"post,omitempty"`
}

// NamedHooks is a map of hook name to its definition.
// Requires custom unmarshaling to handle OS-specific suffixes (.windows, .darwin, .linux).
type NamedHooks map[string]*HookDefinition

// hookDefinitionRaw is used for unmarshaling hook definitions with timeout
type hookDefinitionRaw struct {
	Command []string       `yaml:"command,omitempty"`
	Windows []string       `yaml:"windows,omitempty"`
	Darwin  []string       `yaml:"darwin,omitempty"`
	Linux   []string       `yaml:"linux,omitempty"`
	Timeout *time.Duration `yaml:"timeout,omitempty"`
}

// UnmarshalYAML implements custom YAML parsing for NamedHooks.
// Parses keys like "hook_name" (base), "hook_name.windows", "hook_name.darwin", "hook_name.linux".
func (nh *NamedHooks) UnmarshalYAML(b []byte) error {
	// First, try to parse as map[string]hookDefinitionRaw for timeout support
	var rawMap map[string]hookDefinitionRaw
	if err := yaml.Unmarshal(b, &rawMap); err == nil && len(rawMap) > 0 {
		if *nh == nil {
			*nh = make(NamedHooks)
		}

		for key, def := range rawMap {
			var (
				hookName string
				osSuffix string
			)

			// Split on last dot to handle hook names with dots (though unlikely)
			if idx := strings.LastIndex(key, "."); idx != -1 {
				potentialSuffix := key[idx+1:]
				if potentialSuffix == "windows" || potentialSuffix == "darwin" || potentialSuffix == "linux" {
					hookName = key[:idx]
					osSuffix = potentialSuffix
				} else {
					hookName = key
				}
			} else {
				hookName = key
			}

			if hookName == "" {
				return fmt.Errorf("hook name cannot be empty (key: %q)", key)
			}

			hook, exists := (*nh)[hookName]
			if !exists {
				hook = &HookDefinition{}
				(*nh)[hookName] = hook
			}

			switch osSuffix {
			case "windows":
				hook.Windows = def.Windows
			case "darwin":
				hook.Darwin = def.Darwin
			case "linux":
				hook.Linux = def.Linux
			default:
				hook.Command = def.Command
			}

			// Apply timeout if present (only on base definition)
			if def.Timeout != nil && osSuffix == "" {
				hook.Timeout = *def.Timeout
			}
		}

		return nil
	}

	// Fallback to legacy format: map[string][]string
	var raw map[string][]string
	if err := yaml.Unmarshal(b, &raw); err != nil {
		return err
	}

	if *nh == nil {
		*nh = make(NamedHooks)
	}

	for key, cmd := range raw {
		var (
			hookName string
			osSuffix string
		)

		// Split on last dot to handle hook names with dots (though unlikely)
		if idx := strings.LastIndex(key, "."); idx != -1 {
			potentialSuffix := key[idx+1:]
			if potentialSuffix == "windows" || potentialSuffix == "darwin" || potentialSuffix == "linux" {
				hookName = key[:idx]
				osSuffix = potentialSuffix
			} else {
				hookName = key
			}
		} else {
			hookName = key
		}

		if hookName == "" {
			return fmt.Errorf("hook name cannot be empty (key: %q)", key)
		}

		hook, exists := (*nh)[hookName]
		if !exists {
			hook = &HookDefinition{}
			(*nh)[hookName] = hook
		}

		switch osSuffix {
		case "windows":
			hook.Windows = cmd
		case "darwin":
			hook.Darwin = cmd
		case "linux":
			hook.Linux = cmd
		default:
			hook.Command = cmd
		}
	}

	return nil
}
