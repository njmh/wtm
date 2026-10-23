package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/njmh/wtm/internal/git"
)

func newDeleteCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:     "delete <worktree>",
		Aliases: []string{"rm"},
		Short:   "Delete a worktree, its directory and its branch",
		Long: `Delete the worktree, its directory and its branch. The worktree can be given
by name, path or branch.

Nothing is deleted if the worktree has uncommitted or untracked files.

wtm asks before deleting unless --yes is given. When there's no terminal to ask
in, such as in a script, --yes is required. If the branch has commits that
aren't in its upstream (or in HEAD, when it has none), wtm warns about them but
still deletes the branch: a squash-merged branch, or one whose upstream hasn't
been fetched, looks unmerged too.`,
		Args:              exactArgs("<worktree>"),
		ValidArgsFunction: completeWorktree,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := loadRepo()
			if err != nil {
				return err
			}
			wt, err := repo.Find(args[0])
			if err != nil {
				return err
			}
			if err := repo.CheckChangeable(wt, "delete"); err != nil {
				return err
			}
			name := repo.Name(wt)
			warning := repo.UnmergedWarning(wt)
			if yes {
				if warning != "" {
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %s\n", warning)
				}
			} else {
				ok, err := confirmDelete(cmd, repo, wt, warning)
				if err != nil {
					return err
				}
				if !ok {
					fmt.Fprintln(cmd.OutOrStdout(), "Nothing was deleted.")
					return nil
				}
			}
			if err := repo.Delete(wt); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted worktree %s.\n", name)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "delete without asking first")
	return cmd
}

// confirmDelete shows what deleting wt removes, and warning if there is one,
// and asks whether to go ahead. Like git's own prompts, the question goes to
// stderr.
func confirmDelete(cmd *cobra.Command, repo git.Repo, wt git.Worktree, warning string) (bool, error) {
	if !isTerminal(cmd.InOrStdin()) {
		return false, fmt.Errorf("not deleting %s without confirmation; use --yes to delete it from a script", repo.Name(wt))
	}
	branch := wt.Branch
	if branch == "" {
		branch = "none (detached HEAD)"
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "Delete worktree %s? This permanently removes:\n\n  directory  %s\n  branch     %s\n\n",
		repo.Name(wt), shortPath(repo, wt.Path), branch)
	if warning != "" {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %s\n\n", warning)
	}
	fmt.Fprint(cmd.ErrOrStderr(), "[y/N] ")

	line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	}
	return false, nil
}

// isTerminal reports whether r is an interactive terminal, rather than a pipe
// or file. It's a variable so tests can pretend.
var isTerminal = func(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
