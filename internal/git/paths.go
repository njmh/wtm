package git

import (
	"os"
	"path/filepath"
	"strings"
)

// RelInside returns path relative to base, if path is inside base.
func RelInside(base, path string) (string, bool) {
	rel, err := filepath.Rel(base, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return rel, true
}

// Name is how a worktree is shown and named: its path relative to Root.
// Worktrees outside Root are shown as ./path relative to Dir, or in full.
func (g Repo) Name(wt Worktree) string {
	if rel, ok := g.InRoot(wt); ok {
		return rel
	}
	return g.arg(wt.Path)
}

// InRoot returns the worktree's path relative to Root, if it is inside it.
func (g Repo) InRoot(wt Worktree) (string, bool) {
	return RelInside(g.WorktreeRoot(), wt.Path)
}

// Path is where the worktree called name lives.
func (g Repo) Path(name string) string {
	return filepath.Join(g.WorktreeRoot(), name)
}

// arg passes paths inside Dir to git as ./name, which keeps git's messages short.
func (g Repo) arg(path string) string {
	if rel, ok := RelInside(g.Dir, path); ok {
		return "./" + rel
	}
	return path
}

// removeEmptyParents deletes now-empty directories left between path and
// Root (or Dir, for worktrees outside Root), e.g. "feature" after removing the
// worktree "feature/login". Root and Dir themselves are kept.
func (g Repo) removeEmptyParents(path string) {
	stop := g.WorktreeRoot()
	if _, ok := RelInside(stop, path); !ok {
		stop = g.Dir
	}
	for dir := filepath.Dir(path); ; dir = filepath.Dir(dir) {
		if rel, ok := RelInside(stop, dir); !ok || rel == "." {
			return
		}
		if os.Remove(dir) != nil {
			return
		}
	}
}
