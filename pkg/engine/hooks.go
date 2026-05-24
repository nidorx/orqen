package engine

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// defaultHookTimeout is the default timeout for hook execution (5 minutes).
var defaultHookTimeout = 5 * time.Minute

// ResolvedHook represents a hook with its definition and metadata.
type ResolvedHook struct {
	Name       string
	Definition *HookDefinition
}

// HookExecutor handles the execution of hooks as subprocesses.
type HookExecutor struct {
	workDir string // project root for command execution
}

// NewHookExecutor creates a new hook executor with the given working directory.
func NewHookExecutor(workDir string) *HookExecutor {
	return &HookExecutor{
		workDir: workDir,
	}
}

// Execute runs a hook command as a subprocess with wildcard expansion and timeout.
func (e *HookExecutor) Execute(hookName string, def *HookDefinition, envVars map[string]string) HookResult {
	start := time.Now()

	// Select OS-specific command
	cmdArgs := def.GetCommandForOS(runtime.GOOS)
	if len(cmdArgs) == 0 {
		return HookResult{
			HookName: hookName,
			ExitCode: 1,
			Stderr:   "no command defined for hook",
			Duration: time.Since(start),
			Err:      fmt.Errorf("no command defined for hook %s", hookName),
		}
	}

	// Expand wildcards in command arguments
	expandedArgs := ExpandWildcards(cmdArgs, envVars)

	// Determine timeout
	timeout := def.Timeout
	if timeout == 0 {
		timeout = defaultHookTimeout
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Build command
	cmdName := expandedArgs[0]
	cmdArgs = expandedArgs[1:]
	cmd := exec.CommandContext(ctx, cmdName, cmdArgs...)

	// Set environment variables
	cmd.Env = append(cmd.Env, buildEnvVars(envVars)...)

	// Set working directory
	cmd.Dir = e.workDir

	// Capture stdout and stderr
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	// Execute command
	err := cmd.Run()

	duration := time.Since(start)

	result := HookResult{
		HookName: hookName,
		Stdout:   stdoutBuf.String(),
		Stderr:   stderrBuf.String(),
		Duration: duration,
	}

	// Determine exit code and error
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.Err = &HookTimeoutError{
				HookName: hookName,
				Timeout:  timeout,
			}
			result.ExitCode = -1
		} else if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			result.Err = err
		} else {
			result.ExitCode = -1
			result.Err = err
		}
	}

	return result
}

// buildEnvVars converts a map of environment variables to the format expected by exec.Command.
// It inherits the current process environment and appends the provided vars.
func buildEnvVars(vars map[string]string) []string {
	env := os.Environ()
	for key, value := range vars {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}
	return env
}

// wildcardRegex matches $VAR patterns (letters, digits, underscores).
var wildcardRegex = regexp.MustCompile(`\$[A-Za-z_][A-Za-z0-9_]*`)

// bracedWildcardRegex matches ${VAR} patterns (letters, digits, underscores).
var bracedWildcardRegex = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// ExpandWildcards replaces $VAR and ${VAR} patterns in command arguments with values from the vars map.
// Unknown wildcards are left as-is (no error, pass through).
func ExpandWildcards(args []string, vars map[string]string) []string {
	result := make([]string, len(args))
	for i, arg := range args {
		// First: resolve ${VAR} syntax (braced wildcards)
		resolved := bracedWildcardRegex.ReplaceAllStringFunc(arg, func(match string) string {
			// Extract variable name from ${VAR} (strip ${ and })
			varName := match[2 : len(match)-1]
			if value, exists := vars[varName]; exists {
				return value
			}
			// Unknown wildcard: leave as-is
			return match
		})
		// Then: resolve $VAR syntax (bare wildcards)
		resolved = wildcardRegex.ReplaceAllStringFunc(resolved, func(match string) string {
			// Remove the $ prefix to get the variable name
			varName := match[1:]
			if value, exists := vars[varName]; exists {
				return value
			}
			// Unknown wildcard: leave as-is
			return match
		})
		result[i] = resolved
	}
	return result
}

// ResolveHooks merges module and lane hook bindings, applying exclusions.
// Returns deduplicated lists of resolved hooks ready for execution.
func ResolveHooks(moduleHooks, laneHooks *HookBindings, namedHooks NamedHooks) (preHooks, postHooks []*ResolvedHook) {
	// Track seen hooks to avoid duplicates
	seen := make(map[string]bool)

	// Start with module-level hooks
	if moduleHooks != nil {
		for _, binding := range moduleHooks.Pre {
			if !binding.Negated && namedHooks[binding.Name] != nil {
				if !seen[binding.Name] {
					seen[binding.Name] = true
					preHooks = append(preHooks, &ResolvedHook{
						Name:       binding.Name,
						Definition: namedHooks[binding.Name],
					})
				}
			}
		}
		for _, binding := range moduleHooks.Post {
			if !binding.Negated && namedHooks[binding.Name] != nil {
				if !seen[binding.Name+"_post"] {
					seen[binding.Name+"_post"] = true
					postHooks = append(postHooks, &ResolvedHook{
						Name:       binding.Name,
						Definition: namedHooks[binding.Name],
					})
				}
			}
		}
	}

	// Build exclusion sets from lane hooks
	preExclusions := make(map[string]bool)
	postExclusions := make(map[string]bool)

	if laneHooks != nil {
		for _, binding := range laneHooks.Pre {
			if binding.Negated {
				preExclusions[binding.Name] = true
			}
		}
		for _, binding := range laneHooks.Post {
			if binding.Negated {
				postExclusions[binding.Name] = true
			}
		}
	}

	// Apply lane-level additions (non-negated bindings)
	if laneHooks != nil {
		for _, binding := range laneHooks.Pre {
			if !binding.Negated && !preExclusions[binding.Name] && namedHooks[binding.Name] != nil {
				if !seen[binding.Name] {
					seen[binding.Name] = true
					preHooks = append(preHooks, &ResolvedHook{
						Name:       binding.Name,
						Definition: namedHooks[binding.Name],
					})
				}
			}
		}
		for _, binding := range laneHooks.Post {
			if !binding.Negated && !postExclusions[binding.Name] && namedHooks[binding.Name] != nil {
				if !seen[binding.Name+"_post"] {
					seen[binding.Name+"_post"] = true
					postHooks = append(postHooks, &ResolvedHook{
						Name:       binding.Name,
						Definition: namedHooks[binding.Name],
					})
				}
			}
		}
	}

	// Remove module-level hooks that are excluded by lane
	preHooks = filterExcludedHooks(preHooks, preExclusions, seen)
	postHooks = filterExcludedHooks(postHooks, postExclusions, seen)

	return preHooks, postHooks
}

// filterExcludedHooks removes hooks that are in the exclusions set.
func filterExcludedHooks(hooks []*ResolvedHook, exclusions map[string]bool, seen map[string]bool) []*ResolvedHook {
	var result []*ResolvedHook
	for _, hook := range hooks {
		if !exclusions[hook.Name] {
			result = append(result, hook)
		} else {
			delete(seen, hook.Name)
		}
	}
	return result
}

// CreateHookFailArtifact creates a HOOK-FAIL artifact in the work item directory.
func CreateHookFailArtifact(item *WorkItem, hook *ResolvedHook, result HookResult) error {
	if item.Seq <= 0 {
		return fmt.Errorf("cannot create HOOK-FAIL artifact: item sequence is 0")
	}

	// Determine artifact file path
	itemDir := item.Lane.DirAbs
	itemName := item.Name

	// Check if HOOK-FAIL already exists, add sequence number if needed
	artifactName := fmt.Sprintf("WI-%04d-HOOK-FAIL.md", item.Seq)
	artifactPath := filepath.Join(itemDir, itemName, artifactName)

	// Check for existing HOOK-FAIL files and add sequence number if needed
	seq := 1
	for {
		if _, err := os.Stat(artifactPath); os.IsNotExist(err) {
			break // File doesn't exist, we can use this name
		}
		// File exists, try with sequence number
		seq++
		artifactName = fmt.Sprintf("WI-%04d-HOOK-FAIL-%02d.md", item.Seq, seq)
		artifactPath = filepath.Join(itemDir, itemName, artifactName)
		if seq > 99 {
			return fmt.Errorf("too many HOOK-FAIL artifacts for item %s", item.Name)
		}
	}

	// Build artifact content
	var content strings.Builder
	content.WriteString(fmt.Sprintf("# HOOK-FAIL: %s\n\n", hook.Name))
	content.WriteString(fmt.Sprintf("## Hook Execution Failure\n\n"))
	content.WriteString(fmt.Sprintf("**Hook Name:** %s\n", hook.Name))
	content.WriteString(fmt.Sprintf("**Exit Code:** %d\n", result.ExitCode))
	content.WriteString(fmt.Sprintf("**Duration:** %v\n", result.Duration))
	content.WriteString(fmt.Sprintf("**Timestamp:** %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	content.WriteString("## Reason\n\n")
	content.WriteString("Pre-hook execution failed. Task execution aborted.\n\n")

	if result.Err != nil {
		content.WriteString("## Error\n\n")
		content.WriteString(fmt.Sprintf("```\n%s\n```\n\n", result.Err.Error()))
	}

	if result.Stdout != "" {
		content.WriteString("## Standard Output\n\n")
		content.WriteString(fmt.Sprintf("```\n%s```\n\n", result.Stdout))
	}

	if result.Stderr != "" {
		content.WriteString("## Standard Error\n\n")
		content.WriteString(fmt.Sprintf("```\n%s```\n\n", result.Stderr))
	}

	// Write artifact file
	return os.WriteFile(artifactPath, []byte(content.String()), 0644)
}
