package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// newContainer builds the layout wtm is meant for: a directory holding a bare
// repository in .bare, pointed to by a .git file, with worktrees as siblings.
func newContainer(t *testing.T) Git {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	origin := filepath.Join(root, "origin")
	dir := filepath.Join(root, "container")

	sh(t, root, "git", "init", "-q", "-b", "main", origin)
	sh(t, origin, "git", "commit", "-q", "--allow-empty", "-m", "initial")
	sh(t, root, "git", "clone", "-q", "--bare", origin, filepath.Join(dir, ".bare"))
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: ./.bare\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return Git{Dir: dir}
}

func sh(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
	return string(out)
}

func mustList(t *testing.T, g Git) []Worktree {
	t.Helper()
	list, err := g.List()
	if err != nil {
		t.Fatal(err)
	}
	return list
}

func find(t *testing.T, g Git, name string) Worktree {
	t.Helper()
	for _, wt := range mustList(t, g) {
		if g.Name(wt) == name {
			return wt
		}
	}
	t.Fatalf("worktree %q not found", name)
	return Worktree{}
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func TestParseWorktrees(t *testing.T) {
	out := `worktree /repo/.bare
bare

worktree /repo/feat
HEAD 1234567890abcdef
branch refs/heads/feature/x
locked

worktree /repo/gone
HEAD abcdef
detached
prunable gitdir file points to non-existent location
`
	list := parseWorktrees(out)
	if len(list) != 3 {
		t.Fatalf("got %d worktrees, want 3", len(list))
	}
	if !list[0].Main || !list[0].Bare {
		t.Errorf("first entry = %+v, want main and bare", list[0])
	}
	if list[1].Branch != "feature/x" || !list[1].Locked || list[1].Main {
		t.Errorf("second entry = %+v", list[1])
	}
	if !list[2].Detached || !list[2].Prunable || list[2].Branch != "" {
		t.Errorf("third entry = %+v", list[2])
	}
}

func TestListContainer(t *testing.T) {
	g := newContainer(t)
	list := mustList(t, g)
	if len(list) != 1 || !list[0].Bare || g.Name(list[0]) != ".bare" {
		t.Fatalf("list = %+v", list)
	}
}

func TestCreate(t *testing.T) {
	g := newContainer(t)

	if err := g.Create("feat-a", "main"); err != nil {
		t.Fatal(err)
	}
	wt := find(t, g, "feat-a")
	if wt.Branch != "feat-a" || wt.Path != filepath.Join(g.Dir, "feat-a") {
		t.Errorf("created %+v", wt)
	}

	if err := g.Create("feature/b", ""); err != nil {
		t.Fatal(err)
	}
	if find(t, g, "feature/b").Branch != "feature/b" {
		t.Error("nested worktree has wrong branch")
	}

	// Based on a branch other than main.
	sh(t, filepath.Join(g.Dir, "feat-a"), "git", "commit", "-q", "--allow-empty", "-m", "on a")
	if err := g.Create("from-a", "feat-a"); err != nil {
		t.Fatal(err)
	}
	if find(t, g, "from-a").Head != find(t, g, "feat-a").Head {
		t.Error("from-a is not based on feat-a")
	}
}

func TestCreateErrors(t *testing.T) {
	g := newContainer(t)
	for _, name := range []string{"", "bad name", "-x", "a..b", "@{-1}"} {
		if err := g.Create(name, "main"); err == nil {
			t.Errorf("Create(%q) succeeded, want error", name)
		}
	}
	err := g.Create("ok", "no-such-branch")
	if err == nil || !strings.Contains(err.Error(), "no-such-branch") {
		t.Errorf("bad base: err = %v", err)
	}
	if err := g.Create("dup", "main"); err != nil {
		t.Fatal(err)
	}
	if err := g.Create("dup", "main"); err == nil {
		t.Error("duplicate create succeeded")
	}
}

func TestRename(t *testing.T) {
	g := newContainer(t)
	if err := g.Create("old", "main"); err != nil {
		t.Fatal(err)
	}
	if err := g.Rename(find(t, g, "old"), "new"); err != nil {
		t.Fatal(err)
	}
	wt := find(t, g, "new")
	if wt.Branch != "new" {
		t.Errorf("branch = %q, want new", wt.Branch)
	}
	if exists(filepath.Join(g.Dir, "old")) || !exists(filepath.Join(g.Dir, "new")) {
		t.Error("directory was not moved")
	}
	if g.branchExists("old") {
		t.Error("old branch still exists")
	}
	// The moved worktree is still a working checkout of the renamed branch.
	if got := strings.TrimSpace(sh(t, wt.Path, "git", "branch", "--show-current")); got != "new" {
		t.Errorf("checked out branch = %q", got)
	}
}

func TestRenameNested(t *testing.T) {
	g := newContainer(t)
	if err := g.Create("feature/x", "main"); err != nil {
		t.Fatal(err)
	}
	if err := g.Rename(find(t, g, "feature/x"), "fix/y"); err != nil {
		t.Fatal(err)
	}
	find(t, g, "fix/y")
	if exists(filepath.Join(g.Dir, "feature")) {
		t.Error("empty parent directory was left behind")
	}
}

func TestRenameConflicts(t *testing.T) {
	g := newContainer(t)
	for _, n := range []string{"a", "b"} {
		if err := g.Create(n, "main"); err != nil {
			t.Fatal(err)
		}
	}
	sh(t, g.Dir, "git", "branch", "taken")

	a := find(t, g, "a")
	for _, name := range []string{"b", "taken", "bad name"} {
		if err := g.Rename(a, name); err == nil {
			t.Errorf("Rename to %q succeeded, want error", name)
		}
	}
	// Nothing changed.
	if wt := find(t, g, "a"); wt.Branch != "a" {
		t.Errorf("a changed: %+v", wt)
	}
	if err := g.Rename(mustList(t, g)[0], "x"); err == nil {
		t.Error("renaming the main worktree succeeded")
	}
}

func TestDelete(t *testing.T) {
	g := newContainer(t)
	if err := g.Create("feature/gone", "main"); err != nil {
		t.Fatal(err)
	}
	wt := find(t, g, "feature/gone")
	if err := g.Delete(wt); err != nil {
		t.Fatal(err)
	}
	if exists(wt.Path) || exists(filepath.Join(g.Dir, "feature")) {
		t.Error("directory still exists")
	}
	if g.branchExists("feature/gone") {
		t.Error("branch still exists")
	}
	if len(mustList(t, g)) != 1 {
		t.Error("worktree still listed")
	}
	if err := g.Delete(mustList(t, g)[0]); err == nil {
		t.Error("deleting the main worktree succeeded")
	}
}

func TestDeleteRefusesToLoseWork(t *testing.T) {
	g := newContainer(t)
	if err := g.Create("dirty", "main"); err != nil {
		t.Fatal(err)
	}
	wt := find(t, g, "dirty")

	// Uncommitted file.
	if err := os.WriteFile(filepath.Join(wt.Path, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := g.Delete(wt); err == nil {
		t.Fatal("deleted a worktree with untracked files")
	}
	if !exists(wt.Path) || !g.branchExists("dirty") {
		t.Fatal("worktree or branch was removed")
	}

	// Committed, but not merged into main.
	sh(t, wt.Path, "git", "add", ".")
	sh(t, wt.Path, "git", "commit", "-q", "-m", "work")
	err := g.Delete(wt)
	if err == nil || !strings.Contains(err.Error(), "aren't merged into main") {
		t.Fatalf("err = %v, want unmerged error", err)
	}
	if !exists(wt.Path) || !g.branchExists("dirty") {
		t.Fatal("worktree or branch was removed")
	}
}

func TestPruneAndDeletePrunable(t *testing.T) {
	g := newContainer(t)
	for _, n := range []string{"p1", "p2"} {
		if err := g.Create(n, "main"); err != nil {
			t.Fatal(err)
		}
		if err := os.RemoveAll(filepath.Join(g.Dir, n)); err != nil {
			t.Fatal(err)
		}
	}
	p1 := find(t, g, "p1")
	if !p1.Prunable {
		t.Fatalf("p1 not prunable: %+v", p1)
	}

	// Deleting a worktree whose directory is gone still cleans up the branch.
	if err := g.Delete(p1); err != nil {
		t.Fatal(err)
	}
	if g.branchExists("p1") {
		t.Error("p1 branch still exists")
	}
	if !find(t, g, "p2").Prunable {
		t.Error("deleting p1 touched p2")
	}

	n, err := g.Prune()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("pruned %d, want 1", n)
	}
	if len(mustList(t, g)) != 1 {
		t.Errorf("list after prune = %+v", mustList(t, g))
	}
	if n, err := g.Prune(); err != nil || n != 0 {
		t.Errorf("second prune = %d, %v", n, err)
	}
}

func TestNotARepo(t *testing.T) {
	_, err := Git{Dir: t.TempDir()}.List()
	if err == nil || !strings.Contains(err.Error(), "not a git repository") {
		t.Fatalf("err = %v", err)
	}
}
