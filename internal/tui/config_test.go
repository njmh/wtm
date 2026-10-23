package tui

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/njmh/wtm/internal/config"
	"github.com/njmh/wtm/internal/gittest"
	"github.com/njmh/wtm/internal/hook"
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
	msg, ok := result(t, cmd).(opDoneMsg)
	if !ok || msg.err != nil || msg.postCreate == nil {
		t.Fatalf("create result = %+v", msg)
	}
	if msg.postCreate.Dir() != m.repo.Path("feat") {
		t.Errorf("hook runs in %s", msg.postCreate.Dir())
	}

	next, cmd := m.Update(msg)
	m = next.(model)
	if cmd == nil || m.mode != modeBusy || !strings.Contains(screen(m), "Running .wtm/post_create.sh") {
		t.Fatalf("mode = %v, screen:\n%s", m.mode, screen(m))
	}

	next, _ = m.Update(postCreateDoneMsg{status: msg.status})
	m = next.(model)
	if m.mode != modeList || m.status != "Created worktree feat from main. Ran .wtm/post_create.sh." {
		t.Errorf("mode = %v, status = %q", m.mode, m.status)
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
	if msg := result(t, cmd).(opDoneMsg); msg.err == nil || msg.postCreate != nil {
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

func TestHookPausesOnFailure(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "post_create.sh")
	gittest.WriteFile(t, script, "echo $WTM_BRANCH\n[ \"$WTM_BRANCH\" != fail ] || exit 3\n")

	run := func(branch string) (string, error) {
		p := &pauseOnFailure{PostCreate: hook.NewPostCreate(script, hook.Env{Path: dir, Branch: branch})}
		var out bytes.Buffer
		p.SetStdin(strings.NewReader("\n"))
		p.SetStdout(&out)
		p.SetStderr(&out)
		err := p.Run()
		return out.String(), err
	}

	if out, err := run("ok"); err != nil || out != "ok\n" {
		t.Errorf("success: err = %v, output %q", err, out)
	}
	// A failing script waits for Enter so its output can be read.
	out, err := run("fail")
	if err == nil || out != "fail\n\nwtm: .wtm/post_create.sh failed (exit status 3). Press Enter to return to wtm." {
		t.Errorf("failure: err = %v, output %q", err, out)
	}
}
