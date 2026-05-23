package engine

import (
	"fmt"
	"iter"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"sync"

	"github.com/nidorx/orqen/pkg/utils/tinylfu"
)

const schemaMaxValues = 50 // max unique values per field in schema

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
	Hooks       *HookBindings `yaml:"hooks,omitempty"` // pre/post hook bindings for this module

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
		item.Lane = nil
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

// WorkItems returns all work items across all lanes in this module.
func (m *Module) WorkItems() iter.Seq[*WorkItem] {
	return func(yield func(*WorkItem) bool) {
		for _, lane := range m.Lanes {
			for item := range lane.WorkItems() {
				if !yield(item) {
					// yield returns false if the loop should stop (e.g., 'break' was called)
					return
				}
			}
		}
	}
}

// FilterWorkItems filters work items from a module using a
// condition DSL string.
func (m *Module) FilterWorkItems(cond string) (iter.Seq[*WorkItem], error) {

	var iterators []iter.Seq[*WorkItem]

	for _, lane := range m.Lanes {
		if iterator, err := lane.FilterWorkItems(cond); err != nil {
			return nil, err
		} else {
			iterators = append(iterators, iterator)
		}
	}

	return func(yield func(*WorkItem) bool) {
		for _, items := range iterators {
			for item := range items {
				if !yield(item) {
					// yield returns false if the loop should stop (e.g., 'break' was called)
					return
				}
			}
		}
	}, nil
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

	// Collect max sequence number from all lanes.
	// Values() snapshots keys under lock and iterates without holding it,
	// so this won't block concurrent cache operations.
	maxSeq := 0
	for _, lane := range m.Lanes {
		for item := range lane.WorkItems() {
			if item.Seq > maxSeq {
				maxSeq = item.Seq
			}
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Re-check maxSeq under lock in case another goroutine incremented it
	// between our scan and the lock acquisition.
	for _, lane := range m.Lanes {
		for item := range lane.WorkItems() {
			if item.Seq > maxSeq {
				maxSeq = item.Seq
			}
		}
	}

	return fn(maxSeq + 1)
}

// Schema scans a module directory and returns all observed front matter
// fields with their types and unique values (domains).
func (m *Module) Schema() []SchemaField {
	fieldData := make(map[string]*SchemaField)
	fieldValues := make(map[string]map[string]bool) // track unique values

	for item := range m.WorkItems() {
		for field, val := range item.Attributes {
			if _, exists := fieldData[field]; !exists {
				fieldData[field] = &SchemaField{Field: field}
				fieldValues[field] = map[string]bool{}
			}

			sd := fieldData[field]
			typeName := yamlTypeName(val)
			if !slices.Contains(sd.Types, typeName) {
				sd.Types = append(sd.Types, typeName)
			}

			valKey := fmt.Sprintf("%v", val)
			if !fieldValues[field][valKey] && len(sd.Values) < schemaMaxValues {
				fieldValues[field][valKey] = true
				sd.Values = append(sd.Values, val)
			}
		}
	}

	var fields []SchemaField
	for _, f := range fieldData {
		fields = append(fields, *f)
	}

	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Field < fields[j].Field
	})

	return fields
}

// yamlTypeName returns the YAML type name for a value.
func yamlTypeName(val any) string {
	switch val.(type) {
	case bool:
		return "bool"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return "int"
	case float32, float64:
		return "float"
	case string:
		return "string"
	case []any:
		return "list"
	case map[string]any:
		return "map"
	default:
		return "unknown"
	}
}