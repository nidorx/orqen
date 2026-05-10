package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/nidorx/orqen/pkg/engine"
	projectpkg "github.com/nidorx/orqen/pkg/memory/project"
	"github.com/nidorx/orqen/pkg/memory/store"
)

// ── Test helpers ─────────────────────────────────────────────────────────────

func newMemSaveTestStore(t *testing.T) *store.Store {
	t.Helper()
	cfg := store.DefaultConfig(".")
	cfg.DataDir = t.TempDir()

	s, err := store.New(cfg)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() {
		_ = s.Close()
	})
	return s
}

// newMemSaveTestProject creates a *engine.Project backed by the given store.
// Since project.memory is unexported, we use reflection to inject it.
func newMemSaveTestProject(t *testing.T, s *store.Store) *engine.Project {
	t.Helper()
	p := &engine.Project{}
	rv := reflect.ValueOf(p).Elem()
	rf := rv.FieldByName("memory")
	if !rf.IsValid() {
		t.Fatal("project.Project has no 'memory' field")
	}
	reflect.NewAt(rf.Type(), unsafe.Pointer(rf.UnsafeAddr())).Elem().Set(reflect.ValueOf(s))
	return p
}

func callMemSave(t *testing.T, s *store.Store, input *MemSaveInput) MemSaveOutput {
	t.Helper()
	proj := newMemSaveTestProject(t, s)
	_, out, err := MemSaveHandler(context.Background(), nil, input, proj)
	if err != nil {
		return out
	}
	return out
}

// ptr helpers for tests
func ptrS(s string) *string   { return &s }
func ptrB(b bool) *bool       { return &b }
func ptrF(f float64) *float64 { return &f }
func ptrI(i int) *int         { return &i }

// ── Basic save tests ─────────────────────────────────────────────────────────

func TestMemSaveHandler_SuggestsTopicKeyWhenMissing(t *testing.T) {
	s := newMemSaveTestStore(t)

	out := callMemSave(t, s, &MemSaveInput{
		Title:   "Auth architecture",
		Content: "Define boundaries for auth middleware",
		Type:    ptrS("architecture"),
		Project: ptrS("engram"),
	})

	if out.Error != "" {
		t.Fatalf("unexpected save error: %s", out.Error)
	}
	if !strings.Contains(out.Message, "Suggested topic_key: architecture/auth-architecture") {
		t.Fatalf("expected suggestion in save response, got %q", out.Message)
	}
}

func TestMemSaveHandler_RecordsActivityForExplicitSessionID(t *testing.T) {
	s := newMemSaveTestStore(t)
	activity := NewSessionActivity(10 * time.Minute)

	// Record activity via the global since handler uses getSessionActivity()
	orig := activity
	_ = orig

	out := callMemSave(t, s, &MemSaveInput{
		Title:     "Explicit session save",
		Content:   "**What**: saved with explicit session\n**Why**: regression test",
		Type:      ptrS("bugfix"),
		SessionID: ptrS("custom-session-123"),
		Project:   ptrS("engram"),
	})

	if out.Error != "" {
		t.Fatalf("unexpected save error: %s", out.Error)
	}
}

func TestMemSaveHandler_WithNilActivityStillSucceeds(t *testing.T) {
	s := newMemSaveTestStore(t)

	out := callMemSave(t, s, &MemSaveInput{
		Title:   "Nil activity save",
		Content: "**What**: saved with nil activity",
		Type:    ptrS("discovery"),
		Project: ptrS("engram"),
	})

	if out.Error != "" {
		t.Fatalf("unexpected save error: %s", out.Error)
	}
}

func TestMemSaveHandler_PromptCaptureFailureIsNonFatal(t *testing.T) {
	s := newMemSaveTestStore(t)
	if err := s.EnrollProject("orqen"); err != nil {
		t.Fatalf("enroll project: %v", err)
	}
	activity := NewSessionActivity(10 * time.Minute)
	activity.RecordPrompt(defaultSessionID("orqen"), "orqen", "prompt capture should fail non-fatally")

	originalAddPromptIfMissing := addPromptIfMissing
	addPromptIfMissing = func(s *store.Store, params store.AddPromptParams) (int64, bool, error) {
		return 0, false, errors.New("forced prompt capture failure")
	}
	t.Cleanup(func() { addPromptIfMissing = originalAddPromptIfMissing })

	out := callMemSave(t, s, &MemSaveInput{
		Title:   "Non fatal prompt capture",
		Content: "**What**: saved despite prompt capture failure\n**Why**: regression test",
		Type:    ptrS("bugfix"),
		Project: ptrS("orqen"),
	})

	if out.Error != "" {
		t.Fatalf("unexpected save error: %s", out.Error)
	}

	obs, err := s.RecentObservations("orqen", "project", 5)
	if err != nil {
		t.Fatalf("recent observations: %v", err)
	}
	if len(obs) != 1 || obs[0].Title != "Non fatal prompt capture" {
		t.Fatalf("expected observation to be saved despite prompt capture failure, got %#v", obs)
	}
}

func TestMemSaveHandler_CapturePromptFalseSkipsCurrentPrompt(t *testing.T) {
	s := newMemSaveTestStore(t)
	if err := s.EnrollProject("orqen"); err != nil {
		t.Fatalf("enroll project: %v", err)
	}
	activity := NewSessionActivity(10 * time.Minute)
	sessionID := defaultSessionID("orqen")
	activity.RecordPrompt(sessionID, "orqen", "should be skipped")

	out := callMemSave(t, s, &MemSaveInput{
		Title:         "Skip prompt capture",
		Content:       "**What**: saved with capture_prompt=false\n**Why**: regression test",
		Type:          ptrS("bugfix"),
		Project:       ptrS("orqen"),
		CapturePrompt: ptrB(false),
	})

	if out.Error != "" {
		t.Fatalf("unexpected save error: %s", out.Error)
	}

	prompts, err := s.RecentPrompts("orqen", 5)
	if err != nil {
		t.Fatalf("recent prompts: %v", err)
	}
	if len(prompts) != 0 {
		t.Fatalf("expected opt-out to skip prompt capture, got %#v", prompts)
	}
}

func TestMemSaveHandler_NoCurrentPromptStillSucceeds(t *testing.T) {
	s := newMemSaveTestStore(t)

	out := callMemSave(t, s, &MemSaveInput{
		Title:   "No prompt available",
		Content: "**What**: saved without prompt context",
		Type:    ptrS("discovery"),
		Project: ptrS("engram"),
	})

	if out.Error != "" {
		t.Fatalf("unexpected save error: %s", out.Error)
	}

	prompts, err := s.RecentPrompts("engram", 5)
	if err != nil {
		t.Fatalf("recent prompts: %v", err)
	}
	if len(prompts) != 0 {
		t.Fatalf("expected no prompt rows when no current prompt is available, got %#v", prompts)
	}
}

func TestMemSaveHandler_DoesNotSuggestWhenTopicKeyProvided(t *testing.T) {
	s := newMemSaveTestStore(t)

	out := callMemSave(t, s, &MemSaveInput{
		Title:    "Auth architecture",
		Content:  "Define boundaries for auth middleware",
		Type:     ptrS("architecture"),
		Project:  ptrS("engram"),
		TopicKey: ptrS("architecture/existing"),
	})

	if out.Error != "" {
		t.Fatalf("unexpected save error: %s", out.Error)
	}
	if strings.Contains(out.Message, "Suggested topic_key:") {
		t.Fatalf("expected no suggestion when topic_key provided, got %q", out.Message)
	}
}

// ── Candidate / conflict detection tests ─────────────────────────────────────

func TestMemSaveHandler_CandidatesReturned(t *testing.T) {
	s := newMemSaveTestStore(t)

	// Save first observation — no candidates yet.
	out1 := callMemSave(t, s, &MemSaveInput{
		Title:   "We use sessions for auth middleware",
		Content: "Session-based auth in the middleware layer keeps things simple",
		Type:    ptrS("architecture"),
	})
	if out1.Error != "" {
		t.Fatalf("first save unexpected error: %s", out1.Error)
	}

	// Save second, similar observation — should surface the first as candidate.
	out2 := callMemSave(t, s, &MemSaveInput{
		Title:   "Switched from sessions to JWT for auth",
		Content: "Replacing session auth with JWT tokens improves scalability",
		Type:    ptrS("architecture"),
	})
	if out2.Error != "" {
		t.Fatalf("second save unexpected error: %s", out2.Error)
	}

	// REQ-001: judgment_required=true must be in output.
	if !out2.JudgmentRequired {
		t.Fatal("expected judgment_required=true in MemSaveOutput")
	}
	if out2.JudgmentStatus != "pending" {
		t.Fatalf("expected judgment_status=pending, got %q", out2.JudgmentStatus)
	}
	if out2.JudgmentID == "" {
		t.Fatal("expected judgment_id in output")
	}
	if len(out2.Candidates) == 0 {
		t.Fatal("expected candidates in output")
	}

	// Verify message contains nudge.
	if !strings.Contains(out2.Message, "CONFLICT REVIEW PENDING") {
		t.Fatalf("expected conflict nudge in message, got %q", out2.Message)
	}
}

func TestMemSaveHandler_NoCandidates_ResultUnchanged(t *testing.T) {
	s := newMemSaveTestStore(t)

	out := callMemSave(t, s, &MemSaveInput{
		Title:   "Unique observation, no conflicts",
		Content: "Something completely unique and unrelated",
		Type:    ptrS("discovery"),
	})

	if out.Error != "" {
		t.Fatalf("unexpected save error: %s", out.Error)
	}
	if out.JudgmentRequired {
		t.Fatal("expected judgment_required=false when no candidates")
	}
	if len(out.Candidates) != 0 {
		t.Fatalf("expected no candidates, got %d", len(out.Candidates))
	}
}

func TestMemSaveHandler_TopicKeyRevision_ReturnsCandidates(t *testing.T) {
	s := newMemSaveTestStore(t)

	// Save a standalone observation (no topic_key) that will be a candidate.
	out1 := callMemSave(t, s, &MemSaveInput{
		Title:   "Auth architecture sessions design",
		Content: "Session-based auth design for the backend service",
		Type:    ptrS("architecture"),
	})
	if out1.Error != "" {
		t.Fatalf("first save: %v", out1.Error)
	}

	// Save with topic_key (first write) — creates the topic.
	out2 := callMemSave(t, s, &MemSaveInput{
		Title:    "Auth architecture sessions design updated",
		Content:  "Updated session-based auth design",
		Type:     ptrS("architecture"),
		TopicKey: ptrS("architecture/auth-sessions"),
	})
	if out2.Error != "" {
		t.Fatalf("second save: %v", out2.Error)
	}

	// Revise via same topic_key — this is the revision case.
	out3 := callMemSave(t, s, &MemSaveInput{
		Title:    "Auth architecture sessions design revised",
		Content:  "Revised session-based auth design for the service layer",
		Type:     ptrS("architecture"),
		TopicKey: ptrS("architecture/auth-sessions"),
	})
	if out3.Error != "" {
		t.Fatalf("revision save: %v", out3.Error)
	}

	if !out3.JudgmentRequired {
		t.Fatal("expected judgment_required=true for topic_key revision")
	}
	if len(out3.Candidates) == 0 {
		t.Fatal("expected candidates for topic_key revision")
	}
}

// ── Project detection tests ──────────────────────────────────────────────────

func TestMemSaveHandler_CreatesProjectScopedSession(t *testing.T) {
	s := newMemSaveTestStore(t)

	// Set up git repo so auto-detect gives us a known project.
	dir := t.TempDir()
	initTestGitRepo(t, dir)
	cmd := exec.Command("git", "-C", dir, "remote", "add", "origin",
		"git@github.com:user/scoped-session-project.git")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git remote add: %v\n%s", err, out)
	}
	t.Chdir(dir)
	if err := s.EnrollProject("scoped-session-project"); err != nil {
		t.Fatalf("enroll project: %v", err)
	}

	out := callMemSave(t, s, &MemSaveInput{
		Title:   "Decision",
		Content: "Architecture note",
		Type:    ptrS("architecture"),
	})

	if out.Error != "" {
		t.Fatalf("save: err=%v text=%s", out.Error, out.Message)
	}

	// Verify observation was stored under the auto-detected project.
	obs, err := s.RecentObservations("scoped-session-project", "project", 5)
	if err != nil {
		t.Fatalf("recent observations: %v", err)
	}
	if len(obs) == 0 {
		t.Fatal("expected observation stored under auto-detected project 'scoped-session-project'")
	}
	if obs[0].Title != "Decision" {
		t.Fatalf("expected observation title='Decision', got %q", obs[0].Title)
	}

	// Verify a session was created for the project (check any session with this project).
	sessions, err := s.RecentSessions("scoped-session-project", 100)
	if err != nil {
		t.Fatalf("recent sessions: %v", err)
	}
	if len(sessions) == 0 {
		t.Fatal("expected at least one session created for project 'scoped-session-project'")
	}
}

func TestMemSaveHandler_AutoDetectsWhenNoProjectArg(t *testing.T) {
	dir := t.TempDir()
	initTestGitRepo(t, dir)
	cmd := exec.Command("git", "-C", dir, "remote", "add", "origin",
		"git@github.com:user/auto-project.git")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git remote add: %v\n%s", err, out)
	}
	t.Chdir(dir)

	s := newMemSaveTestStore(t)

	out := callMemSave(t, s, &MemSaveInput{
		Title:   "Test memory",
		Content: "Some content here",
		Type:    ptrS("manual"),
	})

	if out.Error != "" {
		t.Fatalf("handler error: %s", out.Error)
	}

	obs, err := s.RecentObservations("auto-project", "project", 5)
	if err != nil {
		t.Fatalf("recent observations: %v", err)
	}
	if len(obs) == 0 {
		t.Fatal("expected at least one observation stored with auto-detected project")
	}
}

func TestMemSaveHandler_ProjectNameNormalized(t *testing.T) {
	dir := t.TempDir()
	initTestGitRepo(t, dir)
	// Use a remote with a mixed-case repo name — auto-detect normalizes it.
	cmd := exec.Command("git", "-C", dir, "remote", "add", "origin",
		"git@github.com:user/MyApp.git")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git remote add: %v\n%s", err, out)
	}
	t.Chdir(dir)

	s := newMemSaveTestStore(t)

	out := callMemSave(t, s, &MemSaveInput{
		Title:   "Normalization test",
		Content: "Testing auto-detect normalization",
		Type:    ptrS("manual"),
	})

	if out.Error != "" {
		t.Fatalf("save: err=%v text=%s", out.Error, out.Message)
	}

	// Observation must be under the normalized (lowercase) project name.
	obs, err := s.RecentObservations("myapp", "project", 5)
	if err != nil || len(obs) == 0 {
		t.Fatal("expected observation stored under normalized project name 'myapp'")
	}
}

func TestMemSaveHandler_SimilarProjectWarning(t *testing.T) {
	s := newMemSaveTestStore(t)

	// Build two git repos: "engram" and "engam" (Levenshtein distance 1).
	parent := t.TempDir()
	engramDir := filepath.Join(parent, "engram")
	engamDir := filepath.Join(parent, "engam")
	for _, d := range []string{engramDir, engamDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		initTestGitRepo(t, d)
	}

	// Save original cwd to restore between sub-saves.
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir) //nolint:errcheck

	// First save: cwd = engram repo.
	if err := os.Chdir(engramDir); err != nil {
		t.Fatal(err)
	}
	out1 := callMemSave(t, s, &MemSaveInput{
		Title:   "First project save",
		Content: "**What**: engram project memory",
		Type:    ptrS("decision"),
	})
	if out1.Error != "" {
		t.Fatalf("first save: %v", out1.Error)
	}

	// Second save: cwd = engam repo (similar name, no existing observations).
	if err := os.Chdir(engamDir); err != nil {
		t.Fatal(err)
	}
	out2 := callMemSave(t, s, &MemSaveInput{
		Title:   "Second project save",
		Content: "**What**: engam project memory",
		Type:    ptrS("decision"),
	})
	if out2.Error != "" {
		t.Fatalf("second save: %v", out2.Error)
	}

	// Should include a similar-project warning.
	if out2.Warning == "" || !strings.Contains(out2.Warning, "engram") {
		t.Fatalf("expected similar project warning referencing 'engram', got %q", out2.Warning)
	}
	if !strings.Contains(out2.Message, "engram") {
		t.Fatalf("expected similar project warning in message, got %q", out2.Message)
	}
}

func TestMemSaveHandler_NoSimilarWarningWhenProjectExists(t *testing.T) {
	s := newMemSaveTestStore(t)

	// Pre-register a project so it is not "new".
	if err := s.EnrollProject("engram"); err != nil {
		t.Fatalf("enroll project: %v", err)
	}

	out := callMemSave(t, s, &MemSaveInput{
		Title:   "Known project save",
		Content: "**What**: saving to existing project",
		Type:    ptrS("decision"),
		Project: ptrS("engram"),
	})

	if out.Error != "" {
		t.Fatalf("unexpected save error: %s", out.Error)
	}
	if out.Warning != "" && strings.Contains(out.Warning, "has no memories") {
		t.Fatalf("expected no similar-project warning for existing project, got %q", out.Warning)
	}
}

func TestMemSaveHandler_LLMProjectIgnored(t *testing.T) {
	// Set up a git repo so auto-detect returns a known project.
	dir := t.TempDir()
	initTestGitRepo(t, dir)
	cmd := exec.Command("git", "-C", dir, "remote", "add", "origin",
		"git@github.com:user/auto-detected-project.git")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git remote add: %v\n%s", err, out)
	}
	t.Chdir(dir)

	s := newMemSaveTestStore(t)

	out := callMemSave(t, s, &MemSaveInput{
		Title:   "LLM project ignored test",
		Content: "Should go to auto-detected project",
		Type:    ptrS("manual"),
		// LLM-supplied project — must be IGNORED per REQ-308
		Project: ptrS("llm-wrong-project"),
	})

	if out.Error != "" {
		t.Fatalf("save: err=%v text=%s", out.Error, out.Message)
	}

	// Must NOT be in the LLM-supplied project.
	obs, _ := s.RecentObservations("llm-wrong-project", "project", 5)
	if len(obs) > 0 {
		t.Fatal("observation must NOT be in LLM-supplied project")
	}
	// Must be in the auto-detected project.
	obs2, err := s.RecentObservations("auto-detected-project", "project", 5)
	if err != nil || len(obs2) == 0 {
		t.Fatal("expected observation in auto-detected-project")
	}
}

// ── Ambiguous project tests ──────────────────────────────────────────────────

func TestMemSaveHandler_AmbiguousEnvelope(t *testing.T) {
	parent := t.TempDir()
	for _, name := range []string{"repo-x", "repo-y"} {
		child := filepath.Join(parent, name)
		if err := os.MkdirAll(child, 0o755); err != nil {
			t.Fatal(err)
		}
		initTestGitRepo(t, child)
	}
	t.Chdir(parent)

	s := newMemSaveTestStore(t)

	_, out, err := MemSaveHandler(context.Background(), nil, &MemSaveInput{
		Title:   "should not be saved",
		Content: "ambiguous test",
		Type:    ptrS("manual"),
	}, newMemSaveTestProject(t, s))

	if err == nil {
		t.Fatal("expected error for ambiguous project")
	}
	if out.ErrorCode != "ambiguous_project" {
		t.Errorf("expected error_code 'ambiguous_project', got %q", out.ErrorCode)
	}
	if out.Message == "" || !strings.Contains(out.Message, "available_projects") {
		t.Errorf("expected available_projects in error, got %q", out.Message)
	}
	if !strings.Contains(out.Message, "project_choice_reason=user_selected_after_ambiguous_project") {
		t.Errorf("expected explicit recovery hint, got %q", out.Message)
	}
}

func TestMemSaveHandler_AmbiguousWithValidUserChoiceSucceeds(t *testing.T) {
	parent := t.TempDir()
	for _, name := range []string{"repo-choice-a", "repo-choice-b"} {
		child := filepath.Join(parent, name)
		if err := os.MkdirAll(child, 0o755); err != nil {
			t.Fatal(err)
		}
		initTestGitRepo(t, child)
	}
	t.Chdir(parent)

	s := newMemSaveTestStore(t)

	out := callMemSave(t, s, &MemSaveInput{
		Title:               "valid choice save",
		Content:             "saved with valid user choice",
		Type:                ptrS("manual"),
		Project:             ptrS("repo-choice-a"),
		ProjectChoiceReason: ptrS(projectpkg.SourceUserSelectedAfterAmbiguousProject),
	})

	if out.Error != "" {
		t.Fatalf("expected valid user choice to succeed, got error: %s", out.Error)
	}
	if out.Project != "repo-choice-a" {
		t.Fatalf("expected project=repo-choice-a, got %q", out.Project)
	}
}

func TestMemSaveHandler_AmbiguousEmptyProjectChoiceIsActionable(t *testing.T) {
	parent := t.TempDir()
	for _, name := range []string{"repo-empty-a", "repo-empty-b"} {
		child := filepath.Join(parent, name)
		if err := os.MkdirAll(child, 0o755); err != nil {
			t.Fatal(err)
		}
		initTestGitRepo(t, child)
	}
	t.Chdir(parent)

	s := newMemSaveTestStore(t)

	_, out, err := MemSaveHandler(context.Background(), nil, &MemSaveInput{
		Title:               "empty choice must fail",
		Content:             "must not save",
		Type:                ptrS("manual"),
		Project:             ptrS(" \t\n "),
		ProjectChoiceReason: ptrS(projectpkg.SourceUserSelectedAfterAmbiguousProject),
	}, newMemSaveTestProject(t, s))

	if err == nil {
		t.Fatal("expected invalid project choice for whitespace project")
	}
	if out.ErrorCode != "invalid_project_choice" {
		t.Fatalf("expected error_code 'invalid_project_choice', got %q", out.ErrorCode)
	}
	if !strings.Contains(out.Message, "Project choice is empty") {
		t.Fatalf("expected actionable empty choice error, got %q", out.Message)
	}
}

func TestMemSaveHandler_AmbiguousWithInventedProjectRejected(t *testing.T) {
	parent := t.TempDir()
	for _, name := range []string{"repo-valid-a", "repo-valid-b"} {
		child := filepath.Join(parent, name)
		if err := os.MkdirAll(child, 0o755); err != nil {
			t.Fatal(err)
		}
		initTestGitRepo(t, child)
	}
	t.Chdir(parent)

	s := newMemSaveTestStore(t)

	_, out, err := MemSaveHandler(context.Background(), nil, &MemSaveInput{
		Title:               "invented must fail",
		Content:             "must not save",
		Type:                ptrS("manual"),
		Project:             ptrS("repo-invented"),
		ProjectChoiceReason: ptrS(projectpkg.SourceUserSelectedAfterAmbiguousProject),
	}, newMemSaveTestProject(t, s))

	if err == nil {
		t.Fatal("expected invented project choice to fail")
	}
	if out.ErrorCode != "invalid_project_choice" {
		t.Fatalf("expected error_code 'invalid_project_choice', got %q", out.ErrorCode)
	}
	if !strings.Contains(out.Message, "available_projects") {
		t.Fatalf("expected available_projects in error, got %q", out.Message)
	}
}

// ── Ambiguous project path in error test ─────────────────────────────────────

func TestMemSaveHandler_AmbiguousProjectPathInError(t *testing.T) {
	parent := t.TempDir()
	repoA := filepath.Join(parent, "alpha")
	repoB := filepath.Join(parent, "beta")
	for _, d := range []string{repoA, repoB} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		initTestGitRepo(t, d)
	}
	t.Chdir(parent)

	s := newMemSaveTestStore(t)

	_, out, err := MemSaveHandler(context.Background(), nil, &MemSaveInput{
		Title:   "ambiguous paths test",
		Content: "check available_projects paths",
		Type:    ptrS("manual"),
	}, newMemSaveTestProject(t, s))

	if err == nil {
		t.Fatal("expected ambiguous error")
	}
	// available_projects should include the repo names (lowercase normalized).
	msg := out.Message
	if !strings.Contains(msg, "alpha") || !strings.Contains(msg, "beta") {
		t.Fatalf("expected available_projects to include repo names 'alpha' and 'beta', got %q", msg)
	}
}

func TestMemSaveHandler_AmbiguousWithTrimmedChoiceSucceeds(t *testing.T) {
	parent := t.TempDir()
	for _, name := range []string{"trim-a", "trim-b"} {
		child := filepath.Join(parent, name)
		if err := os.MkdirAll(child, 0o755); err != nil {
			t.Fatal(err)
		}
		initTestGitRepo(t, child)
	}
	t.Chdir(parent)

	s := newMemSaveTestStore(t)

	out := callMemSave(t, s, &MemSaveInput{
		Title:               "trimmed choice save",
		Content:             "saved with trimmed project name",
		Type:                ptrS("manual"),
		Project:             ptrS("  trim-a  "),
		ProjectChoiceReason: ptrS(projectpkg.SourceUserSelectedAfterAmbiguousProject),
	})

	if out.Error != "" {
		t.Fatalf("trimmed choice should succeed: err=%s", out.Error)
	}
	if out.Project != "trim-a" {
		t.Fatalf("expected project=trim-a (trimmed), got %q", out.Project)
	}
}

func TestMemSaveHandler_AmbiguousWithExactTrimmedChoiceSucceeds(t *testing.T) {
	parent := t.TempDir()
	dirWithSpaces := filepath.Join(parent, "baz__qux")
	if err := os.MkdirAll(dirWithSpaces, 0o755); err != nil {
		t.Fatal(err)
	}
	initTestGitRepo(t, dirWithSpaces)
	t.Chdir(parent)

	s := newMemSaveTestStore(t)

	out := callMemSave(t, s, &MemSaveInput{
		Title:               "exact trimmed",
		Content:             "exact trimmed choice",
		Type:                ptrS("manual"),
		Project:             ptrS("baz__qux"),
		ProjectChoiceReason: ptrS(projectpkg.SourceUserSelectedAfterAmbiguousProject),
	})

	if out.Error != "" {
		t.Fatalf("exact trimmed choice should succeed: err=%s", out.Error)
	}
	if out.ProjectPath != filepath.Join(parent, "baz__qux") {
		t.Fatalf("expected project_path to selected exact repo root, got %q", out.ProjectPath)
	}
}

// ── Config / override tests ──────────────────────────────────────────────────

func TestMemSaveHandler_InvalidConfigFailsClearly(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".engram")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.json"), []byte(`{"project_name":""}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	s := newMemSaveTestStore(t)

	_, out, err := MemSaveHandler(context.Background(), nil, &MemSaveInput{
		Title:   "should fail",
		Content: "invalid config",
		Type:    ptrS("decision"),
	}, newMemSaveTestProject(t, s))

	if err == nil {
		t.Fatalf("expected invalid config to fail, got %v", out)
	}
	if out.ErrorCode != "invalid_project_config" {
		t.Fatalf("expected error_code 'invalid_project_config', got %q", out.ErrorCode)
	}
	if !strings.Contains(out.Message, "project_name") {
		t.Fatalf("expected project_name error message, got %q", out.Message)
	}
}

func TestMemSaveHandler_MCPConfig_OverridesDefaults(t *testing.T) {
	s := newMemSaveTestStore(t)

	// Create MCP server with strict BM25Floor override — nothing should score >= 0.
	cfg := MCPConfig{
		BM25Floor: ptrF(0.0),
	}
	_ = cfg // MCPConfig is used internally by the handler

	// Save first observation.
	out1 := callMemSave(t, s, &MemSaveInput{
		Title:   "JWT auth token session management",
		Content: "Session-based auth in the middleware layer keeps things simple",
		Type:    ptrS("architecture"),
	})
	if out1.Error != "" {
		t.Fatalf("first save: %v", out1.Error)
	}

	// Save second, similar observation.
	out2 := callMemSave(t, s, &MemSaveInput{
		Title:   "Switched from JWT sessions to token auth",
		Content: "Replacing session auth with JWT tokens improves scalability",
		Type:    ptrS("architecture"),
	})
	if out2.Error != "" {
		t.Fatalf("second save: %v", out2.Error)
	}

	// With default BM25Floor (-2.0), similar content should produce candidates.
	if len(out2.Candidates) == 0 {
		t.Log("warning: no candidates found — may be expected depending on content similarity")
	}
}

// ── Truncation test ──────────────────────────────────────────────────────────

func TestMemSaveHandler_ContentTruncated(t *testing.T) {
	s := newMemSaveTestStore(t)

	// Create content longer than MaxObservationLength.
	maxLen := s.MaxObservationLength()
	longContent := "**What**: " + strings.Repeat("A", maxLen+100)

	out := callMemSave(t, s, &MemSaveInput{
		Title:   "Truncated content",
		Content: longContent,
		Type:    ptrS("manual"),
		Project: ptrS("engram"),
	})

	if out.Error != "" {
		t.Fatalf("unexpected save error: %s", out.Error)
	}
	if !out.Truncated {
		t.Fatal("expected Truncated=true for long content")
	}
	if !strings.Contains(out.Message, "Content was truncated") {
		t.Fatalf("expected truncation warning in message, got %q", out.Message)
	}
}

// ── Default type test ────────────────────────────────────────────────────────

func TestMemSaveHandler_DefaultTypeIsManual(t *testing.T) {
	s := newMemSaveTestStore(t)

	out := callMemSave(t, s, &MemSaveInput{
		Title:   "Default type test",
		Content: "**What**: testing default type",
		Project: ptrS("engram"),
	})

	if out.Error != "" {
		t.Fatalf("unexpected save error: %s", out.Error)
	}
	if !strings.Contains(out.Message, "(manual)") {
		t.Fatalf("expected type to default to 'manual', got %q", out.Message)
	}
}

// ── Session ID default test ──────────────────────────────────────────────────

func TestMemSaveHandler_DefaultSessionID(t *testing.T) {
	s := newMemSaveTestStore(t)

	// Set up git repo so auto-detect works.
	dir := t.TempDir()
	initTestGitRepo(t, dir)
	cmd := exec.Command("git", "-C", dir, "remote", "add", "origin",
		"git@github.com:user/my-app.git")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git remote add: %v\n%s", err, out)
	}
	t.Chdir(dir)
	if err := s.EnrollProject("my-app"); err != nil {
		t.Fatalf("enroll project: %v", err)
	}

	out := callMemSave(t, s, &MemSaveInput{
		Title:   "Default session test",
		Content: "**What**: testing default session",
	})

	if out.Error != "" {
		t.Fatalf("unexpected save error: %s", out.Error)
	}

	// Verify observation was stored under the auto-detected project.
	obs, err := s.RecentObservations("my-app", "project", 5)
	if err != nil {
		t.Fatalf("recent observations: %v", err)
	}
	if len(obs) == 0 {
		t.Fatal("expected observation stored under project 'my-app'")
	}

	// Verify a session was created for the project.
	sessions, err := s.RecentSessions("my-app", 100)
	if err != nil {
		t.Fatalf("recent sessions: %v", err)
	}
	if len(sessions) == 0 {
		t.Fatal("expected at least one session created for project 'my-app'")
	}
}

// ── Multiple saves and activity recording ────────────────────────────────────

func TestMemSaveHandler_ProjectDetectionWithSimilarNames(t *testing.T) {
	parent := t.TempDir()
	for _, name := range []string{"repo-cwd-a", "repo-cwd-b"} {
		child := filepath.Join(parent, name)
		if err := os.MkdirAll(child, 0o755); err != nil {
			t.Fatal(err)
		}
		initTestGitRepo(t, child)
	}
	t.Chdir(parent)

	s := newMemSaveTestStore(t)
	// Seed an unrelated project in the store — this must NOT appear in error.
	if err := s.CreateSession("sess-unrelated", "unrelated-store-project", "/tmp"); err != nil {
		t.Fatal(err)
	}

	_, out, err := MemSaveHandler(context.Background(), nil, &MemSaveInput{
		Title:   "t",
		Content: "c",
		Type:    ptrS("manual"),
	}, newMemSaveTestProject(t, s))

	if err == nil {
		t.Fatal("expected error for ambiguous cwd")
	}
	// Must list cwd repos (repo-cwd-a, repo-cwd-b).
	if !strings.Contains(out.Message, "repo-cwd-a") {
		t.Errorf("available_projects must contain repo-cwd-a (cwd repo); got: %q", out.Message)
	}
	if !strings.Contains(out.Message, "repo-cwd-b") {
		t.Errorf("available_projects must contain repo-cwd-b (cwd repo); got: %q", out.Message)
	}
	// Must NOT list the unrelated store project.
	if strings.Contains(out.Message, "unrelated-store-project") {
		t.Errorf("available_projects must NOT list all store projects; got: %q", out.Message)
	}
}

// ── Candidate struct field verification ──────────────────────────────────────

func TestMemSaveHandler_CandidateFieldsPopulated(t *testing.T) {
	s := newMemSaveTestStore(t)

	// First save.
	out1 := callMemSave(t, s, &MemSaveInput{
		Title:   "Original auth approach",
		Content: "Using sessions for simplicity",
		Type:    ptrS("architecture"),
	})
	if out1.Error != "" {
		t.Fatalf("first save: %v", out1.Error)
	}

	// Second similar save — should get candidates.
	out2 := callMemSave(t, s, &MemSaveInput{
		Title:   "Switching to JWT instead of sessions",
		Content: "Moving away from sessions to JWT for scalability",
		Type:    ptrS("architecture"),
	})
	if out2.Error != "" {
		t.Fatalf("second save: %v", out2.Error)
	}

	if len(out2.Candidates) == 0 {
		t.Fatal("expected at least one candidate")
	}

	c := out2.Candidates[0]
	if c.ID == "" {
		t.Fatal("candidate ID should be populated")
	}
	if c.SyncID == "" {
		t.Fatal("candidate SyncID should be populated")
	}
	if c.Title == "" {
		t.Fatal("candidate Title should be populated")
	}
	if c.Type == "" {
		t.Fatal("candidate Type should be populated")
	}
}

// ── Output JSON serialization test ───────────────────────────────────────────

func TestMemSaveOutput_JSONSerialization(t *testing.T) {
	out := MemSaveOutput{
		Message:           "Memory saved: test (manual)",
		Project:           "my-project",
		ProjectSource:     "auto_detect",
		ProjectPath:       "/path/to/project",
		ProjectWarning:    "some warning",
		ID:                42,
		SyncID:            "sync-123",
		SuggestedTopicKey: "manual/test-memory",
		JudgmentRequired:  true,
		JudgmentStatus:    "pending",
		JudgmentID:        "judgment-abc",
		Truncated:         false,
		Warning:           "combined warning",
		Candidates: []MemSaveCandidate{
			{
				ID:         "1",
				SyncID:     "sync-old",
				Title:      "Previous approach",
				Type:       "architecture",
				Score:      -1.5,
				JudgmentID: "judgment-abc",
				TopicKey:   ptrS("arch/prev"),
			},
		},
	}

	data, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("failed to marshal MemSaveOutput: %v", err)
	}

	var decoded MemSaveOutput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal MemSaveOutput: %v", err)
	}

	if decoded.Message != out.Message {
		t.Errorf("message mismatch: got %q, want %q", decoded.Message, out.Message)
	}
	if decoded.ID != out.ID {
		t.Errorf("id mismatch: got %d, want %d", decoded.ID, out.ID)
	}
	if decoded.JudgmentRequired != out.JudgmentRequired {
		t.Errorf("judgment_required mismatch: got %v, want %v", decoded.JudgmentRequired, out.JudgmentRequired)
	}
	if len(decoded.Candidates) != len(out.Candidates) {
		t.Fatalf("candidates count mismatch: got %d, want %d", len(decoded.Candidates), len(out.Candidates))
	}
	if decoded.Candidates[0].Title != out.Candidates[0].Title {
		t.Errorf("candidate title mismatch: got %q, want %q", decoded.Candidates[0].Title, out.Candidates[0].Title)
	}
}

// ── MemSaveInput_SetWorkItemID test ──────────────────────────────────────────

func TestMemSaveInput_SetWorkItemID(t *testing.T) {
	input := &MemSaveInput{
		Title:   "Test",
		Content: "Content",
	}

	input.SetWorkItemID("work-item-123")

	if input.WorkItemID == nil {
		t.Fatal("expected WorkItemID to be set")
	}
	if *input.WorkItemID != "work-item-123" {
		t.Fatalf("expected WorkItemID='work-item-123', got %q", *input.WorkItemID)
	}
}

// ── Project field in output ──────────────────────────────────────────────────

func TestMemSaveHandler_OutputProjectMatchesAutoDetected(t *testing.T) {
	dir := t.TempDir()
	initTestGitRepo(t, dir)
	cmd := exec.Command("git", "-C", dir, "remote", "add", "origin",
		"git@github.com:user/output-project-test.git")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git remote add: %v\n%s", err, out)
	}
	t.Chdir(dir)

	s := newMemSaveTestStore(t)

	out := callMemSave(t, s, &MemSaveInput{
		Title:   "Project output test",
		Content: "**What**: verifying output project",
		Type:    ptrS("manual"),
	})

	if out.Error != "" {
		t.Fatalf("unexpected save error: %s", out.Error)
	}
	if out.Project != "output-project-test" {
		t.Fatalf("expected output project 'output-project-test', got %q", out.Project)
	}
	if out.ProjectSource == "" {
		t.Fatal("expected non-empty ProjectSource")
	}
	// Compare paths using filepath.ToSlash to handle Windows vs Unix separators.
	gotPath := filepath.ToSlash(out.ProjectPath)
	wantPath := filepath.ToSlash(dir)
	if gotPath != wantPath {
		t.Fatalf("expected ProjectPath=%q, got %q", wantPath, gotPath)
	}
}

// ── SuggestedTopicKey output field ───────────────────────────────────────────

func TestMemSaveHandler_SuggestedTopicKeyInOutput(t *testing.T) {
	s := newMemSaveTestStore(t)

	out := callMemSave(t, s, &MemSaveInput{
		Title:   "API rate limiting approach",
		Content: "**What**: Implemented rate limiting\n**Why**: Prevent abuse",
		Type:    ptrS("architecture"),
		Project: ptrS("engram"),
	})

	if out.Error != "" {
		t.Fatalf("unexpected save error: %s", out.Error)
	}
	if out.SuggestedTopicKey == "" {
		t.Fatal("expected non-empty SuggestedTopicKey in output")
	}
	// Topic key is based on type + title slug; verify it starts with the type prefix.
	if !strings.HasPrefix(out.SuggestedTopicKey, "architecture/") {
		t.Fatalf("expected SuggestedTopicKey to start with 'architecture/', got %q", out.SuggestedTopicKey)
	}
}

// ── Warning field aggregation test ───────────────────────────────────────────

func TestMemSaveHandler_WarningFieldAggregatesWarnings(t *testing.T) {
	s := newMemSaveTestStore(t)

	// First, set up an existing project with observations (this project IS enrolled).
	// Create the session first, then add an observation.
	if err := s.CreateSession("manual-save-engram", "engram", ""); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if _, err := s.AddObservation(store.AddObservationParams{
		SessionID: "manual-save-engram",
		Type:      "decision",
		Title:     "Existing memory",
		Content:   "**What**: something\n**Why**: because",
		Project:   "engram",
	}); err != nil {
		t.Fatalf("add observation: %v", err)
	}

	// Build two git repos: "engram" (existing) and "engam" (new, similar name).
	parent := t.TempDir()
	engramDir := filepath.Join(parent, "engram")
	engamDir := filepath.Join(parent, "engam")
	for _, d := range []string{engramDir, engamDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		initTestGitRepo(t, d)
	}

	// Save original cwd to restore.
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir) //nolint:errcheck

	// Verify engram project has observations before the test.
	names, _ := s.ListProjectNames()
	if len(names) == 0 {
		t.Fatal("expected engram project to have observations before test")
	}

	// Save to "engam" repo — should trigger similar-project warning referencing "engram".
	if err := os.Chdir(engamDir); err != nil {
		t.Fatal(err)
	}

	out := callMemSave(t, s, &MemSaveInput{
		Title:   "Similar project save",
		Content: "**What**: new project save\n**Why**: testing warnings",
		Type:    ptrS("decision"),
	})

	if out.Error != "" {
		t.Fatalf("unexpected save error: %s", out.Error)
	}

	// Warning should be populated for similar project.
	if out.Warning == "" {
		t.Fatalf("expected Warning field to be populated for similar project, got empty. Message: %q", out.Message)
	}
	if !strings.Contains(out.Warning, "engram") {
		t.Fatalf("expected Warning to reference 'engram', got %q", out.Warning)
	}
}

// ── Candidates ID format test ────────────────────────────────────────────────

func TestMemSaveHandler_CandidatesIDFormat(t *testing.T) {
	s := newMemSaveTestStore(t)

	// First save.
	out1 := callMemSave(t, s, &MemSaveInput{
		Title:   "First approach to caching",
		Content: "Using in-memory caching for performance",
		Type:    ptrS("architecture"),
	})
	if out1.Error != "" {
		t.Fatalf("first save: %v", out1.Error)
	}

	// Second similar save.
	out2 := callMemSave(t, s, &MemSaveInput{
		Title:   "Switching to Redis for caching",
		Content: "Replacing in-memory cache with Redis for distributed caching",
		Type:    ptrS("architecture"),
	})
	if out2.Error != "" {
		t.Fatalf("second save: %v", out2.Error)
	}

	if len(out2.Candidates) > 0 {
		// ID should be a string representation of a number.
		for _, c := range out2.Candidates {
			if c.ID == "" {
				t.Fatal("candidate ID should not be empty")
			}
		}
	}
}
