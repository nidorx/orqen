package engine

import (
	"slices"

	"github.com/nidorx/orqen/pkg/utils/glob"
)

// shouldIgnoreIfNotExists checks if any of the referenced lanes DOES NOT have items
// References can be "lane_name" (same module) or "module.lane_name" (cross-module)
func shouldIgnoreIfNotExists(item *WorkItem) bool {
	lane := item.Lane

	if len(lane.IgnoreIfNotExists) <= 0 {
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

	for _, ref := range lane.IgnoreIfNotExists {
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
			if !targetLane.HasWorkItems() {
				return true
			}
		} else {
			if glob.IsGlob(filePath) {
				regex := glob.Cached(filePath)
				has := false
				for item := range lane.workItemsByID.Values() {
					for _, v := range item.Files {
						if regex.Match([]byte(v)) {
							has = true
							break
						}
					}
					if has {
						break
					}
				}
				if !has {
					return true
				}
			} else {
				has := false
				for item := range lane.workItemsByID.Values() {
					if slices.Contains(item.Files, filePath) {
						has = true
						break
					}
				}
				if !has {
					return true
				}
			}
		}
	}

	return false
}
