package engine

import (
	"encoding/json"
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

func (item WorkItem) MarshalJSON() ([]byte, error) {
	// Alias original type so it doesn't have the MarshalJSON method
	type Alias WorkItem

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

	return json.Marshal(&struct {
		Lane         string   `json:"lane"`
		Module       string   `json:"module"`
		Dependencies []string `json:"dependencies"`
		*Alias
	}{
		Lane:         item.Lane.Name,
		Module:       item.Lane.Module.Name,
		Dependencies: deps,
		Alias:        (*Alias)(&item),
	})
}

func (item *WorkItem) AttributesLoad() {
	if item.Seq <= 0 {
		return
	}
	path := filepath.Clean(filepath.Join(item.Lane.DirAbs, item.Name, (item.Name + ".yaml")))
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

	path := filepath.Clean(filepath.Join(item.Lane.DirAbs, item.Name, (item.Name + ".yaml")))
	item.Attributes.Merge(other)
	return item.Attributes.SaveToYAML(path)
}

func (item *WorkItem) AttributesDel(keys []string) error {
	if item.Seq <= 0 || len(keys) == 0 {
		return nil
	}

	path := filepath.Clean(filepath.Join(item.Lane.DirAbs, item.Name, (item.Name + ".yaml")))
	for _, key := range keys {
		if key == "dependencies" {
			continue
		}
		item.Attributes.Delete(key)
	}
	return item.Attributes.SaveToYAML(path)
}

type ShouldIgnore func(item *WorkItem) bool

var shouldIgnoreFns = []ShouldIgnore{
	shouldIgnoreTimeAfter,
	shouldIgnoreIfExists,
	shouldIgnoreIfNotExists,
	shouldIgnoreIfDependency,
	shouldIgnoreIfAttr,
}

// shouldIgnore checks if a work item should be skipped based on ignore rules
func (item *WorkItem) shouldIgnore() bool {
	for _, shouldIgnore := range shouldIgnoreFns {
		if shouldIgnore(item) {
			return true
		}
	}
	return false
}
