package engine

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
