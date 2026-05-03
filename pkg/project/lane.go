package project

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/nidorx/orqen/pkg/utils"
	"github.com/nidorx/orqen/pkg/utils/glob"
	"github.com/nidorx/orqen/pkg/utils/tinylfu"
)

// Lane defines a task lane within a module.
type Lane struct {
	Dir                string   `yaml:"-"`
	Name               string   `yaml:"name"`
	Agent              string   `yaml:"agent"`
	DirAbs             string   `yaml:"-"`
	Prompt             string   `yaml:"-"`
	Purpose            string   `yaml:"purpose"`
	MaxAgents          int      `yaml:"max_agents"`
	Artifacts          []string `yaml:"artifacts"`
	UserAction         string   `yaml:"user_action"`
	ExtraPrompt        string   `yaml:"extra_prompt"`
	AgentBehavior      []string `yaml:"agent_behavior"`
	CriticalRules      []string `yaml:"critical_rules"`
	IgnoreIfExists     []string `yaml:"ignore_if_exists"`     // ignora se existir uma tarefa ou artefato na lane informada
	IgnoreIfNotExists  []string `yaml:"ignore_if_not_exists"` // ignora se existir uma tarefa ou artefato na lane informada
	IgnoreIfDependency []string `yaml:"ignore_if_dependency"` // ignora se exisitir uma tarefa que é dependencia da atual na lane informad
	Module             *Module  `yaml:"-"`                    // reference to parent module

	// Runtime state
	workItemsByID  *tinylfu.SyncCacheT[*WorkItem]
	workItemsBySeq *tinylfu.SyncCacheT[*WorkItem]

	// file:
	ignoreIfExistsRegexp    []*regexp.Regexp `yaml:"-"`
	ignoreIfNotExistsRegexp []*regexp.Regexp `yaml:"-"`
}

// WorkItems iterator https://go.dev/blog/range-functions
func (l *Lane) WorkItems() func(func(*WorkItem) bool) {
	return l.workItemsByID.Values()
}

// onFsysUpdate método responsável por manter o estado da lane atualizada a partir de mudanças no diretório
func (l *Lane) onFsysUpdate(ev FsysEvent) {

	fileRel, err := filepath.Rel(l.DirAbs, ev.Path)
	if err != nil {
		return
	}

	// TASK-001-name, TASK-001-name/TASK-001-CONTENT.md
	fileSlash := filepath.ToSlash(fileRel)

	var (
		itemName      string // TASK-001-name, (inbox: do-something.md, do-something.txt)
		fileExtraPath string // TASK-001-CONTENT.md, do-something.md, path/to/internal/folder/something.md
	)

	if i := strings.IndexByte(fileSlash, '/'); i == -1 {
		itemName = fileSlash
	} else {
		itemName, fileExtraPath = fileSlash[:i], fileSlash[i+1:]
	}

	var item *WorkItem

	// Check if this is a work item directory (e.g., TASK-001-name, ADR-001-title)
	if l.isWorkItemDir(itemName) {
		seq := l.extractItemSeq(itemName)
		id := utils.HashXxh64([]byte(fmt.Sprintf("%d-%s", seq, itemName)))

		if (ev.Op == FsysOpRemove || ev.Op == FsysOpRename) && fileExtraPath == "" {
			// work item removed, renamed or moved to another lane

			l.workItemsByID.Del(id)
			l.workItemsBySeq.Del(strconv.Itoa(seq))
			return
		}

		if existingItem, exists := l.workItemsByID.Get(id); exists {
			// Reuse existing item if it exists to preserve InProgress state
			item = existingItem
		} else {
			item = &WorkItem{
				ID:    id,
				Seq:   seq,
				Name:  itemName,
				Files: []string{},
				Lane:  l,
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

			} else if ev.Op == FsysOpCreate {
				// file created

				rel, err := filepath.Rel(l.Module.Project.DirAbs, filepath.Clean(filepath.Join(l.DirAbs, itemName, fileExtraPath)))
				if err != nil {
					return
				}
				files := utils.Unique(append(item.Files, filepath.ToSlash(rel)))
				item.Files = files
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
			l.workItemsByID.Del(id)
			return
		}

		// Reuse existing item if it exists to preserve InProgress state
		if existingItem, exists := l.workItemsByID.Get(id); exists {
			item = existingItem
		} else {
			rel, err := filepath.Rel(l.Module.Project.DirAbs, filepath.Clean(filepath.Join(l.DirAbs, itemName)))
			if err != nil {
				return
			}

			item = &WorkItem{
				ID:    id,
				Seq:   0,
				Name:  itemName,
				Files: []string{filepath.ToSlash(rel)}, // single file
				Lane:  l,
			}
		}
	}

	if item == nil {
		return
	}

	if ev.FileInfo != nil {
		if modTime := ev.FileInfo.ModTime(); modTime.After(item.ModTime) {
			item.ModTime = modTime
		}
	}

	l.workItemsByID.Set(item.ID, item)
	if item.Seq > 0 {
		l.workItemsBySeq.Set(strconv.Itoa(item.Seq), item)
	}
}

// FindItemBySeq returns a work item by its ID, or nil if not found
func (l *Lane) GetWorkItemByID(workItemID string) *WorkItem {
	if item, exists := l.workItemsByID.Get(workItemID); exists {
		return item
	} else {
		return nil
	}
}

// GetWorkItemBySeq returns a work item by its sequential number, or nil if not found
func (l *Lane) GetWorkItemBySeq(workItemID int) *WorkItem {
	if item, exists := l.workItemsBySeq.Get(strconv.Itoa(workItemID)); exists {
		return item
	} else {
		return nil
	}
}

// HasWorkItems returns true if this lane has work items
func (l *Lane) HasWorkItems() bool {
	return l.workItemsByID.Len() > 0
}

// CountWorkItems returns the number of work items in this lane
func (l *Lane) CountWorkItems() int {
	return l.workItemsByID.Len()
}

// CountActiveWorkItems returns the number of items that are currently being processed
func (l *Lane) CountActiveWorkItems() int {
	count := 0
	for item := range l.workItemsByID.Values() {
		if item.InProgress {
			count++
		}
	}
	return count
}

// HasAvailableSlot checks if this lane can accept more agents
func (l *Lane) HasAvailableSlot() bool {
	if l.MaxAgents <= 0 {
		return true
	}
	return l.CountActiveWorkItems() < l.MaxAgents
}

// FindItemDependencies scans the item directory for dependency files and populates the Dependencies field
func (l *Lane) FindItemDependencies(item *WorkItem) []*WorkItem {
	if item.Name == "" || l.Module == nil {
		return nil
	}

	itemDir := filepath.Join(l.Dir, item.Name)
	entries, err := os.ReadDir(itemDir)
	if err != nil {
		return nil
	}

	var deps []*WorkItem
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Check for dependency files (e.g., DEP_001, DEP_002)
		if strings.HasPrefix(entry.Name(), "DEP_") {
			depID := l.extractDependencyID(entry.Name())
			if depID > 0 {
				// Find the work item with this ID across all lanes in the module
				depItem := l.Module.GetWorkItemBySeq(depID)
				if depItem != nil {
					deps = append(deps, depItem)
				}
			}
		}
	}

	return deps
}

// extractDependencyID extracts the numeric ID from a dependency file name
func (l *Lane) extractDependencyID(name string) int {
	trimmed := strings.TrimPrefix(name, "DEP_")
	id, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0
	}
	return id
}

// ParseLaneReference parses a lane reference which can be just "lane_name" or "module.lane_name"
func ParseLaneReference(ref string) (moduleName, laneName, filePath string) {

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

// HasItemsInReferencedLanes checks if any of the referenced lanes have items
// References can be "lane_name" (same module) or "module.lane_name" (cross-module)
func (l *Lane) HasItemsInReferencedLanes(refs []string) bool {
	// "lane_name
	// "module.lane_name"
	// "file:file.ext"
	// "file:path/to/file.ext"
	// "file:lane_name.file.ext"
	// "file:module.lane_name.file.ext"
	// "file:module.lane_name.path/to/file.ext"
	// "file:module.lane_name.path/to/file.*"
	// "file:module.lane_name.path/to/*.ext"
	// "file:module.lane_name.path/to/*.*"
	// "file:module.lane_name.**/*.*"
	// "file:adr.draft.artifacts/test.md"

	for _, ref := range refs {
		moduleName, laneName, filePath := ParseLaneReference(ref)

		// Same module reference
		targetModule := l.Module

		if moduleName != "" {
			// Cross-module reference
			targetModule = l.Module.Project.GetModule(moduleName)
			if targetModule == nil {
				continue
			}
		}

		if laneName == "" {
			laneName = l.Name
		}

		targetLane := targetModule.GetLane(laneName)
		if targetLane == nil {
			continue
		}

		if filePath == "" {
			if targetLane.HasWorkItems() {
				return true
			}
		} else {
			if glob.IsGlob(filePath) {
				regex := glob.Cached(filePath)
				for item := range l.workItemsByID.Values() {
					for _, v := range item.Files {
						if regex.Match([]byte(v)) {
							return true
						}
					}
				}
			} else {
				for item := range l.workItemsByID.Values() {
					if slices.Contains(item.Files, filePath) {
						return true
					}
				}
			}
		}
	}

	return false
}

// isWorkItemDir determines if a directory name represents a work item
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

// extractItemSeq extracts the numeric ID from a work item directory name
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

	id, err := strconv.Atoi(seqStr)
	if err != nil {
		return 0
	}

	return id
}

// HasDependencyInReferencedLanes checks if work item dependencies exist in referenced lanes
// This is more specific than HasItemsInReferencedLanes as it checks for specific dependency IDs
func HasDependencyInReferencedLanes(project *Project, currentModule *Module, item *WorkItem, laneRefs []string) bool {
	for _, ref := range laneRefs {
		moduleName, laneName, _ := ParseLaneReference(ref)

		var targetModule *Module
		if moduleName != "" {
			targetModule = project.GetModule(moduleName)
		} else {
			targetModule = currentModule
		}

		if targetModule == nil {
			continue
		}

		targetLane := targetModule.GetLane(laneName)
		if targetLane == nil {
			continue
		}

		// Check if any of the item's dependencies exist in this lane
		for _, dep := range item.Dependencies {
			if targetLane == dep.Lane {
				return true
			}
		}
	}

	return false
}

// ResolveLanePath resolves a lane reference to an actual Lane object
func ResolveLanePath(project *Project, currentModule *Module, ref string) *Lane {
	moduleName, laneName, _ := ParseLaneReference(ref)

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
