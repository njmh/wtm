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

func TestDeleteRunsPostDeleteHook(t *testing.T) {
	m := startModel(t)
	gittest.WriteFile(t, filepath.Join(m.repo.Dir, config.PostDeleteName), "true\n")
	for _, name := range []string{"a", "b", "c", "d"} {
		if err := m.repo.Create(name, "main"); err != nil {
			t.Fatal(err)
		}
	}
	m = settle(t, m, m.loadWorktrees())

	// One worktree: the popover mentions the hook, which runs once the
	// worktree is gone, in the directory wtm runs in.
	m = selectName(t, m, "a")
	m, _ = press(m, "d")
	if !strings.Contains(screen(m), "Runs .wtm/post_delete.sh afterwards.") {
		t.Errorf("delete popover doesn't mention the hook:\n%s", screen(m))
	}
	checkFits(t, m)
	m, cmd := press(m, "y")
	step, ok := result(t, cmd).(deleteStepMsg)
	if !ok || step.err != nil || step.postDelete == nil {
		t.Fatalf("delete result = %+v", step)
	}
	if step.postDelete.Dir() != m.repo.Dir {
		t.Errorf("hook runs in %s", step.postDelete.Dir())
	}
	next, cmd := m.Update(step)
	m = next.(model)
	if cmd == nil || m.mode != modeBusy || !strings.Contains(screen(m), "Running .wtm/post_delete.sh for a…") {
		t.Fatalf("mode = %v, screen:\n%s", m.mode, screen(m))
	}
	next, cmd = m.Update(postDeleteDoneMsg{i: 0})
	m = settle(t, next.(model), cmd)
	if m.mode != modeList || m.status != "Deleted worktree a. Ran .wtm/post_delete.sh." {
		t.Errorf("mode = %v, status = %q", m.mode, m.status)
	}

	// A failing hook is reported, but the worktree is still deleted.
	m = selectName(t, m, "b")
	m, _ = press(m, "d")
	m, cmd = press(m, "y")
	next, _ = m.Update(result(t, cmd))
	next, cmd = next.(model).Update(postDeleteDoneMsg{i: 0, err: os.ErrInvalid})
	m = settle(t, next.(model), cmd)
	if m.mode != modeError || !strings.Contains(screen(m), "The worktree was deleted, but .wtm/post_delete.sh failed") {
		t.Errorf("hook failure not shown:\n%s", screen(m))
	}
	checkFits(t, m)
	m, _ = press(m, "esc")
	if len(m.worktrees) != 3 {
		t.Fatalf("%d worktrees left", len(m.worktrees))
	}

	// Several: the hook runs after each delete that succeeds, before the
	// next one starts. d is dirty, so it isn't deleted and has no hook.
	gittest.WriteFile(t, filepath.Join(m.repo.Path("d"), "dirty.txt"), "x\n")
	m = selectName(t, m, "c")
	m, _ = press(m, "space")
	m = selectName(t, m, "d")
	m, _ = press(m, "space", "d")
	if !strings.Contains(screen(m), "Runs .wtm/post_delete.sh after each delete.") {
		t.Errorf("delete popover doesn't mention the hook:\n%s", screen(m))
	}
	checkFits(t, m)
	m, cmd = press(m, "y")
	next, cmd = m.Update(result(t, cmd))
	m = next.(model)
	if cmd == nil || !strings.Contains(screen(m), "Running .wtm/post_delete.sh for c…") {
		t.Fatalf("screen:\n%s", screen(m))
	}
	next, cmd = m.Update(postDeleteDoneMsg{i: 0, err: os.ErrInvalid})
	m = next.(model)
	if m.mode != modeBusy || !strings.Contains(screen(m), "Deleting d… (2 of 2)") {
		t.Fatalf("mode = %v, screen:\n%s", m.mode, screen(m))
	}
	step, ok = cmd().(deleteStepMsg)
	if !ok || step.err == nil || step.postDelete != nil {
		t.Fatalf("dirty delete = %+v", step)
	}
	next, cmd = m.Update(step)
	m = settle(t, next.(model), cmd)
	s := screen(m)
	if m.mode != modeError || !strings.Contains(s, "Deleted 1 of 2 worktrees. This one wasn't:") || !strings.Contains(s, "• d: ") ||
		!strings.Contains(s, ".wtm/post_delete.sh failed after deleting this one:") || !strings.Contains(s, "• c: invalid argument") {
		t.Fatalf("mode = %v, screen:\n%s", m.mode, s)
	}
	checkFits(t, m)
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
		p := &pauseOnFailure{Script: hook.NewPostCreate(script, hook.Env{Path: dir, Branch: branch}), name: config.PostCreateName}
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
