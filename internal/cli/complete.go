package cli

import (
	"github.com/spf13/cobra"
)

// completeWorktree completes the first argument with the names of the
// worktrees that can be changed, described by their branch.
func completeWorktree(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	repo, err := loadRepo()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	list, err := repo.List()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	var names []cobra.Completion
	for _, wt := range list {
		if !wt.Main {
			names = append(names, cobra.CompletionWithDesc(repo.Name(wt), wt.BranchLabel()))
		}
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

// completeBranch completes a flag with the repository's local branches.
func completeBranch(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	repo, err := loadRepo()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	branches, err := repo.Branches()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	return branches, cobra.ShellCompDirectiveNoFileComp
}
