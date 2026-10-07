package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Worktree is a single entry from `git worktree list --porcelain`.
type Worktree struct {
	Path     string
	Head     string
	Branch   string // short branch name; empty when detached or bare
	Bare     bool
	Detached bool
	Locked   bool
	Prunable bool
	Main     bool // the first entry: the main worktree or bare repository
}

// GitError carries the failed command and git's own error output.
type GitError struct {
	Args   []string
	Output string
	Err    error
}

func (e *GitError) Error() string {
	msg := strings.TrimSpace(e.Output)
	if msg == "" {
		msg = e.Err.Error()
	}
	return fmt.Sprintf("$ git %s\n\n%s", strings.Join(e.Args, " "), msg)
}

func (e *GitError) Unwrap() error { return e.Err }

// Git runs git commands against the repository that contains Dir. New
// worktrees are created inside Root.
type Git struct {
	Dir  string // the directory wtm runs in
	Root string // where new worktrees go; Dir when empty
}

func (g Git) root() string {
	if g.Root == "" {
		return g.Dir
	}
	return g.Root
}

// relInside returns path relative to base, if path is inside base.
func relInside(base, path string) (string, bool) {
	rel, err := filepath.Rel(base, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return rel, true
}

func (g Git) run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.Dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), &GitError{Args: args, Output: stderr.String() + stdout.String(), Err: err}
	}
	return stdout.String() + stderr.String(), nil
}

// List returns all worktrees of the repository.
func (g Git) List() ([]Worktree, error) {
	out, err := g.run("worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	return parseWorktrees(out), nil
}

func parseWorktrees(out string) []Worktree {
	var list []Worktree
	var cur *Worktree
	for _, line := range strings.Split(out, "\n") {
		key, value, _ := strings.Cut(line, " ")
		switch key {
		case "worktree":
			list = append(list, Worktree{Path: value})
			cur = &list[len(list)-1]
		case "HEAD":
			if cur != nil {
				cur.Head = value
			}
		case "branch":
			if cur != nil {
				cur.Branch = strings.TrimPrefix(value, "refs/heads/")
			}
		case "bare":
			if cur != nil {
				cur.Bare = true
			}
		case "detached":
			if cur != nil {
				cur.Detached = true
			}
		case "locked":
			if cur != nil {
				cur.Locked = true
			}
		case "prunable":
			if cur != nil {
				cur.Prunable = true
			}
		}
	}
	if len(list) > 0 {
		list[0].Main = true
	}
	return list
}

// Name is how a worktree is shown and named: its path relative to Root.
// Worktrees outside Root are shown as ./path relative to Dir, or in full.
func (g Git) Name(wt Worktree) string {
	if rel, ok := g.InRoot(wt); ok {
		return rel
	}
	return g.arg(wt.Path)
}

// InRoot returns the worktree's path relative to Root, if it is inside it.
func (g Git) InRoot(wt Worktree) (string, bool) {
	return relInside(g.root(), wt.Path)
}

// Path is where the worktree called name lives.
func (g Git) Path(name string) string {
	return filepath.Join(g.root(), name)
}

// arg passes paths inside Dir to git as ./name, which keeps git's messages short.
func (g Git) arg(path string) string {
	if rel, ok := relInside(g.Dir, path); ok {
		return "./" + rel
	}
	return path
}

// validateName checks that name works both as a branch name and as a
// directory inside Root.
func (g Git) validateName(name string) error {
	if name == "" {
		return errors.New("a name is required")
	}
	if strings.HasPrefix(name, "-") {
		return fmt.Errorf("%q is not a valid name: it can't start with '-'", name)
	}
	out, err := g.run("check-ref-format", "--branch", name)
	if err != nil || strings.TrimSpace(out) != name {
		return fmt.Errorf("%q is not a valid branch name", name)
	}
	return nil
}

func (g Git) branchExists(name string) bool {
	_, err := g.run("show-ref", "--verify", "--quiet", "refs/heads/"+name)
	return err == nil
}

// Create adds a worktree at Root/name on a new branch called name, based on base.
func (g Git) Create(name, base string) error {
	if err := g.validateName(name); err != nil {
		return err
	}
	if base == "" {
		base = "main"
	}
	_, err := g.run("worktree", "add", "-b", name, g.arg(g.Path(name)), base)
	return err
}

// Delete removes the worktree, its directory and its branch. It refuses to
// discard uncommitted changes or unmerged commits; git's error is returned.
func (g Git) Delete(wt Worktree) error {
	if wt.Main {
		return errors.New("the main worktree can't be deleted")
	}
	if wt.Branch != "" {
		if err := g.checkMerged(wt.Branch); err != nil {
			return err
		}
	}
	if _, err := g.run("worktree", "remove", g.arg(wt.Path)); err != nil {
		return err
	}
	g.removeEmptyParents(wt.Path)
	if wt.Branch == "" {
		return nil
	}
	if _, err := g.run("branch", "-d", wt.Branch); err != nil {
		return fmt.Errorf("the worktree was removed, but branch %q was not deleted.\n\n%w", wt.Branch, err)
	}
	return nil
}

// checkMerged mirrors the safety check of `git branch -d` (merged into its
// upstream, or into HEAD when it has none) so Delete can refuse up front
// instead of failing after the worktree is already gone.
func (g Git) checkMerged(branch string) error {
	into := "HEAD"
	if _, err := g.run("rev-parse", "--verify", "--quiet", branch+"@{upstream}"); err == nil {
		into = branch + "@{upstream}"
	}
	_, err := g.run("merge-base", "--is-ancestor", branch, into)
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		if into == "HEAD" {
			if out, err := g.run("symbolic-ref", "--short", "HEAD"); err == nil {
				into = strings.TrimSpace(out)
			}
		}
		return fmt.Errorf("branch %q has commits that aren't merged into %s, so nothing was deleted.\n\nMerge it first, or delete it manually with:\n  git worktree remove <path>\n  git branch -D %s", branch, into, branch)
	}
	return err
}

// Rename moves the worktree to Root/newName and renames its branch to newName.
func (g Git) Rename(wt Worktree, newName string) error {
	if wt.Main {
		return errors.New("the main worktree can't be renamed")
	}
	if err := g.validateName(newName); err != nil {
		return err
	}
	newPath := g.Path(newName)
	renameBranch := wt.Branch != "" && wt.Branch != newName
	if renameBranch && g.branchExists(newName) {
		return fmt.Errorf("a branch named %q already exists", newName)
	}

	if filepath.Clean(wt.Path) != newPath {
		if _, err := os.Lstat(newPath); err == nil {
			return fmt.Errorf("%s already exists", newPath)
		}
		// Names containing '/' need their parent directories in place.
		if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
			return err
		}
		if _, err := g.run("worktree", "move", g.arg(wt.Path), g.arg(newPath)); err != nil {
			g.removeEmptyParents(newPath)
			return err
		}
		g.removeEmptyParents(wt.Path)
	}

	if renameBranch {
		if _, err := g.run("branch", "-m", wt.Branch, newName); err != nil {
			return fmt.Errorf("the worktree was moved, but branch %q was not renamed.\n\n%w", wt.Branch, err)
		}
	}
	return nil
}

// Prune removes stale worktree entries and reports how many were removed.
func (g Git) Prune() (int, error) {
	out, err := g.run("worktree", "prune", "--verbose")
	if err != nil {
		return 0, err
	}
	n := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) != "" {
			n++
		}
	}
	return n, nil
}

// removeEmptyParents deletes now-empty directories left between path and
// Root (or Dir, for worktrees outside Root), e.g. "feature" after removing the
// worktree "feature/login". Root and Dir themselves are kept.
func (g Git) removeEmptyParents(path string) {
	stop := g.root()
	if _, ok := relInside(stop, path); !ok {
		stop = g.Dir
	}
	for dir := filepath.Dir(path); ; dir = filepath.Dir(dir) {
		if rel, ok := relInside(stop, dir); !ok || rel == "." {
			return
		}
		if os.Remove(dir) != nil {
			return
		}
	}
}
