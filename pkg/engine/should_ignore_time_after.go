package engine

import "time"

// ignore if recently updated
func shouldIgnoreTimeAfter(item *WorkItem) bool {
	if item.ModTime.After(time.Now().Add(-30 * time.Second)) {
		return true
	}
	return false
}
