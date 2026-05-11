package chat

import (
	"testing"
	"time"

	"github.com/goccy/go-yaml"
)

func TestChatConfig_Parsing(t *testing.T) {
	data := `
chat:
  agent: "qwen"
  telegram:
    token: "123456:ABC-DEF"
`
	var raw struct {
		Chat ChatConfig `yaml:"chat"`
	}
	if err := yaml.Unmarshal([]byte(data), &raw); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	cfg := raw.Chat
	if cfg.Agent != "qwen" {
		t.Errorf("expected Agent=%q, got %q", "qwen", cfg.Agent)
	}
	if cfg.Telegram.Token != "123456:ABC-DEF" {
		t.Errorf("expected Token=%q, got %q", "123456:ABC-DEF", cfg.Telegram.Token)
	}
}

func TestChatConfig_ZeroValue(t *testing.T) {
	data := `
some_other_key: value
`
	var raw struct {
		Chat ChatConfig `yaml:"chat"`
	}
	if err := yaml.Unmarshal([]byte(data), &raw); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	cfg := raw.Chat
	if cfg.Agent != "" {
		t.Errorf("expected empty Agent, got %q", cfg.Agent)
	}
	if cfg.Telegram.Token != "" {
		t.Errorf("expected empty Token, got %q", cfg.Telegram.Token)
	}
}

func TestChatConfig_PartialConfig(t *testing.T) {
	data := `
chat:
  telegram:
    token: "only-token"
`
	var raw struct {
		Chat ChatConfig `yaml:"chat"`
	}
	if err := yaml.Unmarshal([]byte(data), &raw); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	cfg := raw.Chat
	if cfg.Agent != "" {
		t.Errorf("expected empty Agent, got %q", cfg.Agent)
	}
	if cfg.Telegram.Token != "only-token" {
		t.Errorf("expected Token=%q, got %q", "only-token", cfg.Telegram.Token)
	}
}

func TestMessageRole_Constants(t *testing.T) {
	if string(RoleUser) != "user" {
		t.Errorf("RoleUser = %q, want %q", RoleUser, "user")
	}
	if string(RoleAssistant) != "assistant" {
		t.Errorf("RoleAssistant = %q, want %q", RoleAssistant, "assistant")
	}
	if string(RoleSystem) != "system" {
		t.Errorf("RoleSystem = %q, want %q", RoleSystem, "system")
	}
}

func TestConstants_Values(t *testing.T) {
	if SessionTTL != 24*time.Hour {
		t.Errorf("SessionTTL = %v, want %v", SessionTTL, 24*time.Hour)
	}
	if HistoryLimit != 50 {
		t.Errorf("HistoryLimit = %d, want 50", HistoryLimit)
	}
	if SearchLimit != 10 {
		t.Errorf("SearchLimit = %d, want 10", SearchLimit)
	}
	if MaxMessageLen != 4096 {
		t.Errorf("MaxMessageLen = %d, want 4096", MaxMessageLen)
	}
}

func TestPendingEdit_IsExpired(t *testing.T) {
	now := time.Now()

	// Fresh edit — should not be expired
	fresh := PendingEdit{ID: 1, CreatedAt: now}
	if fresh.IsExpired() {
		t.Error("fresh PendingEdit should not be expired")
	}

	// 1 second before TTL — should not be expired
	before := PendingEdit{ID: 2, CreatedAt: now.Add(-PendingEditTTL + time.Second)}
	if before.IsExpired() {
		t.Error("PendingEdit 1s before TTL should not be expired")
	}

	// Exactly at TTL — edge case; time.Since may be exactly TTL, so not > TTL
	at := PendingEdit{ID: 3, CreatedAt: now.Add(-PendingEditTTL)}
	// At exactly TTL, time.Since == TTL, so IsExpired (which uses >) is false.
	// However, due to timing, we just check it doesn't panic.
	_ = at.IsExpired()

	// 1 second after TTL — should be expired
	after := PendingEdit{ID: 4, CreatedAt: now.Add(-PendingEditTTL - time.Second)}
	if !after.IsExpired() {
		t.Error("PendingEdit 1s after TTL should be expired")
	}

	// Well past TTL — should be expired
	old := PendingEdit{ID: 5, CreatedAt: now.Add(-2 * PendingEditTTL)}
	if !old.IsExpired() {
		t.Error("old PendingEdit should be expired")
	}
}

func TestPendingEditTTL_Constant(t *testing.T) {
	if PendingEditTTL != 10*time.Minute {
		t.Errorf("PendingEditTTL = %v, want %v", PendingEditTTL, 10*time.Minute)
	}
}
