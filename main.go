// Command wtm is a terminal UI for managing the git worktrees in the current
// directory.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"github.com/njmh/wtm/internal/git"
)

func main() {
	if _, err := exec.LookPath("git"); err != nil {
		fmt.Fprintln(os.Stderr, "wtm: git was not found in PATH")
		os.Exit(1)
	}
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "wtm:", err)
		os.Exit(1)
	}
	// git reports resolved paths, so resolve symlinks for name comparisons.
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}

	cfg, err := loadConfig(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "wtm:", err)
		os.Exit(1)
	}

	g := git.Repo{Dir: dir, Root: cfg.root(dir)}
	if _, err := tea.NewProgram(newModel(g)).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "wtm:", err)
		os.Exit(1)
	}
}
