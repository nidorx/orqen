package mcp

import (
	"github.com/nidorx/orqen/pkg/project"
)

// findModuleByJobID scans all modules and lanes to find the module
// that contains a work item with the given JobID.
func findModuleByJobID(proj *project.Project, jobID string) *project.Module {
	for _, mod := range proj.Modules {
		for _, lane := range mod.Lanes {
			for _, item := range lane.ListItems() {
				if item.JobID == jobID {
					return mod
				}
			}
		}
	}
	return nil
}
