package engine

import (
	"testing"
)

func TestFilterWorkItems_Simple(t *testing.T) {
	items := []*WorkItem{
		{
			Name:       "WI-001",
			Attributes: Attributes{"priority": int64(5), "type": "bug"},
		},
		{
			Name:       "WI-002",
			Attributes: Attributes{"priority": int64(2), "type": "feature"},
		},
		{
			Name:       "WI-003",
			Attributes: Attributes{"priority": int64(8), "type": "bug"},
		},
	}

	result, err := FilterWorkItems(items, "priority > 3")
	if err != nil {
		t.Fatalf("FilterWorkItems error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result))
	}
	if result[0].Name != "WI-001" || result[1].Name != "WI-003" {
		t.Errorf("unexpected items: %v, %v", result[0].Name, result[1].Name)
	}
}

func TestFilterWorkItems_TypeIn(t *testing.T) {
	items := []*WorkItem{
		{
			Name:       "WI-001",
			Attributes: Attributes{"type": "bug"},
		},
		{
			Name:       "WI-002",
			Attributes: Attributes{"type": "feature"},
		},
		{
			Name:       "WI-003",
			Attributes: Attributes{"type": "chore"},
		},
	}

	result, err := FilterWorkItems(items, "type IN ('bug', 'feature')")
	if err != nil {
		t.Fatalf("FilterWorkItems error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result))
	}
}

func TestFilterWorkItems_AndCondition(t *testing.T) {
	items := []*WorkItem{
		{
			Name:       "WI-001",
			Attributes: Attributes{"type": "bug", "priority": int64(5)},
		},
		{
			Name:       "WI-002",
			Attributes: Attributes{"type": "bug", "priority": int64(1)},
		},
		{
			Name:       "WI-003",
			Attributes: Attributes{"type": "feature", "priority": int64(5)},
		},
	}

	result, err := FilterWorkItems(items, "type == 'bug' AND priority > 3")
	if err != nil {
		t.Fatalf("FilterWorkItems error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result))
	}
	if result[0].Name != "WI-001" {
		t.Errorf("expected WI-001, got %s", result[0].Name)
	}
}

func TestFilterWorkItems_NoMatches(t *testing.T) {
	items := []*WorkItem{
		{
			Name:       "WI-001",
			Attributes: Attributes{"type": "chore"},
		},
	}

	result, err := FilterWorkItems(items, "type == 'bug'")
	if err != nil {
		t.Fatalf("FilterWorkItems error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 items, got %d", len(result))
	}
}

func TestFilterWorkItems_NilAttributes(t *testing.T) {
	items := []*WorkItem{
		{Name: "WI-001", Attributes: nil},
		{Name: "WI-002", Attributes: Attributes{"type": "bug"}},
	}

	result, err := FilterWorkItems(items, "type == 'bug'")
	if err != nil {
		t.Fatalf("FilterWorkItems error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 item (nil attrs should be skipped), got %d", len(result))
	}
	if result[0].Name != "WI-002" {
		t.Errorf("expected WI-002, got %s", result[0].Name)
	}
}

func TestFilterWorkItems_ParseError(t *testing.T) {
	items := []*WorkItem{{Name: "WI-001", Attributes: Attributes{}}}

	_, err := FilterWorkItems(items, "type FOO 'bug'")
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestWorkItemMatches(t *testing.T) {
	item := &WorkItem{
		Name:       "WI-001",
		Attributes: Attributes{"priority": int64(5), "type": "bug"},
	}

	match, err := WorkItemMatches(item, "priority > 3 AND type == 'bug'")
	if err != nil {
		t.Fatalf("WorkItemMatches error: %v", err)
	}
	if !match {
		t.Error("expected match")
	}

	match2, err := WorkItemMatches(item, "priority < 3")
	if err != nil {
		t.Fatalf("WorkItemMatches error: %v", err)
	}
	if match2 {
		t.Error("expected no match")
	}
}

func TestWorkItemMatches_NilAttributes(t *testing.T) {
	item := &WorkItem{Name: "WI-001", Attributes: nil}

	match, err := WorkItemMatches(item, "type == 'bug'")
	if err != nil {
		t.Fatalf("WorkItemMatches error: %v", err)
	}
	if match {
		t.Error("expected no match for nil attributes")
	}
}
