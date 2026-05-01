package project

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// Module defines a project module configuration.
type Module struct {
	Name        string   `yaml:"name"`
	Order       []string `yaml:"order"` // ordem para escolha de trabalho
	Lanes       []*Lane  `yaml:"lanes"`
	Prompt      string   `yaml:"-"`
	Dir         string   `yaml:"dir"`
	DirAbs      string   `yaml:"-"`
	Project     *Project `yaml:"-"`
	DirPrompts  string   `yaml:"-"`
	ExtraPrompt string   `yaml:"extra_prompt"`
}

// GetLane returns a lane by name, or nil if not found
func (m *Module) GetLane(name string) *Lane {
	for _, lane := range m.Lanes {
		if lane.Name == name || lane.Dir == name {
			return lane
		}
	}
	return nil
}

// GetLanesInOrder returns lanes ordered by the module's Order field
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

// ListItems returns all work items across all lanes in this module
func (m *Module) ListItems() []*WorkItem {
	var items []*WorkItem
	for _, lane := range m.Lanes {
		items = append(items, lane.ListItems()...)
	}
	return items
}

// FindItemByID finds a work item by its ID across all lanes
func (m *Module) FindItemByID(id int) *WorkItem {
	for _, lane := range m.Lanes {
		if item := lane.GetItem(id); item != nil {
			return item
		}
	}
	return nil
}

// ActiveItemCount returns the total number of items being processed
func (m *Module) ActiveItemCount() int {
	count := 0
	for _, lane := range m.Lanes {
		count += lane.ActiveItemCount()
	}
	return count
}

// HasAvailableSlot checks if the module can accept more agents based on its lanes
func (m *Module) HasAvailableSlot() bool {
	for _, lane := range m.Lanes {
		if lane.HasAvailableSlot() {
			return true
		}
	}
	return false
}

// Initialize sets up the module's lane directories and parent references
func (m *Module) Initialize(projectDir string) error {
	for _, lane := range m.Lanes {
		lane.Module = m
		if lane.Dir != "" {
			lane.Dir = filepath.Join(projectDir, m.Dir, lane.Dir)
		}
	}
	return nil
}

// GetFullDir returns the full path for a lane directory
func (m *Module) GetFullDir(laneName string) string {
	lane := m.GetLane(laneName)
	if lane == nil || lane.Dir == "" {
		return ""
	}
	return lane.Dir
}

func (m *Module) CreateItem() {

}

func (m *Module) NextSequence() int {
	modType := strings.ToUpper(m.Name)
	prefix := fmt.Sprintf("%s-", modType)

	maxSeq := 0

	for _, lane := range m.Lanes {
		items := lane.ListItems()
		for _, item := range items {
			name := item.Name
			if !strings.HasPrefix(name, prefix) {
				continue
			}

			// Extract sequence: TYPE-0001-name → 0001
			rest := strings.TrimPrefix(name, prefix)
			parts := strings.SplitN(rest, "-", 2)
			if len(parts) < 1 {
				continue
			}

			seqStr := strings.TrimLeft(parts[0], "0")
			if seqStr == "" {
				seqStr = "0"
			}

			seq, err := strconv.Atoi(seqStr)
			if err != nil {
				continue
			}

			if seq > maxSeq {
				maxSeq = seq
			}
		}
	}

	return maxSeq + 1
}
