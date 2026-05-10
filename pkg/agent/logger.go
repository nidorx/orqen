package agent

import (
	"fmt"
	"sync/atomic"
)

type Logger string

var (
	idx uint64
	// prefix is the visual indicator prepended to every message, giving the
	// impression of a charismatic AI assistant.
	colors = []string{
		"\033[31m[•_•] ", // Red
		"\033[32m[•_•] ", // Green
		"\033[33m[•_•] ", // Yellow
		"\033[34m[•_•] ", // Blue
		"\033[35m[•_•] ", // Purple
		"\033[91m[•_•] ", // Red 	- High Intensity
		"\033[92m[•_•] ", // Green 	- High Intensity
		"\033[93m[•_•] ", // Yellow - High Intensity
		"\033[94m[•_•] ", // Blue 	- High Intensity
		"\033[95m[•_•] ", // Purple - High Intensity
	}
)

func nextColor() string {
	i := atomic.AddUint64(&idx, 1)
	return colors[i%uint64(len(colors))]
}

func newLogger(agent, details string) Logger {
	// [•_•] [claude] [inbox] [create-new-task]
	// [•_•] [qwen] [review] [WI-0001-add-new-agent] : tool call -
	return Logger(nextColor() + fmt.Sprintf("[%s]\033[90m%s - \033[0m", agent, details))
}

func (l Logger) Log(format string, args ...any) {
	fmt.Printf(string(l)+format, args...)
}
