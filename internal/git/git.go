// Package git lists and changes the worktrees of a repository by running the
// git command.
package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Error carries the failed command and git's own error output.
type Error struct {
	Args   []string
	Output string
	Err    error
}

func (e *Error) Error() string {
	msg := strings.TrimSpace(e.Output)
	if msg == "" {
		msg = e.Err.Error()
	}
	return fmt.Sprintf("$ git %s\n\n%s", strings.Join(e.Args, " "), msg)
}

func (e *Error) Unwrap() error { return e.Err }

// Repo runs git commands against the repository that contains Dir. New
// worktrees are created inside Root.
type Repo struct {
	Dir  string // the directory wtm runs in
	Root string // where new worktrees go; Dir when empty
}

// WorktreeRoot is where new worktrees go: Root, or Dir when Root is empty.
func (g Repo) WorktreeRoot() string {
	if g.Root == "" {
		return g.Dir
	}
	return g.Root
}

func (g Repo) run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.Dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), &Error{Args: args, Output: stderr.String() + stdout.String(), Err: err}
	}
	return stdout.String() + stderr.String(), nil
}
