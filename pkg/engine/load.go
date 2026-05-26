package engine

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/nidorx/orqen/pkg/utils"
	"github.com/nidorx/orqen/pkg/utils/tinylfu"
)

var (
	projectsMu sync.Mutex
	projects   = map[string]*Project{}

	// use engram memory (WIP)
	FLAG_USE_MEMORY = false
)

func Get(id string) *Project {
	projectsMu.Lock()
	defer projectsMu.Unlock()
	return projects[id]
}

// Unregister removes a project from the global registry. Useful for test cleanup.
func Unregister(id string) {
	projectsMu.Lock()
	defer projectsMu.Unlock()
	delete(projects, id)
}

// Load loads and validates the project configuration from the given project directory.
// It expects the file .orqen/orqen.yaml to exist within the directory.
func Load(projectDir string) (*Project, error) {

	projectDir = filepath.Clean(projectDir)

	id := utils.HashXxh64([]byte(projectDir))

	projectsMu.Lock()
	defer projectsMu.Unlock()
	if proj, exists := projects[id]; exists {
		return proj, nil
	}

	// Validate directory structure
	if err := ValidateDir(projectDir); err != nil {
		return nil, fmt.Errorf("Invalid project directory: %v\n", err)
	}

	configPath := filepath.Join(projectDir, projectConfigDir, projectConfigFile)

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read project config file %s: %w", configPath, err)
	}

	var proj Project
	if err := yaml.Unmarshal(data, &proj); err != nil {
		return nil, fmt.Errorf("failed to parse project config file: %w", err)
	}

	proj.Id = id
	proj.DirAbs = projectDir

	// Apply defaults
	applyDefaults(&proj)

	// Validate configuration
	if err := validate(&proj); err != nil {
		return nil, fmt.Errorf("invalid project configuration: %w", err)
	}

	// create directories and prompts
	if err := initialize(&proj); err != nil {
		return nil, err
	}
	projects[id] = &proj
	return &proj, nil
}

// ValidateDir checks if the given directory contains a valid .orqen/orqen.yaml file.
// Returns an error if the directory doesn't exist, doesn't contain .orqen, or the config file is missing.
func ValidateDir(projectDir string) error {
	projectDir = filepath.Clean(projectDir)

	// @TODO: .lock file

	// Check if directory exists
	info, err := os.Stat(projectDir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("directory does not exist: %s", projectDir)
		}
		return fmt.Errorf("error accessing directory %s: %w", projectDir, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", projectDir)
	}

	// Check if .orqen directory exists
	orqenDir := filepath.Join(projectDir, projectConfigDir)
	orqenInfo, err := os.Stat(orqenDir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("directory %s does not contain a %s subdirectory", projectDir, projectConfigDir)
		}
		return fmt.Errorf("error accessing %s subdirectory: %w", projectConfigDir, err)
	}

	if !orqenInfo.IsDir() {
		return fmt.Errorf("%s exists but is not a directory", projectConfigDir)
	}

	// Check if orqen.yaml exists
	configPath := filepath.Join(orqenDir, projectConfigFile)
	if _, err := os.Stat(configPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s subdirectory does not contain %s", projectConfigDir, projectConfigFile)
		}
		return fmt.Errorf("error accessing config file %s: %w", configPath, err)
	}

	return nil
}

// applyDefaults sets sensible defaults for zero-value fields in ProjectConfig.
func applyDefaults(proj *Project) {

	if proj.Execution == nil {
		proj.Execution = &Execution{}
	}

	if proj.Execution.MaxAgents <= 0 {
		proj.Execution.MaxAgents = 10
	}

	if proj.Execution.SleepIntervalSeconds <= 0 {
		proj.Execution.SleepIntervalSeconds = 60
	}

	// Apply defaults for each module
	for _, mod := range proj.Modules {
		mod.Project = proj

		mod.Name = strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(mod.Name), " ", ""), " ", "_")

		if mod.Dir == "" {
			mod.Dir = "."
		}

		mod.DirAbs = filepath.Clean(filepath.Join(proj.DirAbs, mod.Dir))
		mod.DirPrompts = filepath.Clean(filepath.Join(proj.DirAbs, projectConfigDir, mod.Name, "prompts", "generated"))

		mod.Prefix = strings.ReplaceAll(
			strings.ReplaceAll(
				strings.ReplaceAll(strings.ToUpper(strings.TrimSpace(mod.Prefix)), " ", ""),
				" ", "_",
			),
			"-", "_", // WorkItem pattern: ${PREFIX}-${SEQUENCE}-${SIMPLE_NAME}
		)
		if mod.Prefix == "" {
			mod.Prefix = "WI"
		}

		// special lane
		var inbox *Lane

		for _, lane := range mod.Lanes {
			if lane.Name == "inbox" {
				inbox = lane
				break
			}
		}

		if inbox == nil {
			inbox = &Lane{Name: "inbox"}
			mod.Lanes = append([]*Lane{inbox}, mod.Lanes...)
		}

		if inbox.Purpose == "" {
			inbox.Purpose = fmt.Sprintf("User ideas that are ready to be transformed into %s by the agent", mod.Prefix)
		}

		if inbox.MaxAgents <= 0 {
			inbox.MaxAgents = 2
		}

		if strings.TrimSpace(strings.Join(inbox.AgentBehavior, "")) == "" {
			inbox.AgentBehavior = nil
		}

		if len(inbox.AgentBehavior) == 0 {
			inbox.AgentBehavior = []string{
				"Read the inbox file to understand the idea",
				fmt.Sprintf("Analyze and decompose into %s.", mod.Prefix),
				fmt.Sprintf("An idea may result in one or more %s - the agent should make appropriate judgment", mod.Prefix),
				fmt.Sprintf(
					"Create %s using workitem_create (orqen MCP Server).", mod.Prefix,
				),
				"**Terminate execution** after processing the inbox file",
			}
		}

		if len(inbox.CriticalRules) == 0 {
			inbox.CriticalRules = []string{
				fmt.Sprintf("Move the idea file to the directory of the first %s created from that idea. Do not remove the file, just move it.", mod.Prefix),
			}
		}

		mod.workItemsBySeq = tinylfu.NewSyncCacheT[*WorkItem](10000, 100000, time.Duration(0))

		// keep in memory for 30 seconds only
		mod.workItemsStashed = tinylfu.NewSyncCacheT[*WorkItem](10000, 100000, 30*time.Second)

		for j, lane := range mod.Lanes {

			if strings.TrimSpace(strings.Join(lane.AgentBehavior, "")) == "" {
				lane.AgentBehavior = nil
			}

			if lane.MaxAgents <= 0 {
				lane.MaxAgents = 1
			}

			if lane.IgnoreIfModtime == 0 {
				lane.IgnoreIfModtime = 30
			}

			lane.Dir = fmt.Sprintf("%02d_%s", j+1, strings.ToLower(lane.Name))
			lane.DirAbs = filepath.Clean(path.Join(mod.DirAbs, lane.Dir))
			lane.Module = mod
			lane.workItemsByID = tinylfu.NewSyncCacheT[*WorkItem](10000, 100000, time.Duration(0))
		}
	}
}

// validate checks the project configuration for required fields.
func validate(proj *Project) error {
	// Validate hook definitions
	for hookName, hookDef := range proj.NamedHooks {
		if len(hookDef.Command) == 0 {
			return fmt.Errorf("hook %q has empty command array - must define at least a base command", hookName)
		}
		// Validate hook name is a valid identifier (alphanumeric, underscores, hyphens)
		for _, ch := range hookName {
			if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-') {
				return fmt.Errorf("hook name %q contains invalid character %q - only alphanumeric, underscores, and hyphens are allowed", hookName, ch)
			}
		}
	}

	if len(proj.Modules) == 0 {
		return fmt.Errorf("no modules defined")
	}

	for i, mod := range proj.Modules {
		if mod.Name == "" {
			return fmt.Errorf("module at index %d has no name", i)
		}
		if len(mod.Lanes) == 0 {
			return fmt.Errorf("module %q has no lanes defined", mod.Name)
		}

		// Validate module hook bindings reference existing hooks
		if mod.Hooks != nil {
			if err := validateHookBindings(mod.Hooks, proj.NamedHooks, "module "+mod.Name); err != nil {
				return err
			}
		}

		// Validate lane names are unique
		laneNames := make(map[string]bool)
		for j, lane := range mod.Lanes {
			if lane.Name == "" {
				return fmt.Errorf("module %q has lane at index %d with no name", mod.Name, j)
			}
			if laneNames[lane.Name] {
				return fmt.Errorf("module %q has duplicate lane name: %s", mod.Name, lane.Name)
			}
			laneNames[lane.Name] = true

			// Validate lane hook bindings reference existing hooks
			if lane.Hooks != nil {
				if err := validateHookBindings(lane.Hooks, proj.NamedHooks, "module "+mod.Name+" lane "+lane.Name); err != nil {
					return err
				}
			}

			// Validate MCP server references
			for _, mcpName := range lane.McpServers {
				if _, exists := proj.McpServers[mcpName]; !exists {
					return fmt.Errorf("module %q lane %q references unknown MCP server: %q", mod.Name, lane.Name, mcpName)
				}
			}

			// Validate lane schedule configuration
			if lane.Schedule != nil {
				if err := validateSchedule(lane.Schedule, mod.Name, lane.Name); err != nil {
					return err
				}
			}
		}
	}

	// Validate agent clients exist
	for name, client := range proj.Agents.Clients {
		if len(client.Command) == 0 {
			return fmt.Errorf("agent client %q has empty command", name)
		}
	}

	return nil
}

// validateHookBindings checks that all hook bindings reference existing named hooks.
func validateHookBindings(bindings *HookBindings, namedHooks NamedHooks, context string) error {
	if bindings == nil {
		return nil
	}

	validateBindings := func(bindings []HookBinding, bindingType string) error {
		for _, b := range bindings {
			if _, exists := namedHooks[b.Name]; !exists {
				return fmt.Errorf("%s %s binding references unknown hook: %q", context, bindingType, b.Name)
			}
		}
		return nil
	}

	if err := validateBindings(bindings.Pre, "pre"); err != nil {
		return err
	}
	if err := validateBindings(bindings.Post, "post"); err != nil {
		return err
	}

	return nil
}

// initialize create directories and prompts and watchers
func initialize(proj *Project) error {

	for _, mod := range proj.Modules {

		// create prompts directory
		modPromptsDir := mod.DirPrompts
		if err := os.MkdirAll(modPromptsDir, 0o755); err != nil {
			return err
		}

		var defaultPrompts = []string{
			"HEADER.md",
		}

		for _, promptFile := range defaultPrompts {
			dstPromptFileLoc := path.Join(modPromptsDir, promptFile)

			srcPromptFile, err := embedPromptsFS.Open("prompts/" + promptFile)
			if err != nil {
				return err
			}

			promptBytes, err := io.ReadAll(srcPromptFile)
			if err != nil {
				return err
			}
			prompt := string(promptBytes)

			if promptFile == "HEADER.md" {

				// obtém artefatos de exemplo
				var (
					artifacts         []string
					artifactsExamples []string
					artifactsByName   = map[string]bool{}
				)
				for i, lane := range mod.Lanes {
					if len(lane.AgentBehavior) > 0 && len(lane.Artifacts) > 0 {
						for _, artifact := range lane.Artifacts {
							if exists := artifactsByName[artifact]; !exists {
								artifactsByName[artifact] = true
								artifacts = append(artifacts, artifact)
								artifactsExamples = append(
									artifactsExamples,
									fmt.Sprintf("- `_$_MOD_PREFIX_$_-%04d-%s.md`", i+1, artifact),
								)
								artifactsExamples = append(
									artifactsExamples,
									fmt.Sprintf("- `_$_MOD_PREFIX_$_-%04d-%s-02.md`", i+2, artifact),
								)
								artifactsExamples = append(
									artifactsExamples,
									fmt.Sprintf("- `_$_MOD_PREFIX_$_-%04d-%s-03.md`", i+3, artifact),
								)
							}
						}
					}
				}

				if len(artifacts) > 0 {
					artifactsInstructions := append([]string{
						"### _$_MOD_TYPE_$_ Artifacts",
						"Pattern: `_$_MOD_PREFIX_$_-${SEQUENCE}-${ARTIFACT}[-{ARTIFACT_SEQUENCE}].md`",
						"",
						fmt.Sprintf("- `${ARTIFACT}`: %s", strings.Join(artifacts, ", ")),
						"- `${ARTIFACT_SEQUENCE}`: Optional 2-digit sequence number",
						"",
						"**Examples:**",
					}, artifactsExamples...)

					prompt = strings.Replace(prompt, "_$_ARTIFACTS_INSTRUCTIONS_$_", strings.Join(artifactsInstructions, "\n"), 1)
				} else {
					prompt = strings.Replace(prompt, "_$_ARTIFACTS_INSTRUCTIONS_$_", "", 1)
				}
				prompt = prompt + "\n\n" + strings.TrimSpace(mod.ExtraPrompt)
				prompt = strings.ReplaceAll(prompt, "_$_MOD_TYPE_$_", strings.ToUpper(mod.Name))
				prompt = strings.ReplaceAll(prompt, "_$_MOD_PREFIX_$_", mod.Prefix)
				mod.Prompt = prompt
			} else {
				prompt = strings.ReplaceAll(prompt, "_$_MOD_TYPE_$_", strings.ToUpper(mod.Name))
				prompt = strings.ReplaceAll(prompt, "_$_MOD_PREFIX_$_", mod.Prefix)
			}

			// Create the destination file on the OS
			dstPromptFile, err := os.Create(dstPromptFileLoc)
			if err != nil {
				srcPromptFile.Close()
				return err
			}

			// stream the content from embed to OS
			_, err = io.WriteString(dstPromptFile, prompt)

			srcPromptFile.Close()
			dstPromptFile.Close()

			if err != nil {
				return err
			}
		}

		// create lanes
		for i, lane := range mod.Lanes {
			if err := os.MkdirAll(lane.DirAbs, 0o755); err != nil {
				return err
			}

			// create lane prompt
			if len(lane.AgentBehavior) > 0 {
				promptFile := strings.ToUpper(lane.Dir) + ".md"
				dstPromptFileLoc := path.Join(modPromptsDir, promptFile)

				promptLines := []string{
					"## Workflow Definition - Lanes",
				}

				for j, olane := range mod.Lanes {
					if j != i {
						promptLines = append(promptLines,
							fmt.Sprintf("### %s", olane.Dir),
							fmt.Sprintf("**Purpose:** %s\n", strings.TrimSpace(olane.Purpose)),
						)
						continue
					}

					promptLines = append(promptLines,
						fmt.Sprintf("### %s", lane.Dir),
						fmt.Sprintf("**Purpose:** %s\n", strings.TrimSpace(lane.Purpose)),
						"**Agent Behavior:**",
					)
					for i, behavior := range lane.AgentBehavior {
						promptLines = append(promptLines, fmt.Sprintf("%d. %s", i+1, strings.TrimSpace(behavior)))
					}
					promptLines = append(promptLines, "")

					if len(lane.CriticalRules) > 0 {
						promptLines = append(promptLines, "**Critical Rules:** ")
						for i, rule := range lane.CriticalRules {
							promptLines = append(promptLines, fmt.Sprintf("%d. %s", i+1, strings.TrimSpace(rule)))
						}
						promptLines = append(promptLines, "")
					}

					promptLines = append(promptLines, strings.TrimSpace(lane.ExtraPrompt))
				}

				prompt := strings.Join(promptLines, "\n")
				lane.Prompt = prompt

				// Create the destination file on the OS
				dstPromptFile, err := os.Create(dstPromptFileLoc)
				if err != nil {
					return err
				}

				// stream the content to OS
				_, err = io.WriteString(dstPromptFile, prompt)
				dstPromptFile.Close()
				if err != nil {
					return err
				}
			}
		}
	}

	return initializeFsys(proj)
}

func initializeFsys(proj *Project) error {
	fsys, err := newFsys()
	if err != nil {
		return err
	}

	proj.fsys = fsys

	laneByDir := map[string]*Lane{}

	now := time.Now()

	for _, mod := range proj.Modules {
		for _, lane := range mod.Lanes {
			if err := fsys.addRecursive(lane.DirAbs); err != nil {
				fsys.Close()
				// func (w *Watcher) Remove(path string) error { return w.b.Remove(path) }
				return err
			}

			laneByDir[lane.DirAbs] = lane

			// initialize lan WorkItems
			if entries, err := os.ReadDir(lane.DirAbs); err == nil {
				for _, entry := range entries {
					info, _ := entry.Info()
					isDir := info != nil && info.IsDir()

					lane.onFsysUpdate(FsysEvent{
						Path:     filepath.Join(lane.DirAbs, entry.Name()),
						Op:       FsysOpCreate,
						Time:     now,
						IsDir:    isDir,
						FileInfo: info,
					})
				}
			}
		}
	}

	go func() {
		defer fsys.Close()
		for ev := range fsys.Events() {
			for dir, lane := range laneByDir {
				if strings.HasPrefix(ev.Path, dir) {
					lane.onFsysUpdate(ev)
					break
				}
			}
		}
	}()

	return nil
}

// validateSchedule validates a lane schedule configuration.
// Returns an error with a clear, actionable message if validation fails.
func validateSchedule(sched *LaneSchedule, moduleName, laneName string) error {
	ctx := fmt.Sprintf("module %s lane %s", moduleName, laneName)

	// Validate frequency
	switch sched.Frequency {
	case ScheduleDaily, ScheduleWeekly, ScheduleMonthly, ScheduleCron:
		// valid
	default:
		return fmt.Errorf("%s: schedule.frequency invalid %q — expected one of: daily, weekly, monthly, cron", ctx, sched.Frequency)
	}

	// Validate cron frequency
	if sched.Frequency == ScheduleCron {
		if sched.CronExpression == "" {
			return fmt.Errorf("%s: schedule.cronExpression is required for cron frequency", ctx)
		}
		// Basic validation: 5-6 space-separated fields
		fields := strings.Fields(sched.CronExpression)
		if len(fields) < 5 || len(fields) > 6 {
			return fmt.Errorf("%s: schedule.cronExpression invalid %q — expected 5-6 space-separated fields", ctx, sched.CronExpression)
		}
		// Reject conflicting fields for cron
		if len(sched.Time) > 0 {
			return fmt.Errorf("%s: schedule.time not allowed for cron frequency", ctx)
		}
		if len(sched.DaysOfWeek) > 0 {
			return fmt.Errorf("%s: schedule.daysOfWeek not allowed for cron frequency", ctx)
		}
		if len(sched.DaysOfMonth) > 0 {
			return fmt.Errorf("%s: schedule.daysOfMonth not allowed for cron frequency", ctx)
		}
		return nil
	}

	// For daily/weekly/monthly: require at least one time entry
	if len(sched.Time) == 0 {
		return fmt.Errorf("%s: schedule.time is required for %s frequency", ctx, sched.Frequency)
	}

	// Validate each time entry
	timeRegex := regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)
	for i, t := range sched.Time {
		if !timeRegex.MatchString(t) {
			return fmt.Errorf("%s: schedule.time[%d] invalid format %q — expected HH:MM", ctx, i, t)
		}
	}

	validDaysOfWeek := map[string]bool{
		"monday": true, "tuesday": true, "wednesday": true,
		"thursday": true, "friday": true, "saturday": true, "sunday": true,
	}

	// Validate weekly frequency
	if sched.Frequency == ScheduleWeekly {
		if len(sched.DaysOfWeek) == 0 {
			return fmt.Errorf("%s: schedule.daysOfWeek is required for weekly frequency", ctx)
		}
		for i, day := range sched.DaysOfWeek {
			if !validDaysOfWeek[strings.ToLower(day)] {
				return fmt.Errorf("%s: schedule.daysOfWeek[%d] invalid day %q — expected Monday-Sunday", ctx, i, day)
			}
		}
		if len(sched.DaysOfMonth) > 0 {
			return fmt.Errorf("%s: schedule.daysOfMonth not allowed for weekly frequency", ctx)
		}
		if sched.CronExpression != "" {
			return fmt.Errorf("%s: schedule.cronExpression not allowed for weekly frequency", ctx)
		}
	}

	// Validate monthly frequency
	if sched.Frequency == ScheduleMonthly {
		if len(sched.DaysOfMonth) == 0 {
			return fmt.Errorf("%s: schedule.daysOfMonth is required for monthly frequency", ctx)
		}
		for i, day := range sched.DaysOfMonth {
			if day < 1 || day > 31 {
				return fmt.Errorf("%s: schedule.daysOfMonth[%d] invalid day %d — expected 1-31", ctx, i, day)
			}
		}
		if len(sched.DaysOfWeek) > 0 {
			return fmt.Errorf("%s: schedule.daysOfWeek not allowed for monthly frequency", ctx)
		}
		if sched.CronExpression != "" {
			return fmt.Errorf("%s: schedule.cronExpression not allowed for monthly frequency", ctx)
		}
	}

	// Validate daily frequency: reject conflicting fields
	if sched.Frequency == ScheduleDaily {
		if len(sched.DaysOfWeek) > 0 {
			return fmt.Errorf("%s: schedule.daysOfWeek not allowed for daily frequency", ctx)
		}
		if len(sched.DaysOfMonth) > 0 {
			return fmt.Errorf("%s: schedule.daysOfMonth not allowed for daily frequency", ctx)
		}
		if sched.CronExpression != "" {
			return fmt.Errorf("%s: schedule.cronExpression not allowed for daily frequency", ctx)
		}
	}

	return nil
}
