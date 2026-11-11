// Package hook runs the user's .wtm/post_create.sh and .wtm/post_delete.sh
// scripts.
package hook

import (
	"io"
	"os"
	"os/exec"
)

// Env describes the worktree that was created or deleted to the script, as
// WTM_* variables.
type Env struct {
	Dir    string // the directory wtm runs in
	Root   string // where new worktrees go
	Name   string // the worktree's name, relative to Root
	Path   string // the worktree's absolute path
	Branch string // the worktree's branch; "" for a deleted detached worktree
	Base   string // the branch it was created from; "" after a delete
}

// Vars is the environment the script is given on top of wtm's own, as
// "NAME=value" strings. Every variable is always set, even when it's empty,
// so a script never sees a value inherited from wtm's own environment.
func (e Env) Vars() []string {
	return []string{
		"WTM_DIR=" + e.Dir,
		"WTM_WORKTREE_ROOT=" + e.Root,
		"WTM_WORKTREE_NAME=" + e.Name,
		"WTM_WORKTREE_PATH=" + e.Path,
		"WTM_BRANCH=" + e.Branch,
		"WTM_BASE_BRANCH=" + e.Base,
	}
}

// Script runs one of the hook scripts with bash. It implements
// tea.ExecCommand, so the TUI can hand it the terminal (see tea.Exec); the
// CLI connects it to its own stdin and stdout. Either way the script's output
// is shown as it runs and it can start interactive programs.
type Script struct {
	cmd *exec.Cmd
}

// NewPostCreate prepares the post_create.sh at script to run inside the new
// worktree, env.Path.
func NewPostCreate(script string, env Env) *Script {
	return newScript(script, env.Path, env)
}

// NewPostDelete prepares the post_delete.sh at script. The worktree is gone
// by the time it runs, so it runs in env.Dir, the directory wtm runs in.
func NewPostDelete(script string, env Env) *Script {
	return newScript(script, env.Dir, env)
}

func newScript(script, dir string, env Env) *Script {
	cmd := exec.Command("bash", script)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env.Vars()...)
	return &Script{cmd: cmd}
}

// Dir is the directory the script runs in.
func (h *Script) Dir() string { return h.cmd.Dir }

func (h *Script) SetStdin(r io.Reader)  { h.cmd.Stdin = r }
func (h *Script) SetStdout(w io.Writer) { h.cmd.Stdout = w }
func (h *Script) SetStderr(w io.Writer) { h.cmd.Stderr = w }

// Run runs the script and waits for it to finish.
func (h *Script) Run() error {
	return h.cmd.Run()
}
