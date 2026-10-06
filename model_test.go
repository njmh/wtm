package main

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func key(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	}
	r := []rune(s)[0]
	return tea.KeyPressMsg{Code: r, Text: s}
}

// press sends keys, discarding returned commands (e.g. cursor blinks), and
// returns the command from the last key.
func press(m model, keys ...string) (model, tea.Cmd) {
	var cmd tea.Cmd
	for _, k := range keys {
		var next tea.Model
		next, cmd = m.Update(key(k))
		m = next.(model)
	}
	return m, cmd
}

func typeText(m model, s string) model {
	for _, r := range s {
		m, _ = press(m, string(r))
	}
	return m
}

// settle runs a git operation command and the list reload that follows it.
func settle(t *testing.T, m model, cmd tea.Cmd) model {
	t.Helper()
	for cmd != nil {
		msg := cmd()
		switch msg.(type) {
		case opDoneMsg, worktreesMsg:
		default:
			t.Fatalf("unexpected message %T", msg)
		}
		var next tea.Model
		next, cmd = m.Update(msg)
		m = next.(model)
	}
	return m
}

func startModel(t *testing.T) model {
	t.Helper()
	m := newModel(newContainer(t))
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return settle(t, next.(model), m.Init())
}

func screen(m model) string {
	return ansi.Strip(m.View().Content)
}

func checkFits(t *testing.T, m model) {
	t.Helper()
	lines := strings.Split(m.View().Content, "\n")
	if len(lines) > m.height {
		t.Errorf("view has %d lines, screen has %d", len(lines), m.height)
	}
	for _, l := range lines {
		if w := ansi.StringWidth(l); w > m.width {
			t.Errorf("line is %d wide, screen is %d: %q", w, m.width, ansi.Strip(l))
		}
	}
}

func TestCreateRenameDeleteFlow(t *testing.T) {
	m := startModel(t)
	if len(m.worktrees) != 1 {
		t.Fatalf("worktrees = %+v", m.worktrees)
	}
	for _, s := range []string{"Create new worktree", "Delete selected worktree", "Rename selected worktree", "Prune worktrees", ".bare"} {
		if !strings.Contains(screen(m), s) {
			t.Errorf("main screen is missing %q", s)
		}
	}
	checkFits(t, m)

	// Create.
	m, _ = press(m, "c")
	if m.mode != modeCreate || m.baseInput.Value() != "main" {
		t.Fatalf("mode = %v, base = %q", m.mode, m.baseInput.Value())
	}
	m = typeText(m, "feat")
	if !strings.Contains(screen(m), "Create worktree") {
		t.Error("create popover not shown")
	}
	checkFits(t, m)
	m, cmd := press(m, "enter")
	m = settle(t, m, cmd)
	if m.mode != modeList || len(m.worktrees) != 2 {
		t.Fatalf("after create: mode = %v, err = %v, worktrees = %d", m.mode, m.err, len(m.worktrees))
	}
	if wt, _ := m.selected(); wt.Branch != "feat" {
		t.Errorf("selected %+v, want the new worktree", wt)
	}
	if !strings.Contains(screen(m), "Created worktree feat") {
		t.Error("no success status")
	}

	// Rename.
	m, _ = press(m, "r")
	if m.mode != modeRename || m.renameInput.Value() != "feat" {
		t.Fatalf("mode = %v, value = %q", m.mode, m.renameInput.Value())
	}
	checkFits(t, m)
	m, _ = press(m, "backspace", "backspace", "backspace", "backspace")
	m = typeText(m, "better")
	m, cmd = press(m, "enter")
	m = settle(t, m, cmd)
	if wt, _ := m.selected(); wt.Branch != "better" || m.git.Name(wt) != "better" {
		t.Fatalf("after rename selected %+v (err %v)", wt, m.err)
	}

	// Delete: cancel first, then confirm.
	m, _ = press(m, "d")
	if m.mode != modeDelete {
		t.Fatalf("mode = %v", m.mode)
	}
	checkFits(t, m)
	m, _ = press(m, "n")
	if m.mode != modeList || len(m.worktrees) != 2 {
		t.Fatal("cancel did not return to the list")
	}
	m, _ = press(m, "d")
	m, cmd = press(m, "y")
	m = settle(t, m, cmd)
	if len(m.worktrees) != 1 || m.mode != modeList {
		t.Fatalf("after delete: %d worktrees, mode %v, err %v", len(m.worktrees), m.mode, m.err)
	}
}

func TestErrorsShowInModal(t *testing.T) {
	m := startModel(t)

	// The bare repository can't be deleted or renamed.
	for _, k := range []string{"d", "r"} {
		m, _ = press(m, k)
		if m.mode != modeError || !strings.Contains(screen(m), "can't be") {
			t.Fatalf("%s on main: mode = %v", k, m.mode)
		}
		checkFits(t, m)
		m, _ = press(m, "esc")
		if m.mode != modeList {
			t.Fatal("esc did not dismiss the error")
		}
	}

	// A git failure is reported with git's message.
	m, _ = press(m, "c")
	m = typeText(m, "x")
	m, _ = press(m, "tab", "backspace", "backspace", "backspace", "backspace")
	m = typeText(m, "nope")
	m, cmd := press(m, "enter")
	m = settle(t, m, cmd)
	if m.mode != modeError || !strings.Contains(screen(m), "nope") {
		t.Fatalf("mode = %v, screen:\n%s", m.mode, screen(m))
	}
	checkFits(t, m)

	// An empty name is caught inline without running git.
	m, _ = press(m, "enter", "c")
	m, _ = press(m, "enter")
	if m.busy || m.mode != modeCreate || !strings.Contains(screen(m), "Enter a name") {
		t.Fatal("empty name was not rejected inline")
	}
}

func TestPruneKey(t *testing.T) {
	m := startModel(t)
	m, cmd := press(m, "p")
	if !m.busy {
		t.Fatal("not busy while pruning")
	}
	m = settle(t, m, cmd)
	if m.busy || !strings.Contains(screen(m), "Nothing to prune") {
		t.Fatalf("status = %q", m.status)
	}
}

func TestScrolling(t *testing.T) {
	m := startModel(t)
	for i := range 30 {
		m.worktrees = append(m.worktrees, Worktree{Path: m.git.Dir + "/wt" + string(rune('a'+i%26)) + string(rune('a'+i/26)), Branch: "b"})
	}
	for range 30 {
		m, _ = press(m, "down")
	}
	if m.cursor != 30 {
		t.Fatalf("cursor = %d", m.cursor)
	}
	if !strings.Contains(screen(m), "(31/31)") {
		t.Error("position indicator missing")
	}
	checkFits(t, m)
	for range 40 {
		m, _ = press(m, "up")
	}
	if m.cursor != 0 || m.offset != 0 {
		t.Errorf("cursor = %d, offset = %d", m.cursor, m.offset)
	}
}

func TestNotARepoShowsError(t *testing.T) {
	m := newModel(Git{Dir: t.TempDir()})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	m = settle(t, next.(model), m.Init())
	if m.mode != modeError || !strings.Contains(screen(m), "not a git repository") {
		t.Fatalf("screen:\n%s", screen(m))
	}
	checkFits(t, m)
}
