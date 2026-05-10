package mcp

import (
	"fmt"

	"github.com/nidorx/orqen/pkg/engine"
)

// findTargetModuleBy scans all modules and lanes to find the module
// that contains a work item with the given ID.
func findTargetModuleBy(proj *engine.Project, module *string, workItemID *string) (*engine.Module, error) {
	var targetModule *engine.Module
	if module != nil && *module != "" {
		if targetModule = proj.GetModule(*module); targetModule == nil {
			return nil, fmt.Errorf("module not found: %s", *module)
		} else {
			return targetModule, nil
		}
	}

	// Try to resolve current module from WorkItemID
	if workItemID != nil && *workItemID != "" {
		for _, mod := range proj.Modules {
			for _, lane := range mod.Lanes {
				if item := lane.GetWorkItemByID(*workItemID); item != nil {
					return mod, nil
				}
			}
		}
	}
	if len(proj.Modules) == 1 {
		return proj.Modules[0], nil
	}

	return nil, nil
}
