// Command wtm manages the git worktrees in the current directory, through a
// terminal UI or, given a command such as "wtm create name", from the shell.
package main

import (
	"fmt"
	"os"

	"github.com/njmh/wtm/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "wtm:", err)
		os.Exit(1)
	}
}
