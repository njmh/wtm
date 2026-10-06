# wtm — Worktree Manager

A terminal UI for managing git worktrees, built with
[Bubble Tea](https://charm.land/bubbletea) and [Lip Gloss](https://charm.land/lipgloss).

Run `wtm` from a directory inside a git repository. New worktrees are created
directly inside the current directory, so it's meant for a "container" layout:

```
myproject/
├── .bare/          # git clone --bare <url> .bare
├── .git            # file containing: gitdir: ./.bare
├── main/
└── feature-x/
```

## Keys

| Key        | Action                                                                 |
|------------|------------------------------------------------------------------------|
| ↑/↓, k/j   | Move the selection                                                     |
| C          | Create a worktree: `./<name>` on new branch `<name>`, from a base branch (default `main`) |
| D          | Delete the selected worktree, its directory and its branch             |
| R          | Rename the selected worktree's directory and branch                    |
| P          | `git worktree prune`                                                   |
| Q, ctrl+c  | Quit                                                                   |

Deletion is safe: it refuses (and changes nothing) when the worktree has
uncommitted or untracked files, or when the branch has commits not merged into
its upstream (or `HEAD` if it has none). Every git error is shown in a dialog.

## Build

```sh
go build -o wtm .
go test ./...
```

Requires Go 1.27+ and git on `PATH`.
