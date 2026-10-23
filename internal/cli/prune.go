package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newPruneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "prune",
		Short: "Remove stale worktree entries",
		Long: `Remove git's records of worktrees whose directory no longer exists (git
worktree prune). Their branches are kept.`,
		Args: exactArgs(),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := loadRepo()
			if err != nil {
				return err
			}
			n, err := repo.Prune()
			if err != nil {
				return err
			}
			status := "Nothing to prune."
			if n > 0 {
				status = fmt.Sprintf("Pruned %d stale worktree %s.", n, plural(n, "entry", "entries"))
			}
			fmt.Fprintln(cmd.OutOrStdout(), status)
			return nil
		},
	}
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
