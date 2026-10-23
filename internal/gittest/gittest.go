// Package gittest builds throwaway git repositories for tests.
package gittest

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// NewContainer builds the layout wtm is meant for: a directory holding a bare
// repository in .bare, pointed to by a .git file, with worktrees as siblings.
// It returns that directory.
func NewContainer(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	origin := filepath.Join(root, "origin")
	dir := filepath.Join(root, "container")

	Run(t, root, "git", "init", "-q", "-b", "main", origin)
	Run(t, origin, "git", "commit", "-q", "--allow-empty", "-m", "initial")
	Run(t, root, "git", "clone", "-q", "--bare", origin, filepath.Join(dir, ".bare"))
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: ./.bare\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// Run runs a command in dir with a fixed git identity and returns its output,
// failing the test if it fails.
func Run(t *testing.T, dir string, name string, args ...string) string {
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

// WriteFile writes content to path, creating its parent directories.
func WriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
