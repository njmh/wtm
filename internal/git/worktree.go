package git

import (
	"fmt"
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

// List returns all worktrees of the repository.
func (g Repo) List() ([]Worktree, error) {
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

// ShortHead is the first seven characters of the commit hash.
func (wt Worktree) ShortHead() string {
	if len(wt.Head) > 7 {
		return wt.Head[:7]
	}
	return wt.Head
}

// BranchLabel is the branch to show for the worktree, with placeholders for
// bare repositories and detached worktrees.
func (wt Worktree) BranchLabel() string {
	switch {
	case wt.Branch != "":
		return wt.Branch
	case wt.Bare:
		return "—"
	case wt.Detached:
		return "(detached)"
	}
	return ""
}

// Tags are short labels for the worktree's state: bare or main, locked and
// prunable.
func (wt Worktree) Tags() []string {
	var tags []string
	if wt.Bare {
		tags = append(tags, "bare")
	} else if wt.Main {
		tags = append(tags, "main")
	}
	if wt.Locked {
		tags = append(tags, "locked")
	}
	if wt.Prunable {
		tags = append(tags, "prunable")
	}
	return tags
}

// Find returns the worktree called name. name can be the worktree's Name, its
// path (absolute or relative to Dir) or its branch.
func (g Repo) Find(name string) (Worktree, error) {
	list, err := g.List()
	if err != nil {
		return Worktree{}, err
	}
	// Shell completion of a directory adds a trailing slash.
	trimmed := strings.TrimSuffix(name, "/")
	for _, wt := range list {
		if g.Name(wt) == trimmed {
			return wt, nil
		}
	}
	path := name
	if !filepath.IsAbs(path) {
		path = filepath.Join(g.Dir, path)
	}
	for _, wt := range list {
		if wt.Path == filepath.Clean(path) {
			return wt, nil
		}
	}
	for _, wt := range list {
		if wt.Branch != "" && wt.Branch == name {
			return wt, nil
		}
	}
	return Worktree{}, fmt.Errorf("no worktree named %q", name)
}

// Branches returns the names of the repository's local branches.
func (g Repo) Branches() ([]string, error) {
	out, err := g.run("for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return nil, err
	}
	return strings.Fields(out), nil
}
