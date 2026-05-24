package engine

import (
	"strconv"
	"strings"
)

// shouldIgnoreIfDependency checks if work item dependencies exist in referenced lanes
// This is more specific than HasItemsInReferencedLanes as it checks for specific dependency IDs
func shouldIgnoreIfDependency(item *WorkItem) bool {

	lane := item.Lane

	// Check ignore_if_dependency
	if len(lane.IgnoreIfDependency) <= 0 {
		return false
	}

	// project *Project, currentModule *Module,
	for _, ref := range lane.IgnoreIfDependency {
		refModName, refLaneName, _ := laneParseReference(ref)

		var refModule *Module
		if refModName != "" {
			refModule = lane.Module.Project.GetModule(refModName)
		} else {
			refModule = lane.Module
		}

		if refModule == nil {
			continue
		}

		refLane := refModule.GetLane(refLaneName)
		if refLane == nil {
			continue
		}

		// Check if any of the item's dependencies exist in this lane
		// dependencyParseReference
		for _, dep := range item.Attributes.StringArray("dependencies") {
			depModuleName, depSeq := dependencyParseReference(dep)
			if depSeq <= 0 || (depModuleName != "" && depModuleName != refModName) {
				continue
			}
			if depWorkItem := refModule.GetWorkItemBySeq(depSeq); depWorkItem == nil {
				continue
			} else if depWorkItem.Lane == refLane {
				return true
			}
		}
	}

	return false
}

// dependencyParseReference parses a dependency reference which can be just "SEQUENCE" or "MODULE.SEQUENCE"
func dependencyParseReference(ref string) (moduleName string, workitemSeq int) {

	// "${SEQ}"
	// "${MODULE}.${SEQ}"

	parts := strings.SplitN(ref, ".", 3)
	if len(parts) == 2 {
		seq, err := strconv.Atoi(parts[1])
		if err != nil {
			seq = 0
		}
		return parts[0], seq
	}
	seq, err := strconv.Atoi(ref)
	if err != nil {
		seq = 0
	}
	return "", seq
}
