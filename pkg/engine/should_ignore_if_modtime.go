package engine

import "time"

// ignore if recently updated
func shouldIgnoreIfModtime(item *WorkItem) bool {
	lane := item.Lane

	if lane.IgnoreIfModtime <= 0 {
		return false
	}

	if item.ModTime.After(time.Now().Add(-(time.Duration(lane.IgnoreIfModtime)) * time.Second)) {
		return true
	}
	return false
}
