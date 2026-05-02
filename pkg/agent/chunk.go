package agent

import (
	"strings"
	"time"
)

const chunkPrefixTimeMsg = 5 * time.Second

type Chunk struct {
	logger  Logger
	prefix  string
	started bool
	parts   []string
	lastMsg time.Time
}

func (c *Chunk) add(text string) {
	if !c.started {
		c.started = true
		c.lastMsg = time.Now().Add(-chunkPrefixTimeMsg + 500*time.Millisecond)
		c.parts = append(c.parts, text)
		go func() {
			time.Sleep(1 * time.Second)
			c.check()
		}()
	} else {
		c.parts = append(c.parts, text)
		c.check()
	}
}

func (c *Chunk) stop() {
	c.started = false
	c.check()
}

func (c *Chunk) check() {
	if !c.started {
		if len(c.parts) > 0 {
			if c.prefix != "" {
				c.logger.Log(
					"\033[90m(%s) %s\033[0m\n",
					c.prefix,
					strings.TrimSpace(strings.Join(c.parts, "")),
				)
			} else {
				c.logger.Log(strings.TrimSpace(strings.Join(c.parts, "")) + "\n")
			}
			c.parts = nil
		}
	} else {
		if c.prefix != "" && c.lastMsg.Before(time.Now().Add(-chunkPrefixTimeMsg)) {
			// Thinking ...
			c.logger.Log("\033[90m%s ...\033[0m\n", c.prefix)
			c.lastMsg = time.Now()
		}
	}
}
