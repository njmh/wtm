package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/njmh/wtm/internal/config"
	"github.com/njmh/wtm/internal/git"
	"github.com/njmh/wtm/internal/hook"
)

func newCreateCmd() *cobra.Command {
	var base string
	var noHook bool
	cmd := &cobra.Command{
		Use:     "create <name>",
		Aliases: []string{"new"},
		Short:   "Create a worktree on a new branch",
		Long: `Create a worktree called name inside worktree_path, on a new branch also called
name that starts from the base branch. Then run .wtm/post_create.sh inside it,
if there is one.

Names can contain slashes, e.g. feature/login.`,
		Example: `  wtm create feature-x
  wtm create fix/login --base develop`,
		Args: exactArgs("<name>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := loadRepo()
			if err != nil {
				return err
			}
			name := args[0]
			if base == "" {
				base = "main"
			}
			if err := repo.Create(name, base); err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Created worktree %s from %s at %s.\n", name, base, shortPath(repo, repo.Path(name)))

			script := config.PostCreateScript(repo.Dir)
			if noHook || script == "" {
				return nil
			}
			fmt.Fprintf(out, "Running %s…\n", config.PostCreateName)
			h := hook.NewPostCreate(script, hook.Env{
				Dir:    repo.Dir,
				Root:   repo.WorktreeRoot(),
				Name:   name,
				Path:   repo.Path(name),
				Branch: name,
				Base:   base,
			})
			h.SetStdin(cmd.InOrStdin())
			h.SetStdout(out)
			h.SetStderr(cmd.ErrOrStderr())
			if err := h.Run(); err != nil {
				return fmt.Errorf("the worktree was created, but %s failed: %w", config.PostCreateName, err)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&base, "base", "b", "main", "the branch to start the new branch from")
	cmd.Flags().BoolVar(&noHook, "no-hook", false, "don't run "+config.PostCreateName)
	return cmd
}

// shortPath shows paths inside the working directory as ./name.
func shortPath(repo git.Repo, path string) string {
	if rel, ok := git.RelInside(repo.Dir, path); ok {
		return "./" + rel
	}
	return path
}
