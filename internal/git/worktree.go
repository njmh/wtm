package git

import "strings"

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
