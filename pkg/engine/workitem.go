package engine

import (
	"errors"
	"fmt"
	"iter"
	"os"
	"path/filepath"
	"time"
)

// WorkItem representa uma tarefa que está disponível em uma Lane
type WorkItem struct {
	ID         string     `json:"id,omitempty" jsonschema:"unique identifier for this work item hash(Seq+Name)"`
	Seq        int        `json:"seq,omitempty" jsonschema:"unique sequential id for this work item"`
	Name       string     `json:"name,omitempty" jsonschema:"directory/file name (e.g., WI-001-create-project)"`
	Files      []string   `json:"files,omitempty" jsonschema:"files in directory (e.g., WI-001.md, WI-001-SUmMARY.md)"`
	Lane       *Lane      `json:"-" jsonschema:"the lane this item belongs to"`
	InProgress bool       `json:"in_progress,omitempty" jsonschema:"indica que um agente está executando a tarefa"`
	Attributes Attributes `json:"attributes" jsonschema:"todos os atributos do WorkItem"`
	ModTime    time.Time  `json:"mod_time" jsonschema:"atualização mais recente do item"`

	attributesModTime time.Time `json:"-"`
}

type WorkItemAlias struct {
	*WorkItem
	Lane         string   `json:"lane"`
	Module       string   `json:"module"`
	Dependencies []string `json:"dependencies"`
}

func (item WorkItem) Alias() *WorkItemAlias {
	// DirPath    string            `json:"dir_path"`
	// FilePath   string            `json:"file_path"`
	// ModuleType string            `json:"module_type"`
	// // Relative paths
	// relDir, _ := filepath.Rel(proj.DirAbs, filepath.Clean(dirPath))
	// relFile, _ := filepath.Rel(proj.DirAbs, filepath.Clean(filePath))
	// out.DirPath = filepath.ToSlash(filepath.Rel(proj.DirAbs, filepath.Clean(dirPath)))
	// out.FilePath = filepath.ToSlash(relFile)
	var deps []string
	for _, v := range item.Attributes.StringArray("dependencies") {
		deps = append(deps, v)
	}
	return &WorkItemAlias{
		Lane:         item.Lane.Name,
		Module:       item.Lane.Module.Name,
		Dependencies: deps,
		WorkItem:     &item,
	}
}

// RelativePath returns the work item's relative path within the module (e.g., "04_prioritized/WI-0002-hooks-execution-engine").
func (item *WorkItem) RelativePath() string {
	return filepath.Join(item.Lane.Dir, item.Name)
}

func (item *WorkItem) AttributesLoad() {
	if item.Seq <= 0 {
		return
	}
	path := filepath.Clean(filepath.Join(item.Lane.DirAbs, item.Name, fmt.Sprintf("%s-%04d.yaml", item.Lane.Module.Prefix, item.Seq)))
	info, _ := os.Stat(path)
	if info != nil && item.attributesModTime.Before(info.ModTime()) {
		item.Attributes.LoadFromYAML(path)
		item.attributesModTime = info.ModTime()
	}
}

func (item *WorkItem) AttributesSave(other Attributes) error {
	if item.Seq <= 0 || len(other) == 0 {
		return nil
	}

	item.Attributes.Merge(other)
	path := filepath.Clean(filepath.Join(item.Lane.DirAbs, item.Name, fmt.Sprintf("%s-%04d.yaml", item.Lane.Module.Prefix, item.Seq)))
	return item.Attributes.SaveToYAML(path)
}

func (item *WorkItem) AttributesDel(keys []string) error {
	if item.Seq <= 0 || len(keys) == 0 {
		return nil
	}

	for _, key := range keys {
		if key == "dependencies" {
			continue
		}
		item.Attributes.Delete(key)
	}

	path := filepath.Clean(filepath.Join(item.Lane.DirAbs, item.Name, fmt.Sprintf("%s-%04d.yaml", item.Lane.Module.Prefix, item.Seq)))
	return item.Attributes.SaveToYAML(path)
}

func (item *WorkItem) Dependencies() iter.Seq[*WorkItem] {
	return func(yield func(*WorkItem) bool) {
		for _, dep := range item.Attributes.StringArray("dependencies") {
			depModuleName, depSeq := dependencyParseReference(dep)
			if depSeq <= 0 {
				continue
			}

			var refModule *Module
			if depModuleName == "" {
				refModule = item.Lane.Module
			} else {
				refModule = item.Lane.Module.Project.GetModule(depModuleName)
			}
			if refModule == nil {
				continue
			}

			if depWorkItem := refModule.GetWorkItemBySeq(depSeq); depWorkItem == nil {
				continue
			} else if depWorkItem != item {
				if !yield(depWorkItem) {
					// yield returns false if the loop should stop (e.g., 'break' was called)
					return
				}
			}
		}
	}
}

func (item *WorkItem) Dependents() iter.Seq[*WorkItem] {
	return func(yield func(*WorkItem) bool) {
		if item.Seq <= 0 {
			return
		}

		for other := range item.Lane.Module.Project.WorkItems() {
			if other == item {
				continue
			}

			for _, dep := range other.Attributes.StringArray("dependencies") {
				depModuleName, depSeq := dependencyParseReference(dep)
				if depSeq <= 0 {
					continue
				}

				var refModule *Module
				if depModuleName == "" {
					refModule = other.Lane.Module
				} else {
					refModule = other.Lane.Module.Project.GetModule(depModuleName)
				}
				if refModule == nil || refModule != item.Lane.Module || depSeq != item.Seq {
					continue
				}

				if !yield(other) {
					// yield returns false if the loop should stop (e.g., 'break' was called)
					return
				}
			}
		}
	}
}

func (item *WorkItem) MoveTo(laneName string) error {
	if item.Seq <= 0 {
		return nil
	}

	fromLane := item.Lane

	toLane := item.Lane.Module.GetLane(laneName)
	if toLane == nil {
		return errors.New("to_lane not found: " + laneName)
	}

	if fromLane == toLane {
		return nil
	}

	// Build source and destination paths
	srcDir := filepath.Join(fromLane.DirAbs, item.Name)
	dstDir := filepath.Join(toLane.DirAbs, item.Name)

	// Move the directory
	if err := os.Rename(srcDir, dstDir); err != nil {
		return fmt.Errorf("failed to move item directory: %v", err)
	}

	ts := time.Now()
	for {
		time.Sleep(50 * time.Millisecond)
		if item.Lane != fromLane || ts.Add(2*time.Second).Before(time.Now()) {
			break
		}
	}

	return nil
}
