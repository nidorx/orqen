package project

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
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
	IgnoreIfExists     []string `yaml:"ignore_if_exists"`     // ignora se existir uma tarefa na lane informada
	IgnoreIfDependency []string `yaml:"ignore_if_dependency"` // ignora se exisitir uma tarefa que é dependencia da atual na lane informad
	Module             *Module  `yaml:"-"`                    // reference to parent module

	// Runtime state
	items      []*WorkItem // cached work items
	itemsMutex sync.Mutex  // mutex for thread-safe access
}

// ListItems scans the lane directory and returns work items found in this lane
func (l *Lane) ListItems() []*WorkItem {

	entries, err := os.ReadDir(l.DirAbs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return nil
	}

	l.itemsMutex.Lock()
	defer l.itemsMutex.Unlock()

	// Build a map of existing items by ID for preserving InProgress state
	existingItems := make(map[string]*WorkItem)
	for _, item := range l.items {
		existingItems[fmt.Sprintf("%d-%s", item.ID, item.Name)] = item
	}

	var modTime time.Time
	var items []*WorkItem
	for _, entry := range entries {
		name := entry.Name()

		// regra especial para inbox
		if l.Name == "inbox" {
			ext := path.Ext(name)
			if ext == ".md" || ext == ".txt" {
				info, err := entry.Info()
				if err != nil {
					fmt.Printf("%s", err.Error())
					continue
				}

				if info.Size() == 0 {
					continue
				}

				// // só inicia item se foi modificado a mais de 1 minuto
				// if info.ModTime().After(time.Now().Add(-60 * time.Second)) {
				// 	continue
				// }

				if info.ModTime().After(modTime) {
					modTime = info.ModTime()
				}

				rel, err := filepath.Rel(l.Module.Project.DirAbs, filepath.Clean(path.Join(l.DirAbs, name)))
				if err != nil {
					fmt.Printf("%s", err.Error())
					continue
				}

				// single file
				files := []string{filepath.ToSlash(rel)}

				// Reuse existing item if it exists to preserve InProgress state
				if existingItem, ok := existingItems[fmt.Sprintf("%d-%s", 0, name)]; ok {
					existingItem.Name = name
					existingItem.Lane = l
					existingItem.Files = files
					existingItem.ModTime = modTime
					items = append(items, existingItem)
				} else {
					items = append(items, &WorkItem{
						ID:      0,
						Name:    name,
						Files:   files,
						ModTime: modTime,
						Lane:    l,
					})
				}
			}
			continue
		}

		if !entry.IsDir() {
			continue
		}

		// Check if this is a work item directory (e.g., TASK-001-name, ADR-001-title)
		if l.isWorkItemDir(name) {
			id := l.extractItemID(name)

			var files []string

			filepath.WalkDir(path.Join(l.DirAbs, name), func(path string, d fs.DirEntry, err error) error {
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

			// Reuse existing item if it exists to preserve InProgress state
			if existingItem, ok := existingItems[fmt.Sprintf("%d-%s", id, name)]; ok {
				existingItem.Name = name
				existingItem.Lane = l
				existingItem.Files = files
				existingItem.ModTime = modTime
				items = append(items, existingItem)
			} else {
				items = append(items, &WorkItem{
					ID:      id,
					Name:    name,
					Files:   files,
					ModTime: modTime,
					Lane:    l,
				})
			}
		}
	}

	// Update cache
	l.items = items
	return items
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

// extractItemID extracts the numeric ID from a work item directory name
func (l *Lane) extractItemID(name string) int {
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

// HasItems returns true if this lane has work items
func (l *Lane) HasItems() bool {
	return len(l.ListItems()) > 0
}

// ItemCount returns the number of work items in this lane
func (l *Lane) ItemCount() int {
	return len(l.ListItems())
}

// GetItem returns a work item by its ID, or nil if not found
func (l *Lane) GetItem(id int) *WorkItem {
	items := l.ListItems()
	for _, item := range items {
		if item.ID == id {
			return item
		}
	}
	return nil
}

// ActiveItemCount returns the number of items that are currently being processed
func (l *Lane) ActiveItemCount() int {
	items := l.ListItems()
	count := 0
	for _, item := range items {
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
	return l.ActiveItemCount() < l.MaxAgents
}

// ParseLaneReference parses a lane reference which can be just "lane_name" or "module.lane_name"
func ParseLaneReference(ref string) (moduleName, laneName string) {
	parts := strings.SplitN(ref, ".", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", ref
}

// HasItemsInReferencedLanes checks if any of the referenced lanes have items
// References can be "lane_name" (same module) or "module.lane_name" (cross-module)
func HasItemsInReferencedLanes(project *Project, currentModule *Module, laneRefs []string) bool {
	for _, ref := range laneRefs {
		moduleName, laneName := ParseLaneReference(ref)

		var targetModule *Module
		if moduleName != "" {
			// Cross-module reference
			targetModule = project.GetModule(moduleName)
		} else {
			// Same module reference
			targetModule = currentModule
		}

		if targetModule == nil {
			continue
		}

		targetLane := targetModule.GetLane(laneName)
		if targetLane == nil {
			continue
		}

		if targetLane.HasItems() {
			return true
		}
	}

	return false
}

// HasDependencyInReferencedLanes checks if work item dependencies exist in referenced lanes
// This is more specific than HasItemsInReferencedLanes as it checks for specific dependency IDs
func HasDependencyInReferencedLanes(project *Project, currentModule *Module, item *WorkItem, laneRefs []string) bool {
	for _, ref := range laneRefs {
		moduleName, laneName := ParseLaneReference(ref)

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
	moduleName, laneName := ParseLaneReference(ref)

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
				depItem := l.Module.FindItemByID(depID)
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
