// Package cli is wtm's command line, built with Cobra. With no arguments it
// opens the TUI; otherwise it runs one command, such as "wtm create name",
// and exits.
package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"

	"github.com/njmh/wtm/internal/config"
	"github.com/njmh/wtm/internal/git"
	"github.com/njmh/wtm/internal/tui"
)

// Execute runs the command named by the program's arguments.
func Execute() error {
	return newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "wtm",
		Short: "Manage the git worktrees in the current directory",
		Long: `wtm manages the git worktrees of the repository in the current directory.

Run it with no command to open the interactive worktree manager, or run one of
the commands below to do the same from the shell or a script.

New worktrees go in the worktree_path set in .wtm/config.toml (by default the
current directory), and .wtm/post_create.sh runs after each one is created.`,
		Example: `  wtm                          Open the worktree manager
  wtm list                     List the worktrees
  wtm create feature-x         Create worktree and branch feature-x from main
  wtm create fix -b develop    Create fix from develop
  wtm rename fix bugfix        Rename worktree and branch fix to bugfix
  wtm delete bugfix            Delete worktree and branch bugfix
  wtm prune                    Clean up worktrees whose directory is gone`,
		Version: version(),
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return nil
			}
			msg := fmt.Sprintf("unknown command %q", args[0])
			if s := cmd.SuggestionsFor(args[0]); len(s) > 0 {
				msg += fmt.Sprintf(" (did you mean %q?)", s[0])
			}
			return usageError(cmd, errors.New(msg))
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := loadRepo()
			if err != nil {
				return err
			}
			return tui.Run(repo)
		},
		// main prints errors, and usage errors say where to find help instead
		// of printing it all.
		SilenceErrors: true,
		SilenceUsage:  true,
		// Suggest commands up to two typos away, e.g. list for lst.
		SuggestionsMinimumDistance: 2,
	}
	root.SetFlagErrorFunc(usageError)
	root.AddCommand(
		newListCmd(),
	)
	return root
}

// usageError points from a mistake on the command line to the command's help.
func usageError(cmd *cobra.Command, err error) error {
	return fmt.Errorf("%w\nRun '%s --help' for usage.", err, cmd.CommandPath())
}

// exactArgs requires one argument for each of names, which describe them in
// error messages.
func exactArgs(names ...string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		switch {
		case len(args) < len(names):
			return usageError(cmd, fmt.Errorf("missing %s", strings.Join(names[len(args):], " and ")))
		case len(args) > len(names):
			return usageError(cmd, fmt.Errorf("unexpected argument %q", args[len(names)]))
		}
		return nil
	}
}

// loadRepo reads the configuration of the current directory and returns its
// repository.
func loadRepo() (git.Repo, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return git.Repo{}, errors.New("git was not found in PATH")
	}
	dir, err := os.Getwd()
	if err != nil {
		return git.Repo{}, err
	}
	// git reports resolved paths, so resolve symlinks for name comparisons.
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	cfg, err := config.Load(dir)
	if err != nil {
		return git.Repo{}, err
	}
	return git.Repo{Dir: dir, Root: cfg.Root(dir)}, nil
}

// version is the module version wtm was built from, e.g. v1.2.0 when
// installed with "go install github.com/njmh/wtm@v1.2.0".
func version() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "unknown"
}
