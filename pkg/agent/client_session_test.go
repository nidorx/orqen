package agent

import (
	"context"
	"testing"

	"github.com/coder/acp-go-sdk"
)

func TestPlanEntryCheckbox(t *testing.T) {
	tests := []struct {
		name     string
		status   acp.PlanEntryStatus
		expected string
	}{
		{
			name:     "pending status returns space",
			status:   acp.PlanEntryStatusPending,
			expected: " ",
		},
		{
			name:     "in_progress status returns asterisk",
			status:   acp.PlanEntryStatusInProgress,
			expected: "*",
		},
		{
			name:     "completed status returns x",
			status:   acp.PlanEntryStatusCompleted,
			expected: "x",
		},
		{
			name:     "unknown status returns space",
			status:   acp.PlanEntryStatus("unknown"),
			expected: " ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := planEntryCheckbox(tt.status)
			if result != tt.expected {
				t.Errorf("planEntryCheckbox(%q) = %q, want %q", tt.status, result, tt.expected)
			}
		})
	}
}

func TestClientSession_SessionUpdate_Plan_Empty(t *testing.T) {
	logger := NewLogger("test", "plan-test")
	session := ClientSessionNew(logger, nil)
	ctx := context.Background()

	params := acp.SessionNotification{
		Update: acp.SessionUpdate{
			Plan: &acp.SessionUpdatePlan{
				Entries: []acp.PlanEntry{},
			},
		},
	}

	err := session.SessionUpdate(ctx, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientSession_SessionUpdate_Plan_WithEntries(t *testing.T) {
	logger := NewLogger("test", "plan-test")
	session := ClientSessionNew(logger, nil)
	ctx := context.Background()

	params := acp.SessionNotification{
		Update: acp.SessionUpdate{
			Plan: &acp.SessionUpdatePlan{
				Entries: []acp.PlanEntry{
					{
						Content: "Task 1 - pending",
						Status:  acp.PlanEntryStatusPending,
					},
					{
						Content: "Task 2 - in progress",
						Status:  acp.PlanEntryStatusInProgress,
					},
					{
						Content: "Task 3 - completed",
						Status:  acp.PlanEntryStatusCompleted,
					},
				},
			},
		},
	}

	err := session.SessionUpdate(ctx, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientSession_SessionUpdate_Plan_MultipleCompleted(t *testing.T) {
	logger := NewLogger("test", "plan-test")
	session := ClientSessionNew(logger, nil)
	ctx := context.Background()

	params := acp.SessionNotification{
		Update: acp.SessionUpdate{
			Plan: &acp.SessionUpdatePlan{
				Entries: []acp.PlanEntry{
					{
						Content: "First completed task",
						Status:  acp.PlanEntryStatusCompleted,
					},
					{
						Content: "Second completed task",
						Status:  acp.PlanEntryStatusCompleted,
					},
				},
			},
		},
	}

	err := session.SessionUpdate(ctx, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}