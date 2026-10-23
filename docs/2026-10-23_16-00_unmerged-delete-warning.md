# wtm — warn, don't refuse, on unmerged branches

**Date:** 2026-10-23

## Problem

Delete refused any branch whose commits weren't ancestors of its upstream (or
`HEAD`), like `git branch -d`. In practice that blocked almost every delete:

- **Squash/rebase merges** put new commits on `main`, so the branch's own
  commits are never ancestors of it, even after the PR is merged.
- **Bare clones don't move `main`.** `git clone --bare` sets no fetch refspec,
  so `git fetch` only updates `FETCH_HEAD`; the local `main` stays where it was
  until it's updated on purpose.

## Change

- `git.Repo.UnmergedWarning(wt)` runs the same check (`rev-list --count
  <upstream or HEAD>..<branch>`) and returns a warning naming the commit count
  and what it was compared with, or `""`.
- `Delete` no longer checks; it deletes the branch with `branch -D`. Callers
  show the warning first. The uncommitted-changes check is unchanged:
  `git worktree remove` still refuses a dirty worktree.
- CLI: the warning is shown in the `[y/N]` prompt; with `--yes` it's printed
  to stderr and the delete goes ahead.
- TUI: the warning is computed when the delete popover opens and shown in it.
