package git

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// validateName checks that name works both as a branch name and as a
// directory inside Root.
func (g Repo) validateName(name string) error {
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

// CheckChangeable returns an error explaining why wt can't be deleted or
// renamed (verb is "delete" or "rename"), or nil if it can. Only the main
// worktree or bare repository can't.
func (g Repo) CheckChangeable(wt Worktree, verb string) error {
	if !wt.Main {
		return nil
	}
	kind := "main worktree"
	if wt.Bare {
		kind = "bare repository"
	}
	return fmt.Errorf("the %s (%s) can't be %sd", kind, g.Name(wt), verb)
}

func (g Repo) branchExists(name string) bool {
	_, err := g.run("show-ref", "--verify", "--quiet", "refs/heads/"+name)
	return err == nil
}

// Create adds a worktree at Root/name on a new branch called name, based on base.
func (g Repo) Create(name, base string) error {
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
func (g Repo) Delete(wt Worktree) error {
	if err := g.CheckChangeable(wt, "delete"); err != nil {
		return err
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
func (g Repo) checkMerged(branch string) error {
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
func (g Repo) Rename(wt Worktree, newName string) error {
	if err := g.CheckChangeable(wt, "rename"); err != nil {
		return err
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
func (g Repo) Prune() (int, error) {
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
