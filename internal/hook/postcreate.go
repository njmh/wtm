// Package hook runs the user's .wtm/post_create.sh script.
package hook

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/njmh/wtm/internal/config"
)

// Env describes the new worktree to the script, as WTM_* variables.
type Env struct {
	Dir    string // the directory wtm runs in
	Root   string // where new worktrees go
	Name   string // the worktree's name, relative to Root
	Path   string // the worktree's absolute path; the script runs here
	Branch string // the new branch
	Base   string // the branch it was created from
}

// PostCreate runs .wtm/post_create.sh with bash inside a new worktree. It
// implements tea.ExecCommand, so it can take over the terminal (see
// tea.Exec): the script's output is shown as it runs and it can start
// interactive programs.
type PostCreate struct {
	cmd *exec.Cmd
}

// NewPostCreate prepares script to run inside env.Path.
func NewPostCreate(script string, env Env) *PostCreate {
	cmd := exec.Command("bash", script)
	cmd.Dir = env.Path
	cmd.Env = append(os.Environ(),
		"WTM_DIR="+env.Dir,
		"WTM_WORKTREE_ROOT="+env.Root,
		"WTM_WORKTREE_NAME="+env.Name,
		"WTM_WORKTREE_PATH="+env.Path,
		"WTM_BRANCH="+env.Branch,
		"WTM_BASE_BRANCH="+env.Base,
	)
	return &PostCreate{cmd: cmd}
}

// Dir is the directory the script runs in.
func (h *PostCreate) Dir() string { return h.cmd.Dir }

func (h *PostCreate) SetStdin(r io.Reader)  { h.cmd.Stdin = r }
func (h *PostCreate) SetStdout(w io.Writer) { h.cmd.Stdout = w }
func (h *PostCreate) SetStderr(w io.Writer) { h.cmd.Stderr = w }

// Run runs the script. When it fails, its output is still on screen, so wait
// for Enter before handing the terminal back to wtm.
func (h *PostCreate) Run() error {
	err := h.cmd.Run()
	if err != nil {
		out := h.cmd.Stdout
		if out == nil {
			out = os.Stdout
		}
		fmt.Fprintf(out, "\nwtm: %s failed (%v). Press Enter to return to wtm.", config.PostCreateName, err)
		if h.cmd.Stdin != nil {
			buf := make([]byte, 1)
			for {
				n, rerr := h.cmd.Stdin.Read(buf)
				if rerr != nil || (n == 1 && (buf[0] == '\n' || buf[0] == '\r')) {
					break
				}
			}
		}
	}
	return err
}
