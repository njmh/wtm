# wtm — Worktree Manager

A terminal UI and command line for managing git worktrees, built with
[Bubble Tea](https://charm.land/bubbletea), [Lip Gloss](https://charm.land/lipgloss)
and [Cobra](https://cobra.dev).

Run `wtm` from a directory inside a git repository to open the worktree
manager, or `wtm <command>` to do the same from the shell or a script (see
[Command line](#command-line)). By default new worktrees
are created directly inside the current directory (see
[Configuration](#configuration)), so it's meant for a "container" layout:

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
| C          | Create a worktree: `<worktree_path>/<name>` on new branch `<name>`, from a base branch (default `main`), then run `post_create.sh` |
| Space      | Check or uncheck the selected worktree, to delete several at once      |
| Esc        | Uncheck all                                                            |
| D          | Delete the checked worktrees, or the selected one if none are checked, with their directories and branches |
| R          | Rename the selected worktree's directory and branch (not while any are checked) |
| P          | `git worktree prune`                                                   |
| Q, ctrl+c  | Quit                                                                   |

While an action runs, a popover shows its progress and other keys are
ignored until it finishes. Checked worktrees are deleted one at a time; if
one fails the rest still go ahead, and the failures are listed afterwards.

Deletion refuses (and changes nothing) when the worktree has uncommitted or
untracked files. When the branch has commits that aren't in its upstream (or
`HEAD` if it has none), the confirmation shows a warning, but you can still
delete it: squash-merged branches, and branches whose upstream hasn't been
fetched, look unmerged too. Every git error is shown in a dialog.

## Command line

Every action in the TUI is also a command:

| Command                                      | Alias | Does                                                         |
|----------------------------------------------|-------|--------------------------------------------------------------|
| `wtm list [--json]`                          | `ls`  | List the worktrees, as a table or as JSON                    |
| `wtm create <name> [-b <base>] [--no-hook]`  | `new` | Create a worktree on new branch `<name>` from `<base>` (default `main`), then run `post_create.sh` unless `--no-hook` |
| `wtm delete <worktree> [-y]`                 | `rm`  | Delete the worktree, its directory and its branch            |
| `wtm rename <worktree> <new-name>`           | `mv`  | Rename the worktree's directory and branch                   |
| `wtm prune`                                  |       | `git worktree prune`                                         |

`<worktree>` is a name as shown by `wtm list`, a path, or a branch name. The
same safety checks apply as in the TUI. `delete` asks for confirmation unless
`-y`/`--yes` is given, and in a script (no terminal on stdin) it refuses
without `--yes`. Errors go to stderr and exit with status 1.

`wtm help`, `wtm --help` and `wtm <command> --help` show usage, and
`wtm --version` prints the version.

### Shell completion

`wtm completion <bash|zsh|fish|powershell>` prints a completion script that
completes commands, worktree names and `--base` branches. For example, for zsh:

```sh
wtm completion zsh > "${fpath[1]}/_wtm"
```

Run `wtm completion <shell> --help` for each shell's setup.

## Configuration

wtm reads an optional `.wtm/` directory in the directory it's started from:

```
myproject/
├── .wtm/
│   ├── config.toml
│   └── post_create.sh
└── ...
```

### `.wtm/config.toml`

```toml
# Where new worktrees are created, relative to this directory (default "./").
worktree_path = "trees"
```

Worktrees are named by their path inside `worktree_path`, so with the setting
above, creating `feature-x` makes `./trees/feature-x` on branch `feature-x`.
Worktrees outside it are listed as `./path`. Renaming one of them moves it
into `worktree_path`. Unknown options or invalid TOML stop wtm with an error.

### `.wtm/post_create.sh`

Runs with `bash` after each successful create, so it doesn't need to be
executable. It's connected to the terminal while it runs, from the TUI or
`wtm create`, so you see its output and it can start interactive programs. It runs inside the new worktree with:

| Variable            | Value                                                    |
|---------------------|----------------------------------------------------------|
| `WTM_WORKTREE_NAME` | Name of the new worktree, relative to `worktree_path`    |
| `WTM_WORKTREE_PATH` | Absolute path of the new worktree                        |
| `WTM_WORKTREE_ROOT` | Absolute path of `worktree_path`                         |
| `WTM_BRANCH`        | The new branch (currently the same as the worktree name) |
| `WTM_BASE_BRANCH`   | The branch it was created from                           |
| `WTM_DIR`           | The directory wtm was started from                       |

```bash
# e.g. share an env file and install dependencies
cp "$WTM_DIR/.env" .env
npm install
```

If the script exits non-zero the worktree is kept and wtm reports an error.
In the TUI, wtm first waits for Enter so you can read the script's output.

## Install

```sh
go install github.com/njmh/wtm@latest
```

## Build

```sh
go build -o wtm .
go test ./...
```

`main.go` is the entry point; the code lives in packages under `internal/`
(`cli`, `git`, `config`, `hook` and `tui`).

Requires Go 1.27+ and git on `PATH`.
