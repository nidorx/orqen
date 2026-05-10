package engine

import (
	"github.com/nidorx/orqen/pkg/condition"
)

// FilterWorkItems filters a slice of WorkItems using a condition DSL string.
//
// The condition is parsed and evaluated against each WorkItem's Attributes.
// Only items whose attributes match the condition are returned.
//
// Example:
//
//	items, err := FilterWorkItems(allItems, "priority > 3 AND type IN ('bug', 'feature')")
func FilterWorkItems(items []*WorkItem, cond string) ([]*WorkItem, error) {
	node, err := condition.Parse(cond)
	if err != nil {
		return nil, err
	}

	var result []*WorkItem
	for _, item := range items {
		if item.Attributes == nil {
			continue
		}
		if condition.Evaluate(node, item.Attributes) {
			result = append(result, item)
		}
	}

	return result, nil
}

// FilterWorkItemsByLane filters work items from a specific lane using a
// condition DSL string.
//
// This is a convenience wrapper around FilterWorkItems that first selects
// items from the given lane.
func FilterWorkItemsByLane(items []*WorkItem, laneName string, cond string) ([]*WorkItem, error) {
	var laneItems []*WorkItem
	for _, item := range items {
		if item.Lane != nil && item.Lane.Name == laneName {
			laneItems = append(laneItems, item)
		}
	}
	return FilterWorkItems(laneItems, cond)
}

// WorkItemMatches checks if a single WorkItem matches a condition DSL string.
// Returns false if the item has no attributes or the condition fails.
func WorkItemMatches(item *WorkItem, cond string) (bool, error) {
	if item.Attributes == nil {
		return false, nil
	}
	return condition.ParseAndEvaluate(cond, item.Attributes)
}
