# wtm — Worktree Manager

A terminal UI for managing git worktrees, built with
[Bubble Tea](https://charm.land/bubbletea) and [Lip Gloss](https://charm.land/lipgloss).

Run `wtm` from a directory inside a git repository. By default new worktrees
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
| D          | Delete the selected worktree, its directory and its branch             |
| R          | Rename the selected worktree's directory and branch                    |
| P          | `git worktree prune`                                                   |
| Q, ctrl+c  | Quit                                                                   |

Deletion is safe: it refuses (and changes nothing) when the worktree has
uncommitted or untracked files, or when the branch has commits not merged into
its upstream (or `HEAD` if it has none). Every git error is shown in a dialog.

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
executable. wtm hands over the terminal while it runs, so you see its output
and it can start interactive programs. It runs inside the new worktree with:

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

If the script exits non-zero, wtm waits for Enter so you can read its output,
then shows an error. The worktree is kept.

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
(`git`, `config`, `hook` and `tui`).

Requires Go 1.27+ and git on `PATH`.
