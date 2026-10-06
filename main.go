// Command wtm is a terminal UI for managing the git worktrees in the current
// directory.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
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

	if _, err := tea.NewProgram(newModel(Git{Dir: dir})).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "wtm:", err)
		os.Exit(1)
	}
}
