package engine

import (
	"path/filepath"
	"strconv"
	"sync"

	"github.com/nidorx/orqen/pkg/utils/tinylfu"
)

// Module represents a project module configuration (e.g., task, adr, learning).
//
// A module groups related work items into lanes within a dedicated directory.
// Each module has its own set of lanes that define the workflow stages.
//
// YAML structure:
//
//	modules:
//	  - name: task                    # module name (unique within project)
//	    dir: "tasks"                  # directory relative to project root where work items are stored
//	    prefix: "TASK"                # prefix for work item names (default: "WI")
//	    order: ["doing", "inbox"]     # priority order for lane selection when choosing work
//	    extra_prompt: |               # additional context injected into module's HEADER.md
//	      **Consult ADRs:** Before refining...
//	    lanes:                        # list of lanes in this module
//	      - name: "inbox"
//	        purpose: "..."
type Module struct {
	Name        string   `yaml:"name"`
	Order       []string `yaml:"order"`  // priority order for lane selection when choosing work
	Prefix      string   `yaml:"prefix"` // prefix for work item names (e.g., TASK, ADR, LKN)
	Lanes       []*Lane  `yaml:"lanes"`
	Prompt      string   `yaml:"-"`
	Dir         string   `yaml:"dir"`
	DirAbs      string   `yaml:"-"`
	Project     *Project `yaml:"-"`
	DirPrompts  string   `yaml:"-"`
	ExtraPrompt string   `yaml:"extra_prompt"`

	mu               sync.Mutex
	workItemsBySeq   *tinylfu.SyncCacheT[*WorkItem]
	workItemsStashed *tinylfu.SyncCacheT[*WorkItem]
}

func (m *Module) set(item *WorkItem) {
	if item.Seq > 0 {
		m.workItemsBySeq.Set(strconv.Itoa(item.Seq), item)
	}
}

func (m *Module) stash(seq int) {
	if item, exists := m.workItemsBySeq.Get(strconv.Itoa(seq)); exists {
		m.workItemsStashed.Set(strconv.Itoa(seq), item)
		m.workItemsBySeq.Del(strconv.Itoa(seq))
	}
}

func (m *Module) unstash(seq int) *WorkItem {
	if item, exists := m.workItemsStashed.Get(strconv.Itoa(seq)); exists {
		m.workItemsBySeq.Set(strconv.Itoa(seq), item)
		m.workItemsStashed.Del(strconv.Itoa(seq))
		return item
	}
	return nil
}

// GetLane returns a lane by name, or nil if not found.
func (m *Module) GetLane(name string) *Lane {
	for _, lane := range m.Lanes {
		if lane.Name == name || lane.Dir == name {
			return lane
		}
	}
	return nil
}

// GetLanesInOrder returns lanes ordered by the module's Order field.
func (m *Module) GetLanesInOrder() []*Lane {
	if len(m.Order) == 0 {
		return m.Lanes
	}

	var ordered []*Lane
	for _, orderName := range m.Order {
		if lane := m.GetLane(orderName); lane != nil {
			ordered = append(ordered, lane)
		}
	}

	// Add any lanes not in the Order list
	for _, lane := range m.Lanes {
		found := false
		for _, o := range ordered {
			if o.Name == lane.Name {
				found = true
				break
			}
		}
		if !found {
			ordered = append(ordered, lane)
		}
	}

	return ordered
}

// ListWorkItems returns all work items across all lanes in this module.
func (m *Module) ListWorkItems() []*WorkItem {
	var items []*WorkItem
	for _, lane := range m.Lanes {
		for item := range lane.WorkItems() {
			items = append(items, item)
		}
	}
	return items
}

// GetWorkItemBySeq finds a work item by its sequential id across all lanes.
func (m *Module) GetWorkItemBySeq(seq int) *WorkItem {
	if item, exists := m.workItemsBySeq.Get(strconv.Itoa(seq)); exists {
		return item
	} else {
		return nil
	}
}

// GetWorkItemById finds a work item by its ID across all lanes.
func (m *Module) GetWorkItemById(workItemID string) *WorkItem {
	for _, lane := range m.Lanes {
		if item := lane.GetWorkItemByID(workItemID); item != nil {
			return item
		}
	}
	return nil
}

// ActiveItemCount returns the total number of items being processed.
func (m *Module) ActiveItemCount() int {
	count := 0
	for _, lane := range m.Lanes {
		count += lane.CountActiveWorkItems()
	}
	return count
}

// HasAvailableSlot checks if the module can accept more agents based on its lanes.
func (m *Module) HasAvailableSlot() bool {
	for _, lane := range m.Lanes {
		if lane.HasAvailableSlot() {
			return true
		}
	}
	return false
}

// Initialize sets up the module's lane directories and parent references.
func (m *Module) Initialize(projectDir string) error {
	for _, lane := range m.Lanes {
		lane.Module = m
		if lane.Dir != "" {
			lane.Dir = filepath.Join(projectDir, m.Dir, lane.Dir)
		}
	}
	return nil
}

// GetFullDir returns the full path for a lane directory.
func (m *Module) GetFullDir(laneName string) string {
	lane := m.GetLane(laneName)
	if lane == nil || lane.Dir == "" {
		return ""
	}
	return lane.Dir
}

func (m *Module) CreateItem() {

}

func (m *Module) TxNewWorkItem(fn func(nextSeq int) error) error {

	m.mu.Lock()
	defer m.mu.Unlock()

	maxSeq := 0

	for _, lane := range m.Lanes {
		for item := range lane.WorkItems() {
			if item.Seq > maxSeq {
				maxSeq = item.Seq
			}
		}
	}

	return fn(maxSeq + 1)
}
