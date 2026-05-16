package dialog

import (
	"context"
	"time"

	"github.com/nidorx/orqen/pkg/chat/memory"
)

type Message struct {
	ID     string         `json:"message_id"`
	ChatID string         `json:"chat_id"`
	Text   string         `json:"text,omitempty"`
	Time   time.Time      `json:"time"`
	Meta   map[string]any `json:"meta"`

	session *memory.ChatSession
	channel *channel `json:"-"`
	// Audio                         *Audio                         `json:"audio,omitempty"`
	// Document                      *Document                      `json:"document,omitempty"`
	// MessageThreadID int   `json:"message_thread_id,omitempty"`
	// DirectMessagesTopic           *DirectMessagesTopic           `json:"direct_messages_topic,omitempty"`
	// From                          *User                          `json:"from,omitempty"`
}

type Response struct {
	Text    string    `json:"text"`
	ChatID  string    `json:"chat_id"`
	Message *Message  `json:"message"`
	Time    time.Time `json:"time"`
}

type channel struct {
	msg       *Message
	connector Connector
}

func (m *channel) Send(ctx context.Context, res *Response) {
	defer func() {
		if res.Message != nil {
			res.Message.channel = nil
			res.Message.session = nil
		}
	}()

	res.Time = time.Now()
	res.ChatID = m.msg.ChatID
	res.Message = m.msg

	m.connector.Send(ctx, res)
}

func (m *channel) OnProgress(ctx context.Context, res *Response) {
	res.Message = m.msg
	res.ChatID = m.msg.ChatID
	m.connector.OnProgress(ctx, res)
}

type agentChannel struct {
	channel *channel
}

func (c *agentChannel) Send(ctx context.Context, text string) {
	c.channel.Send(ctx, &Response{Text: text})
}

func (c *agentChannel) OnProgress(ctx context.Context, text string) {
	c.channel.OnProgress(ctx, &Response{Text: text})
}
