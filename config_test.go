package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadConfig(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, configDir, "config.toml")

	// No .wtm directory: defaults.
	cfg, err := loadConfig(dir)
	if err != nil || cfg.WorktreePath != "./" || cfg.root(dir) != dir {
		t.Fatalf("default config = %+v, root %q, err %v", cfg, cfg.root(dir), err)
	}

	// An empty file also gives the defaults.
	writeFile(t, cfgPath, "")
	if cfg, err := loadConfig(dir); err != nil || cfg.root(dir) != dir {
		t.Fatalf("empty config = %+v, err %v", cfg, err)
	}

	writeFile(t, cfgPath, "# where worktrees go\nworktree_path = \"trees\"\n")
	cfg, err = loadConfig(dir)
	if err != nil || cfg.root(dir) != filepath.Join(dir, "trees") {
		t.Fatalf("config = %+v, root %q, err %v", cfg, cfg.root(dir), err)
	}

	writeFile(t, cfgPath, "worktree_path = \"../sibling\"\n")
	if cfg, _ := loadConfig(dir); cfg.root(dir) != filepath.Join(filepath.Dir(dir), "sibling") {
		t.Errorf("root = %q", cfg.root(dir))
	}

	for content, want := range map[string]string{
		"worktree_path = ":               "config.toml",
		"worktree_pth = \"trees\"":       `unknown option "worktree_pth"`,
		"worktree_path = \"\"":           "can't be empty",
		"worktree_path = 3":              "config.toml",
		"[section]\nworktree_path=\"x\"": "unknown option",
	} {
		writeFile(t, cfgPath, content)
		if _, err := loadConfig(dir); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("config %q: err = %v, want it to mention %q", content, err, want)
		}
	}
}

func TestWorktreePath(t *testing.T) {
	g := newContainer(t)
	g.Root = filepath.Join(g.Dir, "trees")

	// git creates the worktree path itself the first time.
	if err := g.Create("feature/a", "main"); err != nil {
		t.Fatal(err)
	}
	wt := find(t, g, "feature/a")
	if wt.Path != filepath.Join(g.Dir, "trees", "feature", "a") {
		t.Errorf("created at %s", wt.Path)
	}
	if name := g.Name(mustList(t, g)[0]); name != "./.bare" {
		t.Errorf("bare repo outside the worktree path is named %q", name)
	}

	if err := g.Rename(wt, "b"); err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(g.Dir, "trees", "feature")) {
		t.Error("empty parent folder left behind after rename")
	}
	if err := g.Delete(find(t, g, "b")); err != nil {
		t.Fatal(err)
	}
	if !exists(filepath.Join(g.Dir, "trees")) {
		t.Error("the worktree path itself was removed")
	}

	// A worktree created outside the worktree path is moved into it by rename.
	sh(t, g.Dir, "git", "worktree", "add", "-q", "-b", "old", "./old", "main")
	old := find(t, g, "./old")
	if _, ok := g.InRoot(old); ok {
		t.Fatal("./old should be outside the worktree path")
	}
	if err := g.Rename(old, "old"); err != nil {
		t.Fatal(err)
	}
	if find(t, g, "old").Path != filepath.Join(g.Dir, "trees", "old") || exists(filepath.Join(g.Dir, "old")) {
		t.Error("rename didn't move the worktree into the worktree path")
	}
}

func TestPostCreateHook(t *testing.T) {
	g := newContainer(t)
	if postCreateScript(g.Dir) != "" {
		t.Fatal("found a post_create.sh that doesn't exist")
	}
	script := filepath.Join(g.Dir, configDir, "post_create.sh")
	// Not executable: it's run with bash.
	writeFile(t, script, `echo "pwd=$(pwd)"
echo "dir=$WTM_DIR path=$WTM_WORKTREE_PATH branch=$WTM_BRANCH base=$WTM_BASE_BRANCH"
[ "$WTM_BRANCH" != fail ] || exit 3
`)
	if postCreateScript(g.Dir) != script {
		t.Fatal("post_create.sh not found")
	}
	if err := g.Create("feat", "main"); err != nil {
		t.Fatal(err)
	}

	run := func(branch, stdin string) (string, error) {
		h := newPostCreateHook(script, g, g.Path("feat"), branch, "main")
		var out bytes.Buffer
		h.SetStdin(strings.NewReader(stdin))
		h.SetStdout(&out)
		h.SetStderr(&out)
		err := h.Run()
		return out.String(), err
	}

	out, err := run("feat", "")
	if err != nil {
		t.Fatalf("hook failed: %v\n%s", err, out)
	}
	path := g.Path("feat")
	want := "pwd=" + path + "\ndir=" + g.Dir + " path=" + path + " branch=feat base=main\n"
	if out != want {
		t.Errorf("hook output:\n%s\nwant:\n%s", out, want)
	}

	// A failing script waits for Enter so its output can be read.
	out, err = run("fail", "x\n")
	if err == nil || !strings.Contains(out, "post_create.sh failed (exit status 3). Press Enter") {
		t.Errorf("err = %v, output:\n%s", err, out)
	}
}

func TestCreateRunsPostCreateHook(t *testing.T) {
	m := startModel(t)
	writeFile(t, filepath.Join(m.git.Dir, configDir, "post_create.sh"), "true\n")

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
	if msg.postCreate.cmd.Dir != m.git.Path("feat") {
		t.Errorf("hook runs in %s", msg.postCreate.cmd.Dir)
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
	if wt, _ := m.selected(); m.git.Name(wt) != "x" || wt.Path != filepath.Join(g.Dir, "trees", "x") {
		t.Errorf("selected %+v", wt)
	}
	checkFits(t, m)
}
