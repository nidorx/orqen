package chat

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nidorx/orqen/pkg/engine"
)

// ── Confirmation Manager ─────────────────────────────────────────────────────

// confirmationManager is the concrete implementation of the ConfirmationManager interface.
// It handles the lifecycle of pending file edits that require user confirmation.
type confirmationManager struct {
	store   *ChatStore
	proj    *engine.Project
	pending map[string]*PendingEdit // keyed by sessionID (one pending edit per session)
	mu      sync.RWMutex
}

// NewConfirmationManager creates a new ConfirmationManager.
func NewConfirmationManager(store *ChatStore, proj *engine.Project) ConfirmationManager {
	return &confirmationManager{
		store:   store,
		proj:    proj,
		pending: make(map[string]*PendingEdit),
	}
}

// ── Create Pending Edit ──────────────────────────────────────────────────────

// CreateEdit validates and creates a pending file edit.
// Returns the edit ID on success.
func (cm *confirmationManager) CreateEdit(sessionID, filePath, content, reason string) (int64, error) {
	// Validate file path is not in blocked list
	if isBlockedPath(filePath) {
		return 0, fmt.Errorf("access denied: %q is a protected path", filePath)
	}

	// Resolve absolute path
	absPath, err := safeFilePath(cm.proj, filePath)
	if err != nil {
		return 0, err
	}

	// Read current file content
	currentContent := ""
	data, readErr := os.ReadFile(absPath)
	if readErr == nil {
		currentContent = string(data)
	}

	// Generate unified diff
	diff := GenerateDiff(currentContent, content, filePath)
	// If no meaningful diff, content is identical
	if diff == "" {
		return 0, fmt.Errorf("proposed content is identical to current file")
	}

	// Save pending edit via store
	editID, saveErr := cm.store.SavePendingEdit(sessionID, filePath, content, reason)
	if saveErr != nil {
		return 0, fmt.Errorf("save pending edit: %w", saveErr)
	}

	// Store in-memory reference
	edit := &PendingEdit{
		ID:        editID,
		SessionID: sessionID,
		FilePath:  filePath,
		Content:   content,
		Reason:    reason,
		CreatedAt: time.Now(),
	}
	cm.mu.Lock()
	cm.pending[sessionID] = edit
	cm.mu.Unlock()

	return editID, nil
}

// ── Generate Diff ────────────────────────────────────────────────────────────

const diffPreviewLimit = 50

// GenerateDiff produces a unified diff (diff -u style) between current and proposed content.
// Returns an empty string if content is identical.
// Limits preview to 50 lines; truncates with a note if larger.
func GenerateDiff(current, proposed, filePath string) string {
	if current == proposed {
		return ""
	}

	currentLines := strings.Split(current, "\n")
	proposedLines := strings.Split(proposed, "\n")

	// Find the common prefix and suffix to build a proper hunk
	// For simplicity, we produce a line-by-line unified-style diff
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("--- a/%s\n", filePath))
	sb.WriteString(fmt.Sprintf("+++ b/%s\n", filePath))

	// Compute a simple unified diff with context lines
	hunks := computeUnifiedHunks(currentLines, proposedLines)

	totalLines := 0
	for _, hunk := range hunks {
		for _, line := range hunk.lines {
			sb.WriteString(line)
			sb.WriteString("\n")
			totalLines++
			if totalLines >= diffPreviewLimit {
				sb.WriteString("\n... [diff truncated, showing first 50 lines] ...")
				return sb.String()
			}
		}
	}

	return sb.String()
}

// diffHunk represents a contiguous range of changes in a unified diff.
type diffHunk struct {
	oldStart int
	newStart int
	lines    []string
}

// computeUnifiedHunks computes unified diff hunks between old and new line slices.
func computeUnifiedHunks(oldLines, newLines []string) []diffHunk {
	// Use LCS-based approach to find differences
	// For simplicity, we use a basic diff algorithm
	lcs := longestCommonSubsequence(oldLines, newLines)

	// Build hunks from LCS
	var hunks []diffHunk
	oldIdx, newIdx := 0, 0
	lcsIdx := 0

	contextBefore := 3
	contextAfter := 3

	for lcsIdx < len(lcs) {
		lcsItem := lcs[lcsIdx]

		// Find where this LCS item appears in old and new
		oldPos := findLine(oldLines, lcsItem.Line, oldIdx)
		newPos := findLine(newLines, lcsItem.Line, newIdx)

		// Lines before this match in old
		oldChanged := oldLines[oldIdx:oldPos]
		newChanged := newLines[newIdx:newPos]

		if len(oldChanged) > 0 || len(newChanged) > 0 {
			// Build hunk with context
			ctxStart := max(0, oldPos-contextBefore)
			ctxOldBefore := oldLines[ctxStart:oldPos]

			var lines []string

			// Context before
			for _, l := range ctxOldBefore {
				lines = append(lines, " "+l)
			}

			// Removed lines
			for _, l := range oldChanged {
				lines = append(lines, "-"+l)
			}

			// Added lines
			for _, l := range newChanged {
				lines = append(lines, "+"+l)
			}

			// Context after (up to contextAfter lines from the match)
			ctxEnd := min(oldPos+contextAfter, len(oldLines))
			for i := oldPos; i < ctxEnd; i++ {
				lines = append(lines, " "+oldLines[i])
			}

			oldStart := ctxStart + 1
			newStart := (newIdx - len(ctxOldBefore)) + 1
			if newStart < 1 {
				newStart = 1
			}

			hunks = append(hunks, diffHunk{
				oldStart: oldStart,
				newStart: newStart,
				lines:    lines,
			})
		}

		oldIdx = oldPos + 1
		newIdx = newPos + 1
		lcsIdx++
	}

	// Handle trailing changes after last LCS match
	if oldIdx < len(oldLines) || newIdx < len(newLines) {
		oldChanged := oldLines[oldIdx:]
		newChanged := newLines[newIdx:]

		var lines []string
		// Context before (last few unchanged lines)
		ctxStart := max(0, len(oldLines)-len(oldChanged)-contextBefore)
		ctxOldBefore := oldLines[ctxStart : len(oldLines)-len(oldChanged)]
		for _, l := range ctxOldBefore {
			lines = append(lines, " "+l)
		}

		for _, l := range oldChanged {
			lines = append(lines, "-"+l)
		}
		for _, l := range newChanged {
			lines = append(lines, "+"+l)
		}

		oldStart := ctxStart + 1
		newStart := (newIdx - len(ctxOldBefore)) + 1
		if newStart < 1 {
			newStart = 1
		}

		hunks = append(hunks, diffHunk{
			oldStart: oldStart,
			newStart: newStart,
			lines:    lines,
		})
	}

	return hunks
}

// lcsItem represents a matched line in the LCS with its index.
type lcsItem struct {
	Line string
}

// longestCommonSubsequence computes the LCS of two line slices.
func longestCommonSubsequence(a, b []string) []lcsItem {
	m, n := len(a), len(b)
	// Limit LCS computation for large files
	if m > 1000 || n > 1000 {
		// Fallback: just return empty — diff will show everything as changed
		return nil
	}

	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else {
				dp[i][j] = max(dp[i-1][j], dp[i][j-1])
			}
		}
	}

	// Backtrack
	var result []lcsItem
	i, j := m, n
	for i > 0 && j > 0 {
		if a[i-1] == b[j-1] {
			result = append([]lcsItem{{Line: a[i-1]}}, result...)
			i--
			j--
		} else if dp[i-1][j] > dp[i][j-1] {
			i--
		} else {
			j--
		}
	}

	return result
}

func findLine(lines []string, target string, start int) int {
	for i := start; i < len(lines); i++ {
		if lines[i] == target {
			return i
		}
	}
	return len(lines)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ── Format Confirmation Message ──────────────────────────────────────────────

// FormatConfirmationMessage returns a Telegram-formatted confirmation message.
func FormatConfirmationMessage(edit *PendingEdit, diff string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📝 Proposed edit to `%s`\n\n", edit.FilePath))

	if edit.Reason != "" {
		sb.WriteString(fmt.Sprintf("Reason: %s\n\n", edit.Reason))
	}

	sb.WriteString("Changes:\n")
	sb.WriteString("```\n")
	sb.WriteString(diff)
	sb.WriteString("\n```\n\n")
	sb.WriteString("Reply \"yes\" or \"ok\" to apply, or \"no\" / \"cancel\" to discard.")

	return sb.String()
}

// ── Apply Edit ───────────────────────────────────────────────────────────────

// ApplyEdit applies a pending edit after user approval.
func (cm *confirmationManager) ApplyEdit(sessionID string) error {
	cm.mu.Lock()
	edit, exists := cm.pending[sessionID]
	if !exists {
		cm.mu.Unlock()
		return fmt.Errorf("no pending edit for this session")
	}

	// Check expiry
	if edit.IsExpired() {
		delete(cm.pending, sessionID)
		cm.mu.Unlock()
		// Also clean up from store
		_, _ = cm.store.ExpirePendingEdits(sessionID)
		return fmt.Errorf("edit expired")
	}
	cm.mu.Unlock()

	// Resolve absolute path
	absPath, err := safeFilePath(cm.proj, edit.FilePath)
	if err != nil {
		return err
	}

	// Create parent directories if needed
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}

	// Write the content to file
	if err := os.WriteFile(absPath, []byte(edit.Content), 0o644); err != nil {
		return fmt.Errorf("write file %s: %w", absPath, err)
	}

	// Clean up
	cm.mu.Lock()
	delete(cm.pending, sessionID)
	cm.mu.Unlock()

	// Delete from store (best effort — the edit was already applied)
	if edit.ID > 0 {
		_ = cm.store.DeletePendingEdit(edit.ID)
	}

	return nil
}

// ── Reject Edit ──────────────────────────────────────────────────────────────

// RejectEdit discards a pending edit.
func (cm *confirmationManager) RejectEdit(sessionID string) error {
	cm.mu.Lock()
	edit, exists := cm.pending[sessionID]
	if !exists {
		cm.mu.Unlock()
		return fmt.Errorf("no pending edit for this session")
	}

	editID := edit.ID
	delete(cm.pending, sessionID)
	cm.mu.Unlock()

	// Delete from store
	if editID > 0 {
		return cm.store.DeletePendingEdit(editID)
	}

	return nil
}

// ── Check Pending Edit ───────────────────────────────────────────────────────

// HasPendingEdit returns true if there's an unexpired pending edit for the session.
func (cm *confirmationManager) HasPendingEdit(sessionID string) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	edit, exists := cm.pending[sessionID]
	if !exists {
		return false
	}

	// Check expiry — if expired, clean it up
	if edit.IsExpired() {
		return false
	}

	return true
}

// ── Get Pending Edit ─────────────────────────────────────────────────────────

// GetPendingEdit returns the pending edit for the session, or nil if none or expired.
func (cm *confirmationManager) GetPendingEdit(sessionID string) *PendingEdit {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	edit, exists := cm.pending[sessionID]
	if !exists {
		return nil
	}

	if edit.IsExpired() {
		return nil
	}

	// Return a copy to prevent mutation
	cp := *edit
	return &cp
}

// ── Approval/Rejection Keywords (Public) ─────────────────────────────────────

// IsApproval returns true if the text matches an approval keyword.
func IsApproval(text string) bool {
	return isApprovalKeyword(strings.ToLower(strings.TrimSpace(text)))
}

// IsRejection returns true if the text matches a rejection keyword.
func IsRejection(text string) bool {
	return isRejectionKeyword(strings.ToLower(strings.TrimSpace(text)))
}

// ── Cleanup Expired Edits ────────────────────────────────────────────────────

// CleanupExpiredEdits removes all expired pending edits from memory and store.
// Returns the count of cleaned up edits.
func (cm *confirmationManager) CleanupExpiredEdits() (int, error) {
	cm.mu.Lock()
	var toDelete []string
	for sessionID, edit := range cm.pending {
		if edit.IsExpired() {
			toDelete = append(toDelete, sessionID)
		}
	}
	for _, sessionID := range toDelete {
		delete(cm.pending, sessionID)
	}
	cm.mu.Unlock()

	count := len(toDelete)

	// Also clean up from store
	storeCount, err := cm.cleanupStoreEdits()
	if err != nil {
		return count, err
	}

	return count + int(storeCount), nil
}

// cleanupStoreEdits deletes all expired pending edits from the database.
// Returns the count of deleted rows.
func (cm *confirmationManager) cleanupStoreEdits() (int64, error) {
	cutoff := time.Now().UTC().Add(-PendingEditTTL).Format(time.RFC3339)
	res, err := cm.store.db.Exec(
		`DELETE FROM pending_edits WHERE created_at <= ?`,
		cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("chat: cleanup expired edits: %w", err)
	}
	return res.RowsAffected()
}
