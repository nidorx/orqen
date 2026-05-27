package telegram

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/nidorx/orqen/pkg/chat/conn/telegram/markdown"
	"github.com/nidorx/orqen/pkg/chat/dialog"
)

// Telegram max message length in characters
const maxMessageLength = 4096
const onProgressTimeMsg = 5 * time.Second

var md = markdown.New()

// Telegram holds the Telegram bot configuration.
type Config struct {
	Enabled bool
	Token   string
}

// Connector holds the bot configuration and project references.
// Exported fields are accessed by command handlers (cmd_*.go).
type Connector struct {
	bot    *bot.Bot
	dialog dialog.Handler
}

// New creates a new Telegram Connector instance.
func New(token string) (*Connector, error) {
	tb, err := bot.New(token)
	if err != nil {
		return nil, fmt.Errorf("chat: create telegram connector: %w", err)
	}

	c := &Connector{bot: tb}

	// Register handler for all text messages
	tb.RegisterHandler(bot.HandlerTypeMessageText, "", bot.MatchTypePrefix, c.handleMessage)

	return c, nil
}

func (c *Connector) Set(handler dialog.Handler) {
	c.dialog = handler
}

// Start begins bot polling in a goroutine.
func (c *Connector) Start(ctx context.Context) error {

	// Start bot polling - this blocks, so call in goroutine
	go c.bot.Start(context.Background())
	slog.Info("chat: telegram connector started")
	return nil
}

// Stop shuts down the bot and closes all agent sessions.
func (c *Connector) Stop() {
	// Stop bot
	_, _ = c.bot.Close(context.Background())
	slog.Info("chat: telegram connector stopped")
}

// Send delivers the final agent response to the user via Telegram.
// Splits text if > 4096 chars (Telegram message limit).
// SEND TO USER: delivers final agent response via Telegram
func (c *Connector) Send(ctx context.Context, res *dialog.Response) {

	var buf bytes.Buffer
	_ = md.Convert([]byte(res.Text), &buf)
	txt := buf.String()
	msgId, _ := strconv.Atoi(res.Message.ID)

	chunks := splitMessage(txt, maxMessageLength)
	for _, chunk := range chunks {
		_, err := c.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    res.ChatID,
			Text:      chunk,
			ParseMode: models.ParseModeMarkdown,
			ReplyParameters: &models.ReplyParameters{
				MessageID: msgId,
			},
		})
		if err != nil {
			// return fmt.Errorf("telegram send: %w", err)
		}
	}
}

// OnProgress delivers streaming progress messages to the user.
// SEND TO USER: streaming progress (thinking, tool calls)
// Examples: "🔄 Thinking...", "🔧 Calling tool: read_file..."
func (c *Connector) OnProgress(ctx context.Context, res *dialog.Response) {
	if res.Text == "" || strings.HasSuffix(res.Text, ".") {
		return
	}

	if res.Message != nil {
		if lastMsgV, exist := res.Message.Meta["telegram:lastOnProgress"]; exist {
			if lastMsg, ok := lastMsgV.(time.Time); ok {
				if lastMsg.Before(time.Now().Add(-onProgressTimeMsg)) {
					msgId, _ := strconv.Atoi(res.Message.ID)
					c.bot.SendMessage(ctx, &bot.SendMessageParams{
						ChatID: res.ChatID,
						Text:   "...", // Thinking ...
						ReplyParameters: &models.ReplyParameters{
							MessageID: msgId,
						},
					})
					res.Message.Meta["telegram:lastOnProgress"] = time.Now()
				}
			}
		}
	}
}

// handleMessage is the main entry point for all incoming Telegram messages.
func (c *Connector) handleMessage(ctx context.Context, b *bot.Bot, update *models.Update) {

	var msg *dialog.Message

	if update != nil && update.Message != nil && update.Message.From != nil {
		msg = &dialog.Message{
			ID:     fmt.Sprintf("%d", update.Message.ID),
			ChatID: fmt.Sprintf("%d", update.Message.Chat.ID),
			Text:   update.Message.Text,
			Meta: map[string]any{
				"telegram:lastOnProgress": time.Now().Add(-onProgressTimeMsg + 500*time.Millisecond),
			},
		}
	} else {
		return
	}

	c.dialog(ctx, msg)
}

// splitMessage splits text into chunks of maxLen characters.
// Tries to split on natural boundaries (newlines) to avoid cutting words.
func splitMessage(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}

	var chunks []string
	for len(text) > 0 {
		if len(text) <= maxLen {
			chunks = append(chunks, text)
			break
		}

		// Try to split on last newline within maxLen
		splitAt := maxLen
		for i := maxLen - 1; i >= 0; i-- {
			if text[i] == '\n' {
				splitAt = i + 1
				break
			}
		}

		// If no newline found, hard cut at maxLen
		if splitAt == 0 {
			splitAt = maxLen
		}

		chunks = append(chunks, text[:splitAt])
		text = text[splitAt:]
	}

	return chunks
}
