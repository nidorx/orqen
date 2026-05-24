package engine

import (
	"fmt"
	"io/fs"
	"iter"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nidorx/orqen/pkg/condition"
	"github.com/nidorx/orqen/pkg/utils"
	"github.com/nidorx/orqen/pkg/utils/tinylfu"
)

// Lane defines a task lane within a module.
//
// A lane represents a stage in a workflow pipeline (e.g., inbox, doing, review).
// Each lane has a purpose, optional agent behavior instructions, and conditional
// rules that control when work items should be ignored.
//
// YAML structure:
//
//	lanes:
//	  - name: "doing"                   # lane name (directory created as NN_name automatically)
//	    purpose: "Task being implemented"  # description injected into agent prompt
//	    agent: "qwen"                  # override default agent for this lane (optional)
//	    max_agents: 3                  # max concurrent agents in this lane (0 = unlimited)
//	    artifacts: ["SUMMARY", "FAIL"] # artifact types the agent may create
//	    user_action: "approve"         # short label for expected user action
//	    agent_behavior:                # sequential steps the agent follows (numbered 1., 2., 3...)
//	      - "Read the provided task document"
//	      - "Implement the task according to specifications"
//	    critical_rules:                # absolute rules that must never be ignored
//	      - "Create ALL tasks from the inbox file in this single invocation"
//	    ignore_if_attr: "priority > 3" # ignore if work item attributes match condition
//	    ignore_if_exists: ["draft"]    # ignore if any items exist in referenced lanes
//	    ignore_if_not_exists: ["metricas.md"] # ignore if referenced lanes/files don't exist
//	    ignore_if_dependency: ["doing"] # ignore if work item has dependencies in referenced lanes
//	    extra_prompt: |                # additional context injected after agent_behavior
//	      Upon successful completion, create the SUMMARY artifact...
type Lane struct {
	Dir                string        `yaml:"-"`
	Name               string        `yaml:"name"`
	Agent              string        `yaml:"agent"`
	DirAbs             string        `yaml:"-"`
	Prompt             string        `yaml:"-"`
	Purpose            string        `yaml:"purpose"`
	MaxAgents          int           `yaml:"max_agents"`
	Artifacts          []string      `yaml:"artifacts"`
	UserAction         string        `yaml:"user_action"`
	ExtraPrompt        string        `yaml:"extra_prompt"`
	AgentBehavior      []string      `yaml:"agent_behavior"`
	CriticalRules      []string      `yaml:"critical_rules"`
	IgnoreIfAttr       string        `yaml:"ignore_if_attr"`       // ignore if work item attributes match a condition
	IgnoreIfModtime    int           `yaml:"ignore_if_modtime"`    // ignore if recently updated
	IgnoreIfExists     []string      `yaml:"ignore_if_exists"`     // ignore if items exist in referenced lanes
	IgnoreIfNotExists  []string      `yaml:"ignore_if_not_exists"` // ignore if items/files don't exist in referenced lanes
	IgnoreIfDependency []string      `yaml:"ignore_if_dependency"` // ignore if item has dependencies in referenced lanes
	McpServers         []string      `yaml:"mcpServers"`           // list of MCP server names to inject for this lane
	Module             *Module       `yaml:"-"`                    // reference to parent module
	Hooks              *HookBindings `yaml:"hooks,omitempty"`      // pre/post hook bindings for this lane (can exclude module-level hooks)
	Schedule           *LaneSchedule `yaml:"schedule,omitempty"`   // optional schedule configuration for execution windows

	// Runtime state
	workItemsByID *tinylfu.SyncCacheT[*WorkItem]
}

// GetWorkItemByID returns a work item by its ID, or nil if not found.
func (l *Lane) GetWorkItemByID(workItemID string) *WorkItem {
	if item, exists := l.workItemsByID.Get(workItemID); exists {
		return item
	} else {
		return nil
	}
}

// WorkItems returns an iterator over all work items in this lane.
// https://go.dev/blog/range-functions
func (l *Lane) WorkItems() iter.Seq[*WorkItem] {
	return l.workItemsByID.Values()
}

// FilterWorkItems filters work items from a specific lane using a
// condition DSL string.
// https://go.dev/blog/range-functions
func (l *Lane) FilterWorkItems(cond string) (iter.Seq[*WorkItem], error) {
	node, err := condition.Parse(cond)
	if err != nil {
		return nil, err
	}

	return func(yield func(*WorkItem) bool) {
		for item := range l.WorkItems() {
			if item.Attributes == nil {
				continue
			}
			if condition.Evaluate(node, item.Attributes) {
				if !yield(item) {
					// yield returns false if the loop should stop (e.g., 'break' was called)
					return
				}
			}
		}
	}, nil
}

// HasWorkItems returns true if this lane has work items.
func (l *Lane) HasWorkItems() bool {
	return l.workItemsByID.Len() > 0
}

// CountWorkItems returns the number of work items in this lane.
func (l *Lane) CountWorkItems() int {
	return l.workItemsByID.Len()
}

// CountActiveWorkItems returns the number of items that are currently being processed.
func (l *Lane) CountActiveWorkItems() int {
	count := 0
	for item := range l.workItemsByID.Values() {
		if item.InProgress {
			count++
		}
	}
	return count
}

// HasAvailableSlot checks if this lane can accept more agents.
func (l *Lane) HasAvailableSlot() bool {
	if l.MaxAgents <= 0 {
		return true
	}
	return l.CountActiveWorkItems() < l.MaxAgents
}

// kebabCasePattern validates kebab-case names (lowercase, numbers, hyphens).
var kebabCasePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$|^[a-z0-9]$`)

func (l *Lane) CreateWorkItem(simpleNameP string) (wi *WorkItem, err error) {

	// Validate kebab-case
	simpleName := strings.ToLower(strings.TrimSpace(simpleNameP))
	if !kebabCasePattern.MatchString(simpleName) {
		return nil, fmt.Errorf("simple_name must be kebab-case (lowercase letters, numbers, hyphens): %q", simpleNameP)
	}

	var wiSeq int

	err = l.Module.TxNewWorkItem(func(nextSeq int) (e error) {

		wiDirPath := filepath.Join(l.DirAbs, fmt.Sprintf("%s-%04d-%s", l.Module.Prefix, nextSeq, simpleName))

		defer func() {
			if e != nil {
				_ = os.RemoveAll(wiDirPath)
			}
		}()

		// Create directory
		if err := os.MkdirAll(wiDirPath, 0o755); err != nil {
			e = fmt.Errorf("failed to create directory: %v", err)
			return
		}

		// Create empty .yaml attribute file
		wiAttrPath := filepath.Join(wiDirPath, fmt.Sprintf("%s-%04d.yaml", l.Module.Prefix, nextSeq))
		if err := os.WriteFile(wiAttrPath, []byte{}, 0644); err != nil {
			e = fmt.Errorf("failed to create file: %v", err)
			return
		}

		wiSeq = nextSeq

		return nil
	})

	if err == nil {
		ts := time.Now()
		for {
			time.Sleep(5 * time.Millisecond)
			wi = l.Module.GetWorkItemBySeq(wiSeq)
			if wi != nil || ts.Add(2*time.Second).Before(time.Now()) {
				break
			}
		}
	}

	return
}

// onFsysUpdate método responsável por manter o estado da lane atualizada a partir de mudanças no diretório
func (l *Lane) onFsysUpdate(ev FsysEvent) {

	fileRel, err := filepath.Rel(l.DirAbs, ev.Path)
	if err != nil {
		return
	}

	// WI-001-name, WI-001-name/WI-001-CONTENT.md
	fileSlash := filepath.ToSlash(fileRel)

	var (
		itemName      string // WI-001-name, (inbox: do-something.md, do-something.txt)
		fileExtraPath string // WI-001-CONTENT.md, do-something.md, path/to/internal/folder/something.md
	)

	if i := strings.IndexByte(fileSlash, '/'); i == -1 {
		itemName = fileSlash
	} else {
		itemName, fileExtraPath = fileSlash[:i], fileSlash[i+1:]
	}

	var item *WorkItem

	// Check if this is a work item directory (e.g., WI-001-name, ADR-001-title)
	if l.isWorkItemDir(itemName) {
		seq := l.extractItemSeq(itemName)
		id := utils.HashXxh64([]byte(fmt.Sprintf("%d-%s", seq, itemName)))

		if (ev.Op == FsysOpRemove || ev.Op == FsysOpRename) && fileExtraPath == "" {
			// work item removed, renamed or moved to another lane

			if it, exists := l.workItemsByID.Get(id); exists && it.Lane == l {
				it.Lane = nil
			}
			l.workItemsByID.Del(id)

			// temporarily remove
			l.Module.stash(seq)
			return
		}

		// Reuse existing item if it exists to preserve InProgress state

		if existingLaneItem, exists := l.workItemsByID.Get(id); exists {
			item = existingLaneItem
			item.Lane = l

		} else if existingUnstashedItem := l.Module.unstash(seq); existingUnstashedItem != nil {
			item = existingUnstashedItem
			item.Lane = l

		} else if existingModuleItem := l.Module.GetWorkItemBySeq(seq); existingModuleItem != nil {
			item = existingModuleItem
			item.Lane = l

		} else {
			item = &WorkItem{
				ID:         id,
				Seq:        seq,
				Name:       itemName,
				Files:      []string{},
				Lane:       l,
				Attributes: make(Attributes),
			}
		}

		if ev.Op == FsysOpCreate && ev.IsDir && fileExtraPath == "" {
			// work item created

			var (
				files   = []string{}
				modTime time.Time
			)
			filepath.WalkDir(path.Join(l.DirAbs, itemName), func(path string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return err
				}

				rel, err := filepath.Rel(l.Module.Project.DirAbs, filepath.Clean(path))
				if err != nil {
					return err
				}
				files = append(files, filepath.ToSlash(rel))

				if info, err := d.Info(); err == nil {
					if info.ModTime().After(modTime) {
						modTime = info.ModTime()
					}
				}

				return nil
			})
			item.Files = files
			item.ModTime = modTime
			item.AttributesLoad()

		} else if fileExtraPath != "" {
			// is internal file or sub dir

			if ev.Op == FsysOpRemove || ev.Op == FsysOpRename {
				// file or sub dir removed

				var files = []string{}
				rel, err := filepath.Rel(l.Module.Project.DirAbs, filepath.Clean(filepath.Join(l.DirAbs, itemName, fileExtraPath)))
				if err != nil {
					return
				}
				file := filepath.ToSlash(rel)

				for _, v := range item.Files {
					if !strings.HasPrefix(v, file) {
						files = append(files, v)
					}
				}

				item.Files = files
				item.ModTime = time.Now()

				if fileExtraPath == (itemName + ".yaml") {
					// attributes removed
					item.Attributes = make(Attributes)
				}

			} else if ev.Op == FsysOpCreate && ev.IsDir {
				// sub dir created

				var (
					files   = item.Files
					modTime time.Time
				)
				filepath.WalkDir(path.Join(l.DirAbs, itemName, fileExtraPath), func(path string, d fs.DirEntry, err error) error {
					if err != nil || d.IsDir() {
						return err
					}

					rel, err := filepath.Rel(l.Module.Project.DirAbs, filepath.Clean(path))
					if err != nil {
						return err
					}

					info, err := d.Info()
					if err != nil {
						return err
					}

					if info.ModTime().After(modTime) {
						modTime = info.ModTime()
					}

					files = append(files, filepath.ToSlash(rel))

					return nil
				})

				item.Files = utils.Unique(files)
				item.ModTime = modTime

			} else {
				if ev.Op == FsysOpCreate {
					// file created

					rel, err := filepath.Rel(l.Module.Project.DirAbs, filepath.Clean(filepath.Join(l.DirAbs, itemName, fileExtraPath)))
					if err != nil {
						return
					}
					files := utils.Unique(append(item.Files, filepath.ToSlash(rel)))
					item.Files = files
				}

				if fileExtraPath == (itemName + ".yaml") {
					// attributes created or updated
					item.AttributesLoad()
				}
			}
		}

	} else if l.Name == "inbox" && fileExtraPath == "" {
		// inbox special rules (allow files)

		ext := path.Ext(itemName)
		if ext != ".md" && ext != ".txt" {
			// @TODO: define extension accepted by inbox on orqen.yaml
			return
		}

		id := utils.HashXxh64([]byte(fmt.Sprintf("%d-%s", 0, itemName)))

		// removed, renamed or moved to another lane
		if ev.Op == FsysOpRemove || ev.Op == FsysOpRename {
			if it, exists := l.workItemsByID.Get(id); exists && it.Lane == l {
				it.Lane = nil
			}
			l.workItemsByID.Del(id)
			return
		}

		// Reuse existing item if it exists to preserve InProgress state
		if existingInboxItem, exists := l.workItemsByID.Get(id); exists {
			item = existingInboxItem
			item.Lane = l
		} else {
			rel, err := filepath.Rel(l.Module.Project.DirAbs, filepath.Clean(filepath.Join(l.DirAbs, itemName)))
			if err != nil {
				return
			}

			item = &WorkItem{
				ID:         id,
				Seq:        0,
				Name:       itemName,
				Files:      []string{filepath.ToSlash(rel)}, // single file
				Lane:       l,
				Attributes: make(Attributes),
			}
		}
	}

	if item == nil {
		return
	}

	item.Lane = l

	if ev.FileInfo != nil {
		if modTime := ev.FileInfo.ModTime(); modTime.After(item.ModTime) {
			item.ModTime = modTime
		}
	}

	fmt.Printf(">>> SET workItemsByID  %s - %s", l.Name, item.Name)

	l.workItemsByID.Set(item.ID, item)
	if item.Seq > 0 {
		l.Module.set(item)
	}
}

// isWorkItemDir determines if a directory name represents a work item.
func (l *Lane) isWorkItemDir(name string) bool {
	// Check for common work item patterns: TASK-NNN, ADR-NNN, etc.
	parts := strings.SplitN(name, "-", 2)
	if len(parts) != 2 {
		return false
	}

	// Try to extract sequence number from the second part
	seqStr := ""
	for _, ch := range parts[1] {
		if ch >= '0' && ch <= '9' {
			seqStr += string(ch)
		} else if seqStr != "" {
			break
		}
	}

	if seqStr == "" {
		return false
	}

	_, err := strconv.Atoi(seqStr)
	return err == nil
}

// extractItemSeq extracts the numeric ID from a work item directory name.
func (l *Lane) extractItemSeq(name string) int {
	parts := strings.SplitN(name, "-", 2)
	if len(parts) != 2 {
		return 0
	}

	seqStr := ""
	for _, ch := range parts[1] {
		if ch >= '0' && ch <= '9' {
			seqStr += string(ch)
		} else if seqStr != "" {
			break
		}
	}

	if seqStr == "" {
		return 0
	}

	seq, err := strconv.Atoi(seqStr)
	if err != nil {
		return 0
	}

	return seq
}

// laneParseReference parses a lane reference which can be "lane_name", "module.lane_name", or "file:...".
func laneParseReference(ref string) (moduleName, laneName, filePath string) {

	isFile := strings.HasPrefix(ref, "file:")
	if isFile {
		// "file:file"
		// "file:file.ext"
		// "file:path/to/file.ext"
		// "file:lane_name.file.ext"
		// "file:module.lane_name.file.ext"
		// "file:module.lane_name.path/to/file.ext"
		// "file:module.lane_name.path/to/file.*"
		// "file:module.lane_name.path/to/*.ext"
		// "file:module.lane_name.path/to/*.*"
		// "file:module.lane_name.**/*.*"

		ref = strings.TrimPrefix(ref, "file:")

		parts := strings.SplitN(ref, ".", 3)
		if len(parts) <= 1 {
			// "file:file"
			// "file:file.ext"
			// "file:path/to/file"
			// "file:path/to/file.ext"

			return "", "", ref
		}

		// Check if the first part contains a path separator (e.g., "path/to/file.ext")
		// If so, treat the entire ref as a file path with no lane
		if strings.Contains(parts[0], "/") {
			return "", "", ref
		}

		// If we have exactly 2 parts after SplitN(..., 3), it means there's only one dot
		// This means we have "file.ext" - treat as just a file path
		// "file:file.ext" → "", "", "file.ext"
		if len(parts) == 2 {
			return "", "", ref
		}

		// len(parts) == 3: could be "module.lane_name.file.ext" or "lane_name.file.ext"
		// We need to determine if the first part is a module or a lane
		// Check if parts[1] contains a dot (meaning it's part of a file path like "path/to/file")
		// or if parts[2] exists (meaning we have module.lane.rest)
		//
		// Heuristic: if parts[1] contains "/" or parts[1] + "." + parts[2] looks like a file path,
		// then parts[0] is the lane name
		// Otherwise, parts[0] is module, parts[1] is lane, parts[2] is file
		//
		// Based on the examples, "file:module.lane_name.file.ext" should parse as:
		//   module="module", lane="lane_name", file="file.ext"
		// While "file:lane_name.path/to/file.ext" should parse as:
		//   module="", lane="lane_name", file="path/to/file.ext"
		//
		// The distinguishing factor is whether parts[1] contains "/" (it's a path) or not (it's a lane)

		if strings.Contains(parts[1], "/") {
			// "file:lane_name.path/to/file.ext"
			return "", parts[0], parts[1] + "." + parts[2]
		}

		// Check if parts[2] contains a dot (has a file extension)
		// If yes: "module.lane_name.file.ext" format
		// If no: "lane_name.file" where parts[2] is just an extension fragment
		if strings.Contains(parts[2], ".") {
			// "file:module.lane_name.file.ext"
			return parts[0], parts[1], parts[2]
		}

		// "file:lane_name.file.ext" where SplitN gave us ["lane_name", "file", "ext"]
		return "", parts[0], parts[1] + "." + parts[2]
	} else {
		// "lane_name
		// "module.lane_name"

		parts := strings.SplitN(ref, ".", 3)
		if len(parts) == 2 {
			return parts[0], parts[1], ""
		}
		return "", ref, ""
	}
}

// laneResolvePath resolves a lane reference to an actual Lane object.
func laneResolvePath(project *Project, currentModule *Module, ref string) *Lane {
	moduleName, laneName, _ := laneParseReference(ref)

	var targetModule *Module
	if moduleName != "" {
		targetModule = project.GetModule(moduleName)
	} else {
		targetModule = currentModule
	}

	if targetModule == nil {
		return nil
	}

	return targetModule.GetLane(laneName)
}
