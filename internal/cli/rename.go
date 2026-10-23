package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newRenameCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "rename <worktree> <new-name>",
		Aliases: []string{"mv"},
		Short:   "Rename a worktree's directory and branch",
		Long: `Rename the worktree's directory and its branch to new-name. The worktree can be
given by name, path or branch.

The worktree moves to new-name inside worktree_path, so renaming one that is
outside worktree_path moves it in.`,
		Args:              exactArgs("<worktree>", "<new-name>"),
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
			oldName, newName := repo.Name(wt), args[1]
			if newName == oldName && (wt.Branch == "" || wt.Branch == newName) {
				return fmt.Errorf("%s is already named %s", oldName, newName)
			}
			if err := repo.Rename(wt, newName); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Renamed %s to %s.\n", oldName, newName)
			return nil
		},
	}
}
