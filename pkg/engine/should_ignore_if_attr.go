package engine

import (
	"strings"

	"github.com/nidorx/orqen/pkg/condition"
)

func shouldIgnoreIfAttr(item *WorkItem) bool {
	if cond := strings.TrimSpace(item.Lane.IgnoreIfAttr); cond != "" {
		ignore, _ := condition.ParseAndEvaluate(cond, item.Attributes)
		return ignore
	}
	return false
}
