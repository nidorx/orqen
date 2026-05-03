package mcp

import (
	"github.com/nidorx/orqen/pkg/project"
)

// findModuleByWorkItemID scans all modules and lanes to find the module
// that contains a work item with the given ID.
func findModuleByWorkItemID(proj *project.Project, workItemID string) *project.Module {
	for _, mod := range proj.Modules {
		for _, lane := range mod.Lanes {
			if item := lane.GetWorkItemByID(workItemID); item != nil {
				return mod
			}
		}
	}
	return nil
}
