package engine

import (
	"slices"

	"github.com/nidorx/orqen/pkg/utils/glob"
)

// shouldIgnoreIfExists checks if any of the referenced lanes have items
// References can be "lane_name" (same module) or "module.lane_name" (cross-module)
func shouldIgnoreIfExists(item *WorkItem) bool {
	lane := item.Lane

	if len(lane.IgnoreIfExists) <= 0 {
		return false
	}

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

	for _, ref := range lane.IgnoreIfExists {
		moduleName, laneName, filePath := laneParseReference(ref)

		// Same module reference
		targetModule := lane.Module

		if moduleName != "" {
			// Cross-module reference
			targetModule = lane.Module.Project.GetModule(moduleName)
			if targetModule == nil {
				continue
			}
		}

		if laneName == "" {
			laneName = lane.Name
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
				for targetItem := range targetLane.WorkItems() {
					for _, v := range targetItem.Files {
						if regex.Match([]byte(v)) {
							return true
						}
					}
				}
			} else {
				for targetItem := range targetLane.WorkItems() {
					if slices.Contains(targetItem.Files, filePath) {
						return true
					}
				}
			}
		}
	}

	return false
}
