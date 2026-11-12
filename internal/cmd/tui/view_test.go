// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestViewHelperFunctions(t *testing.T) {
	// Test renderWithMargin
	marginStr := renderWithMargin("content")
	if !strings.Contains(marginStr, "content") {
		t.Errorf("expected margin content, got %q", marginStr)
	}

	// Test renderFooter standard & success
	footerStr := renderFooter("footer-text")
	if !strings.Contains(footerStr, "footer-text") {
		t.Errorf("expected footer text, got %q", footerStr)
	}

	successFooter := renderFooter("Changes staged successfully!")
	if !strings.Contains(successFooter, "✓") || !strings.Contains(successFooter, "Changes staged successfully!") {
		t.Errorf("expected green checkmark in success footer, got %q", successFooter)
	}

	if !isSuccessMessage("Policy initialized successfully.") || !isSuccessMessage("Root key added!") {
		t.Error("expected isSuccessMessage to return true for success texts")
	}
	if isSuccessMessage("Read-only mode") || isSuccessMessage("No action selected.") {
		t.Error("expected isSuccessMessage to return false for standard info texts")
	}

	// Test renderErrorMsg
	if renderErrorMsg("") != "" {
		t.Error("expected empty string for empty errorMsg")
	}
	errMsg := renderErrorMsg("some error")
	if !strings.Contains(errMsg, "some error") {
		t.Errorf("expected error message formatted, got %q", errMsg)
	}

	// Test renderDeleteOverlay
	delRuleOverlay := renderDeleteOverlay("rule", "test-target")
	if !strings.Contains(delRuleOverlay, "Delete rule \"test-target\"? [y/n]") {
		t.Errorf("expected delete rule overlay string, got %q", delRuleOverlay)
	}
	delPrincipalOverlay := renderDeleteOverlay("principal", "test-principal")
	if !strings.Contains(delPrincipalOverlay, "Delete principal \"test-principal\"? [y/n]") {
		t.Errorf("expected delete principal overlay string, got %q", delPrincipalOverlay)
	}
	delGlobalOverlay := renderDeleteOverlay("global rule", "test-gr")
	if !strings.Contains(delGlobalOverlay, "Delete global rule \"test-gr\"? [y/n]") {
		t.Errorf("expected delete global rule overlay string, got %q", delGlobalOverlay)
	}
	delDefaultOverlay := renderDeleteOverlay("", "test-default")
	if !strings.Contains(delDefaultOverlay, "Delete rule \"test-default\"? [y/n]") {
		t.Errorf("expected default to 'rule', got %q", delDefaultOverlay)
	}

	// Test renderActionHints for readOnly vs edit mode
	hintsReadOnly := renderActionHints(true)
	if !strings.Contains(hintsReadOnly, "help") || strings.Contains(hintsReadOnly, "add") {
		t.Errorf("unexpected readOnly action hints: %q", hintsReadOnly)
	}

	hintsEdit := renderActionHints(false)
	if !strings.Contains(hintsEdit, "add") || !strings.Contains(hintsEdit, "edit") {
		t.Errorf("unexpected edit mode action hints: %q", hintsEdit)
	}
}

func TestDiffOverlayRendering(t *testing.T) {
	tmpDir := t.TempDir()
	currentDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(currentDir) //nolint:errcheck
	gitinterface.CreateTestGitRepository(t, tmpDir, false)

	o := &options{
		readOnly:  true,
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: "dummy-key"},
	}

	m := initialModel(context.Background(), o)
	m.width = 80
	m.height = 24

	if renderDiffOverlay(m) != "" {
		t.Error("expected empty string when showDiffOverlay is false")
	}

	m.toggleDiffOverlay()
	diffStr := renderDiffOverlay(m)
	if !strings.Contains(diffStr, "Staged Policy & Trust Changes") {
		t.Errorf("expected diff overlay header, got %q", diffStr)
	}

	m.toggleDiffOverlay()
	if m.showDiffOverlay {
		t.Error("expected showDiffOverlay to be false after toggle")
	}
}

func TestGenerateStagedDiffEqualTips(t *testing.T) {
	tmpDir := t.TempDir()
	currentDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(currentDir) //nolint:errcheck
	gitinterface.CreateTestGitRepository(t, tmpDir, false)

	o := &options{
		readOnly:  true,
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: "dummy-key"},
	}

	m := initialModel(context.Background(), o)
	diff := m.generateStagedDiff()
	if !strings.Contains(diff, "No staged policy changes detected") {
		t.Errorf("expected 'No staged policy changes detected', got %q", diff)
	}
}

func TestRenderFooterBoxVariants(t *testing.T) {
	o := &options{
		readOnly:  true,
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: "dummy-key"},
	}

	m := initialModel(context.Background(), o)
	m.readOnly = true
	m.showHelp = true
	m.signerError = "signer unavailable"

	// Read-only mode with showHelp and signerError
	boxStr := renderFooterBox(m)
	if !strings.Contains(boxStr, "Read-only mode") || !strings.Contains(boxStr, "signer unavailable") {
		t.Errorf("expected read-only help box with signer error, got %q", boxStr)
	}

	// Read-only mode without showHelp but with signerError
	m.showHelp = false
	boxStr = renderFooterBox(m)
	if !strings.Contains(boxStr, "Notice: signer") {
		t.Errorf("expected signer notice in footer, got %q", boxStr)
	}
}

func TestRenderErrorAndPopupDialog(t *testing.T) {
	o := &options{
		readOnly:  false,
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: "dummy-key"},
	}

	m := initialModel(context.Background(), o)
	m.width = 100

	// When no error dialog present
	if renderErrorDialog(m) != "" || renderPopupDialog(m) != "" {
		t.Error("expected empty string when no error dialog is present")
	}

	// Open error dialog
	m.openErrorDialog("Fatal Error", "Something went wrong")

	dialogStr := renderPopupDialog(m)
	if !strings.Contains(dialogStr, "Fatal Error") || !strings.Contains(dialogStr, "Something went wrong") {
		t.Errorf("expected dialog with title and message, got %q", dialogStr)
	}

	// Test small width error dialog boundary
	m.width = 10
	dialogSmallStr := renderErrorDialog(m)
	if !strings.Contains(dialogSmallStr, "Fatal Error") {
		t.Errorf("expected small dialog, got %q", dialogSmallStr)
	}
}

func TestRenderStatusBar(t *testing.T) {
	// Width 0 fallback (defaults to 80)
	barStr := renderStatusBar("TestScreen", false, 0)
	if !strings.Contains(barStr, "TestScreen") || !strings.Contains(barStr, "Edit Mode") {
		t.Errorf("expected status bar with Edit Mode, got %q", barStr)
	}

	// Read-only status bar
	barReadOnly := renderStatusBar("TestScreen", true, 100)
	if !strings.Contains(barReadOnly, "Read-only") {
		t.Errorf("expected status bar with Read-only, got %q", barReadOnly)
	}
}

func TestRenderListOrEmpty(t *testing.T) {
	o := &options{
		readOnly:  true,
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: "dummy-key"},
	}

	m := initialModel(context.Background(), o)
	m.readOnly = true
	m.signerError = "key error"
	m.width = 80
	m.height = 24

	l := list.New([]list.Item{item{title: "item-1"}}, list.NewDefaultDelegate(), 80, 20)

	// Non-empty list returns l.View()
	viewStr := m.renderListOrEmpty(l, 1, "No items")
	if !strings.Contains(viewStr, "item-1") {
		t.Errorf("expected item-1 in list view, got %q", viewStr)
	}

	// Empty list returns placeholder
	emptyView := m.renderListOrEmpty(l, 0, "No items found")
	if !strings.Contains(emptyView, "No items found") {
		t.Errorf("expected empty state text, got %q", emptyView)
	}
}

func TestModelViewScreenStates(t *testing.T) {
	o := &options{
		readOnly:  false,
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: "dummy-key"},
	}

	m := initialModel(context.Background(), o)

	// Width/Height == 0 returns spinner loading
	m.width = 0
	m.height = 0
	viewStr := m.View()
	if !strings.Contains(viewStr, "Loading TUI...") {
		t.Errorf("expected Loading TUI... when width/height 0, got %q", viewStr)
	}

	m.width = 80
	m.height = 24

	// screenLoading without error
	m.screen = screenLoading
	viewStr = m.View()
	if !strings.Contains(viewStr, "Loading, please wait...") {
		t.Errorf("expected loading message, got %q", viewStr)
	}

	// screenLoading with error
	m.errorMsg = "Failed to load repo"
	viewStr = m.View()
	if !strings.Contains(viewStr, "Failed to load repo") {
		t.Errorf("expected errorMsg in loading screen, got %q", viewStr)
	}
	m.errorMsg = ""

	// Verifying mode
	m.verifying = true
	m.loadingMsg = "Verifying repository..."
	viewStr = m.View()
	if !strings.Contains(viewStr, "Verifying repository...") {
		t.Errorf("expected verifying message, got %q", viewStr)
	}
	m.verifying = false

	// Default unknown screen
	m.screen = screen(999)
	viewStr = m.View()
	if !strings.Contains(viewStr, "Unknown screen") {
		t.Errorf("expected 'Unknown screen' for unknown screen enum, got %q", viewStr)
	}
}

func TestWrapDiffLine(t *testing.T) {
	// Short line should not be wrapped
	short := "  - short line"
	if wrapped := wrapDiffLine(short, 50, "    "); wrapped != short {
		t.Errorf("expected %q, got %q", short, wrapped)
	}

	// Long line with spaces should be wrapped with hanging indent
	long := "  - Authorized Principals: key1, key2, key3, key4, key5"
	wrapped := wrapDiffLine(long, 30, "      ")
	lines := strings.Split(wrapped, "\n")
	assert.GreaterOrEqual(t, len(lines), 2)
	for i, l := range lines {
		assert.LessOrEqual(t, len(l), 30, "line %d exceeds maxWidth 30: %q (len %d)", i, l, len(l))
		if i > 0 {
			assert.True(t, strings.HasPrefix(l, "      "), "expected continuation line %d to have hanging indent, got %q", i, l)
		}
	}

	// Very long single word should be hard broken without exceeding maxWidth
	longWord := "ssh-ed25519-AAAAC3NzaC1lZDI1NTE5AAAAIExampleVeryLongSingleWordKeyStringThatDoesNotHaveAnySpacesInIt"
	wrappedWord := wrapDiffLine(longWord, 25, "  ")
	for i, l := range strings.Split(wrappedWord, "\n") {
		assert.LessOrEqual(t, len(l), 25, "line %d exceeds maxWidth 25: %q", i, l)
	}

	// Long first word followed by other words should break the first word without overflow
	longFirstWord := "extremelylongfirstwordthatoverflows and secondword"
	wrappedFirst := wrapDiffLine(longFirstWord, 15, "  ")
	firstLines := strings.Split(wrappedFirst, "\n")
	assert.GreaterOrEqual(t, len(firstLines), 2)
	for i, l := range firstLines {
		assert.LessOrEqual(t, len(l), 15, "line %d exceeds maxWidth 15: %q (len %d)", i, l, len(l))
	}

	// Narrow width (<= 20) should wrap rather than returning un-wrapped line
	narrowLine := "this is a sentence that wraps at narrow width"
	wrappedNarrow := wrapDiffLine(narrowLine, 10, "  ")
	narrowLines := strings.Split(wrappedNarrow, "\n")
	assert.Greater(t, len(narrowLines), 1)
	for i, l := range narrowLines {
		assert.LessOrEqual(t, len(l), 10, "line %d exceeds maxWidth 10: %q (len %d)", i, l, len(l))
	}

	// Non-positive maxWidth should return original line without changes
	assert.Equal(t, narrowLine, wrapDiffLine(narrowLine, 0, "  "))
	assert.Equal(t, narrowLine, wrapDiffLine(narrowLine, -10, "  "))
}

func TestDiffCacheHitAndInvalidationOnStagingCommit(t *testing.T) {
	tmpDir := t.TempDir()
	currentDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(currentDir) //nolint:errcheck

	gitRepo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

	treeBuilder := gitinterface.NewTreeBuilder(gitRepo)
	emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
	require.NoError(t, err)

	commit1, err := gitRepo.Commit(emptyTreeID, policy.PolicyStagingRef, "Staging commit 1\n", false)
	require.NoError(t, err)

	o := &options{
		readOnly:  false,
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: "dummy-key"},
	}
	m := initialModel(context.Background(), o)

	// Mock loaders to prevent them from failing on the empty tree commit.
	// We just want to test the caching logic in model.generateStagedDiff.
	origGetRulesForRef := getRulesForRefFn
	origGetGlobalRulesForRef := getGlobalRulesForRefFn
	origGetPrincipalsForRef := getPrincipalsForRefFn
	t.Cleanup(func() {
		getRulesForRefFn = origGetRulesForRef
		getGlobalRulesForRefFn = origGetGlobalRulesForRef
		getPrincipalsForRefFn = origGetPrincipalsForRef
	})

	getRulesForRefFn = func(_ context.Context, _ *gittuf.Repository, _ string) ([]rule, error) {
		return nil, nil
	}
	getGlobalRulesForRefFn = func(_ context.Context, _ *gittuf.Repository, _ string) ([]globalRule, error) {
		return nil, nil
	}
	getPrincipalsForRefFn = func(_ context.Context, _ *gittuf.Repository, _, _ string) ([]tuf.Principal, error) {
		return nil, nil
	}

	// First call populates cache
	diff1 := m.generateStagedDiff(80)
	assert.NotEmpty(t, diff1)
	assert.Equal(t, commit1.String(), m.cachedStagingTip)
	assert.Equal(t, 80, m.cachedDiffWidth)

	// Inject a stale sentinel to prove the cache is actually hit (not recomputed).
	// If the cache is bypassed, the real computation returns a different string and
	// the assertion below will fail, catching any regression.
	m.stagedDiffCache = "stale-sentinel-value"
	diff1Cached := m.generateStagedDiff(80)
	assert.Equal(t, "stale-sentinel-value", diff1Cached, "expected cache to be hit and return the stale sentinel")

	// Width resize invalidates cache and updates cached width
	diffResized := m.generateStagedDiff(40)
	assert.Equal(t, 40, m.cachedDiffWidth)
	assert.NotEmpty(t, diffResized)

	// New commit on policy-staging ref triggers automatic cache invalidation
	commit2, err := gitRepo.Commit(emptyTreeID, policy.PolicyStagingRef, "Staging commit 2\n", false)
	require.NoError(t, err)

	diff2 := m.generateStagedDiff(80)
	assert.Equal(t, commit2.String(), m.cachedStagingTip)
	assert.NotEmpty(t, diff2)
}

func TestSelectiveDiffHighlighting(t *testing.T) {
	// Case 1: Only authorized principals modified (pattern & threshold remain identical)
	activeRules := []rule{
		{name: "rule-1", pattern: "refs/heads/main", key: "alice", threshold: 1},
	}
	stagedRulesOnlyKeys := []rule{
		{name: "rule-1", pattern: "refs/heads/main", key: "alice, bob", threshold: 1},
	}

	diffKeys, hasChanges := formatStagedDiff(activeRules, stagedRulesOnlyKeys, nil, nil, nil, nil, 80)
	assert.True(t, hasChanges)
	assert.Contains(t, diffKeys, "~ Rule: rule-1")
	// Unchanged pattern and threshold must appear as context subtext, NOT marked with - or +
	assert.Contains(t, diffKeys, "    pattern: refs/heads/main, threshold: 1")
	assert.NotContains(t, diffKeys, "- pattern:")
	assert.NotContains(t, diffKeys, "+ pattern:")
	// Modified authorized principals must be marked with - and +
	assert.Contains(t, diffKeys, "  - Authorized Principals: alice")
	assert.Contains(t, diffKeys, "  + Authorized Principals: alice, bob")

	// Case 2: Only pattern & threshold modified (authorized principals remain identical)
	stagedRulesOnlyPattern := []rule{
		{name: "rule-1", pattern: "refs/heads/feature", key: "alice", threshold: 2},
	}

	diffPattern, hasChanges := formatStagedDiff(activeRules, stagedRulesOnlyPattern, nil, nil, nil, nil, 80)
	assert.True(t, hasChanges)
	assert.Contains(t, diffPattern, "~ Rule: rule-1")
	// Changed pattern and threshold must be marked with - and +
	assert.Contains(t, diffPattern, "  - pattern: refs/heads/main, threshold: 1")
	assert.Contains(t, diffPattern, "  + pattern: refs/heads/feature, threshold: 2")
	// Unchanged authorized principals must NOT be marked with - or +
	assert.NotContains(t, diffPattern, "Authorized Principals:")

	// Case 3: Global rules selective highlighting (only threshold modified)
	activeGRs := []globalRule{
		{ruleName: "gr-1", ruleType: "threshold", rulePatterns: []string{"main"}, threshold: 1},
	}
	stagedGRs := []globalRule{
		{ruleName: "gr-1", ruleType: "threshold", rulePatterns: []string{"main"}, threshold: 2},
	}

	diffGR, hasChanges := formatStagedDiff(nil, nil, activeGRs, stagedGRs, nil, nil, 80)
	assert.True(t, hasChanges)
	assert.Contains(t, diffGR, "~ Global Rule: gr-1")
	assert.Contains(t, diffGR, "  - threshold: 1")
	assert.Contains(t, diffGR, "  + threshold: 2")
	assert.NotContains(t, diffGR, "type:")
	assert.NotContains(t, diffGR, "patterns:")
}
