package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/njmh/wtm/internal/config"
	"github.com/njmh/wtm/internal/gittest"
)

func TestCreateRunsPostCreateHook(t *testing.T) {
	m := startModel(t)
	gittest.WriteFile(t, filepath.Join(m.repo.Dir, config.PostCreateName), "true\n")

	m, _ = press(m, "c")
	if !strings.Contains(screen(m), "Runs .wtm/post_create.sh.") {
		t.Errorf("create popover doesn't mention the hook:\n%s", screen(m))
	}
	m = typeText(m, "feat")
	_, cmd := press(m, "enter")
	msg, ok := cmd().(opDoneMsg)
	if !ok || msg.err != nil || msg.postCreate == nil {
		t.Fatalf("create result = %+v", msg)
	}
	if msg.postCreate.Dir() != m.repo.Path("feat") {
		t.Errorf("hook runs in %s", msg.postCreate.Dir())
	}

	next, cmd := m.Update(msg)
	m = next.(model)
	if cmd == nil || !m.busy || !strings.Contains(m.status, "Running .wtm/post_create.sh") {
		t.Fatalf("busy = %v, status = %q", m.busy, m.status)
	}

	next, _ = m.Update(postCreateDoneMsg{status: msg.status})
	m = next.(model)
	if m.busy || m.status != "Created worktree feat from main. Ran .wtm/post_create.sh." {
		t.Errorf("busy = %v, status = %q", m.busy, m.status)
	}

	next, _ = m.Update(postCreateDoneMsg{status: msg.status, err: os.ErrInvalid})
	m = next.(model)
	if m.mode != modeError || !strings.Contains(screen(m), "The worktree was created, but .wtm/post_create.sh failed") {
		t.Errorf("hook failure not shown:\n%s", screen(m))
	}
	checkFits(t, m)

	// The hook only runs after a successful create.
	m, _ = press(m, "esc", "c")
	m = typeText(m, "feat")
	_, cmd = press(m, "enter")
	if msg := cmd().(opDoneMsg); msg.err == nil || msg.postCreate != nil {
		t.Errorf("duplicate create = %+v", msg)
	}
}

func TestWorktreePathShown(t *testing.T) {
	g := newContainer(t)
	g.Root = filepath.Join(g.Dir, "trees")
	m := newModel(g)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = settle(t, next.(model), m.Init())

	if !strings.Contains(screen(m), "worktrees in ./trees") || !strings.Contains(screen(m), "./.bare") {
		t.Errorf("main screen:\n%s", screen(m))
	}
	m, _ = press(m, "c")
	if !strings.Contains(screen(m), "Created in ./trees/.") {
		t.Errorf("create popover:\n%s", screen(m))
	}
	m = typeText(m, "x")
	m, cmd := press(m, "enter")
	m = settle(t, m, cmd)
	if wt, _ := m.selected(); m.repo.Name(wt) != "x" || wt.Path != filepath.Join(g.Dir, "trees", "x") {
		t.Errorf("selected %+v", wt)
	}
	checkFits(t, m)
}
