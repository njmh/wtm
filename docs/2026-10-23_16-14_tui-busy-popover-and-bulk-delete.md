# wtm — TUI busy popover and bulk delete

**Date:** 2026-10-23

## Goal

- Show a running action in a popover that blocks the rest of the TUI, so a
  slow delete can't be doubled up with another action.
- Check several worktrees and delete them together.

## Busy popover

- New `modeBusy`. `startBusy(activity, op)` enters it and batches `op` with
  the spinner's first tick; `endBusy` leaves it, unless an error popover has
  already replaced it. This replaces the old `busy` flag and the status line
  that showed progress, so one field (`mode`) decides what's on screen and
  which keys work.
- While busy, every key except ctrl+c is ignored. ctrl+c stays as an escape
  hatch in case git hangs.
- The spinner (`bubbles/spinner`) only animates while busy: ticks arriving
  outside `modeBusy` are dropped, which ends the tick chain until the next
  `startBusy`.

## Bulk delete

| Question | Decision |
|----------|----------|
| What's checked | `checked map[string]bool` keyed by worktree path, so checks survive list reloads and cursor moves. Paths that disappear on reload are dropped, so successful deletes uncheck themselves and failed ones stay checked |
| Main worktree | No checkbox, and space does nothing on it, since it can't be deleted |
| Keys | Space toggles the selected worktree; esc unchecks all |
| Which actions | D acts on the checked worktrees when any are, otherwise the selected one. R needs one worktree, so it's greyed out in the menu and ignored. C, P and Q don't act on a selection, so they stay enabled |
| Confirmation | One worktree: the existing popover. Several: a list with `!` beside branches with unmerged commits and one shared warning, shortened to "…and N more" when it wouldn't fit the screen |
| Running | One `git.Repo.Delete` per command. Each `deleteStepMsg` records its result and starts the next, so the popover shows "Deleting x… (2 of 5)" |
| Failures | A failure doesn't stop the rest. A single delete shows git's error as before; for several, the error popover lists each failure on one line (`briefError` drops the `$ git …` line). The error popover now cuts off text that doesn't fit the screen |

## Tests

`settle` now expands `tea.BatchMsg` and drops spinner ticks, and `result`
pulls an operation's message out of the batch `startBusy` returns.
`TestPruneKey` checks keys are blocked while busy; `TestCheckAndDeleteSeveral`
covers checking, the blocked rename, progress, a partial failure and esc;
`TestDeleteManyFitsScreen` checks the confirmation and failure list fit on a
24-line screen with 30 worktrees.
