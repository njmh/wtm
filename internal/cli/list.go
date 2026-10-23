package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/njmh/wtm/internal/git"
)

func newListCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List the worktrees",
		Long: `List the worktrees with their branch, commit and state.

Worktrees are named by their path inside worktree_path; others are shown as
./path. These names are what the other commands take.`,
		Args: exactArgs(),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := loadRepo()
			if err != nil {
				return err
			}
			list, err := repo.List()
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), repo, list)
			}
			return writeTable(cmd.OutOrStdout(), repo, list)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON, for scripts")
	return cmd
}

// writeTable prints the worktrees in columns, like the TUI's list.
func writeTable(w io.Writer, repo git.Repo, list []git.Worktree) error {
	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "WORKTREE\tBRANCH\tHEAD\t")
	for _, wt := range list {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", repo.Name(wt), wt.BranchLabel(), wt.ShortHead(), strings.Join(wt.Tags(), " "))
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	// tabwriter pads the last column too.
	for _, line := range strings.SplitAfter(buf.String(), "\n") {
		if line == "" {
			continue
		}
		if _, err := io.WriteString(w, strings.TrimRight(line, " \n")+"\n"); err != nil {
			return err
		}
	}
	return nil
}

// worktreeJSON is how "wtm list --json" describes a worktree. It has its own
// type so the JSON field names stay stable if git.Worktree changes.
type worktreeJSON struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Branch   string `json:"branch"`
	Head     string `json:"head"`
	Main     bool   `json:"main"`
	Bare     bool   `json:"bare"`
	Detached bool   `json:"detached"`
	Locked   bool   `json:"locked"`
	Prunable bool   `json:"prunable"`
}

func writeJSON(w io.Writer, repo git.Repo, list []git.Worktree) error {
	// make, not a nil slice, so no worktrees prints [] rather than null.
	out := make([]worktreeJSON, 0, len(list))
	for _, wt := range list {
		out = append(out, worktreeJSON{
			Name:     repo.Name(wt),
			Path:     wt.Path,
			Branch:   wt.Branch,
			Head:     wt.Head,
			Main:     wt.Main,
			Bare:     wt.Bare,
			Detached: wt.Detached,
			Locked:   wt.Locked,
			Prunable: wt.Prunable,
		})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
