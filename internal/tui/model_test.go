package tui

import (
	"errors"
	"strings"
	"testing"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/njmh/wtm/internal/git"
	"github.com/njmh/wtm/internal/gittest"
)

func newContainer(t *testing.T) git.Repo {
	t.Helper()
	return git.Repo{Dir: gittest.NewContainer(t)}
}

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
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
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
// Spinner ticks are dropped, as the animation would never settle.
func settle(t *testing.T, m model, cmd tea.Cmd) model {
	t.Helper()
	queue := []tea.Cmd{cmd}
	for len(queue) > 0 {
		cmd, queue = queue[0], queue[1:]
		if cmd == nil {
			continue
		}
		msg := cmd()
		switch msg := msg.(type) {
		case tea.BatchMsg:
			queue = append(queue, msg...)
			continue
		case spinner.TickMsg:
			continue
		case opDoneMsg, worktreesMsg, deleteStepMsg:
		default:
			t.Fatalf("unexpected message %T", msg)
		}
		next, cmd := m.Update(msg)
		m = next.(model)
		queue = append(queue, cmd)
	}
	return m
}

// result runs an operation command and returns its result, without the
// spinner tick that startBusy batches with it.
func result(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	batch, ok := cmd().(tea.BatchMsg)
	if !ok {
		t.Fatal("operation was not started with startBusy")
	}
	for _, c := range batch {
		if msg := c(); msg != nil {
			if _, tick := msg.(spinner.TickMsg); !tick {
				return msg
			}
		}
	}
	t.Fatal("no operation in batch")
	return nil
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
	if wt, _ := m.selected(); wt.Branch != "better" || m.repo.Name(wt) != "better" {
		t.Fatalf("after rename selected %+v (err %v)", wt, m.err)
	}

	// Delete: cancel first, then confirm.
	m, _ = press(m, "d")
	if m.mode != modeDelete || strings.Contains(screen(m), "Warning") {
		t.Fatalf("mode = %v, screen:\n%s", m.mode, screen(m))
	}
	checkFits(t, m)
	m, _ = press(m, "n")
	if m.mode != modeList || len(m.worktrees) != 2 {
		t.Fatal("cancel did not return to the list")
	}
	// Unmerged commits are warned about, but don't stop the delete.
	wt, _ := m.selected()
	gittest.Run(t, wt.Path, "git", "commit", "-q", "--allow-empty", "-m", "work")
	m, _ = press(m, "d")
	if s := screen(m); !strings.Contains(s, "Warning: branch \"better\" has 1") {
		t.Errorf("no unmerged warning:\n%s", s)
	}
	checkFits(t, m)
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
	if m.mode != modeCreate || !strings.Contains(screen(m), "Enter a name") {
		t.Fatal("empty name was not rejected inline")
	}
}

func TestPruneKey(t *testing.T) {
	m := startModel(t)
	m, cmd := press(m, "p")
	if m.mode != modeBusy || !strings.Contains(screen(m), "Pruning worktrees…") {
		t.Fatalf("no busy popover while pruning:\n%s", screen(m))
	}
	checkFits(t, m)
	// Other actions are blocked until the operation finishes.
	for _, k := range []string{"c", "d", "r", "p", "q", "esc"} {
		if m, cmd := press(m, k); m.mode != modeBusy || cmd != nil {
			t.Fatalf("%s while busy: mode = %v, cmd = %v", k, m.mode, cmd)
		}
	}
	m = settle(t, m, cmd)
	if m.mode != modeList || !strings.Contains(screen(m), "Nothing to prune") {
		t.Fatalf("status = %q", m.status)
	}
}

func TestScrolling(t *testing.T) {
	m := startModel(t)
	for i := range 30 {
		m.worktrees = append(m.worktrees, git.Worktree{Path: m.repo.Dir + "/wt" + string(rune('a'+i%26)) + string(rune('a'+i/26)), Branch: "b"})
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
	m := newModel(git.Repo{Dir: t.TempDir()})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	m = settle(t, next.(model), m.Init())
	if m.mode != modeError || !strings.Contains(screen(m), "not a git repository") {
		t.Fatalf("screen:\n%s", screen(m))
	}
	checkFits(t, m)
}

// selectName moves the cursor to the worktree called name.
func selectName(t *testing.T, m model, name string) model {
	t.Helper()
	for i, wt := range m.worktrees {
		if m.repo.Name(wt) == name {
			m.cursor = i
			return m
		}
	}
	t.Fatalf("no worktree %q", name)
	return m
}

func TestCheckAndDeleteSeveral(t *testing.T) {
	m := startModel(t)
	for _, name := range []string{"a", "b", "c"} {
		if err := m.repo.Create(name, "main"); err != nil {
			t.Fatal(err)
		}
	}
	m = settle(t, m, m.loadWorktrees())

	// The bare repository has no checkbox and can't be checked.
	m, _ = press(m, "space")
	if len(m.checked) != 0 {
		t.Fatal("checked the bare repository")
	}
	m, _ = press(m, "esc") // nothing to clear; stays on the list
	if m.mode != modeList {
		t.Fatalf("mode = %v", m.mode)
	}

	// Space toggles the selected worktree.
	m = selectName(t, m, "a")
	m, _ = press(m, "space")
	m = selectName(t, m, "c")
	m, _ = press(m, "space", "space", "space")
	m = selectName(t, m, "b")
	if len(m.checked) != 2 || !strings.Contains(screen(m), "[x] a") || !strings.Contains(screen(m), "[ ] b") {
		t.Fatalf("checked = %v, screen:\n%s", m.checked, screen(m))
	}
	if !strings.Contains(screen(m), "Delete 2 checked worktrees") {
		t.Errorf("menu doesn't offer to delete the checked worktrees:\n%s", screen(m))
	}
	checkFits(t, m)

	// Rename needs a single worktree, so it's blocked while any are checked.
	if m, _ := press(m, "r"); m.mode != modeList {
		t.Fatalf("rename with worktrees checked: mode = %v", m.mode)
	}

	// Delete acts on the checked worktrees, not the selected one. One has
	// unmerged commits and the other uncommitted changes.
	a := m.checkedWorktrees()[0]
	gittest.Run(t, a.Path, "git", "commit", "-q", "--allow-empty", "-m", "work")
	c := m.checkedWorktrees()[1]
	gittest.WriteFile(t, c.Path+"/dirty.txt", "x\n")
	m, _ = press(m, "d")
	s := screen(m)
	if m.mode != modeDelete || !strings.Contains(s, "Delete 2 worktrees?") || !strings.Contains(s, "! a") || !strings.Contains(s, "  c  c") || strings.Contains(s, "b  b") {
		t.Fatalf("mode = %v, screen:\n%s", m.mode, s)
	}
	if !strings.Contains(s, "branches marked ! have commits") {
		t.Errorf("no unmerged warning:\n%s", s)
	}
	checkFits(t, m)

	// They're deleted one at a time, showing progress.
	m, cmd := press(m, "y")
	if !strings.Contains(screen(m), "Deleting a… (1 of 2)") {
		t.Errorf("no progress:\n%s", screen(m))
	}
	next, cmd := m.Update(result(t, cmd))
	m = next.(model)
	if m.mode != modeBusy || !strings.Contains(screen(m), "Deleting c… (2 of 2)") {
		t.Errorf("mode = %v, screen:\n%s", m.mode, screen(m))
	}
	m = settle(t, m, cmd)

	// The dirty one fails, is reported and stays checked.
	if m.mode != modeError || !strings.Contains(screen(m), "Deleted 1 of 2 worktrees. This one wasn't:") || !strings.Contains(screen(m), "• c: ") {
		t.Fatalf("mode = %v, screen:\n%s", m.mode, screen(m))
	}
	checkFits(t, m)
	m, _ = press(m, "esc")
	if len(m.worktrees) != 3 || len(m.checked) != 1 || !m.checked[c.Path] {
		t.Fatalf("worktrees = %d, checked = %v", len(m.worktrees), m.checked)
	}

	// Esc unchecks everything, and rename works again.
	m, _ = press(m, "esc")
	if len(m.checked) != 0 || strings.Contains(screen(m), "[x]") {
		t.Fatalf("esc left %v checked", m.checked)
	}
	if m, _ := press(m, "r"); m.mode != modeRename {
		t.Fatalf("rename after unchecking: mode = %v", m.mode)
	}

	// Deleting several that all succeed reports how many.
	gittest.Run(t, c.Path, "git", "clean", "-qf")
	m = selectName(t, m, "b")
	m, _ = press(m, "space")
	m = selectName(t, m, "c")
	m, _ = press(m, "space", "d")
	m, cmd = press(m, "y")
	m = settle(t, m, cmd)
	if m.mode != modeList || len(m.worktrees) != 1 || len(m.checked) != 0 || !strings.Contains(screen(m), "Deleted 2 worktrees.") {
		t.Fatalf("mode = %v, err = %v, worktrees = %d, screen:\n%s", m.mode, m.err, len(m.worktrees), screen(m))
	}
}

func TestDeleteManyFitsScreen(t *testing.T) {
	m := startModel(t)
	for i := range 30 {
		wt := git.Worktree{Path: m.repo.Dir + "/wt" + string(rune('a'+i%26)) + string(rune('a'+i/26)), Branch: "b"}
		m.worktrees = append(m.worktrees, wt)
		m.checked[wt.Path] = true
	}
	m, _ = press(m, "d")
	if m.mode != modeDelete || !strings.Contains(screen(m), "Delete 30 worktrees?") || !strings.Contains(screen(m), "more") {
		t.Fatalf("mode = %v, screen:\n%s", m.mode, screen(m))
	}
	checkFits(t, m)

	// So does the error listing every failure.
	m.deleteErrs = make([]error, len(m.deleting))
	for i := range m.deleteErrs {
		m.deleteErrs[i] = errors.New("it failed")
	}
	next, _ := m.finish(m.deleteResult())
	m = next.(model)
	if m.mode != modeError || !strings.Contains(screen(m), "Deleted 0 of 30") {
		t.Fatalf("mode = %v, screen:\n%s", m.mode, screen(m))
	}
	checkFits(t, m)
}
