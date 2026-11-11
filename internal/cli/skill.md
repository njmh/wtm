# wtm — Worktree Manager

wtm manages the git worktrees of one repository, through a terminal UI (`wtm`
with no arguments) or through the commands below. This text is printed by
`wtm skill` to tell an AI agent how wtm works, so it can run wtm or write
wtm's configuration for its user.

## For agents

- Don't run `wtm` with no command: that opens the interactive terminal UI,
  which you can't drive. Use the commands below.
- Run wtm from the directory that holds `.wtm/` (see [Layout](#layout)), not
  from inside a worktree. wtm reads its configuration from the directory it's
  started in and doesn't search parent directories.
- Use `wtm list --json` to read the worktrees; the table is for people.
- `wtm delete` asks for confirmation, and refuses when stdin isn't a terminal.
  Pass `--yes` once the user has agreed to the delete: it permanently removes
  the directory and the branch.
- `wtm create` runs `.wtm/post_create.sh`, which can be slow or interactive.
  Pass `--no-hook` to skip it.
- Errors go to stderr and the exit status is 1.

## Layout

By default new worktrees are created directly inside the directory wtm runs
in, so wtm is meant for a "container" directory that holds a bare clone and
its worktrees side by side:

```
myproject/
├── .bare/          # git clone --bare <url> .bare
├── .git            # file containing: gitdir: ./.bare
├── .wtm/           # optional wtm configuration
│   ├── config.toml
│   └── post_create.sh
├── main/
└── feature-x/
```

## Commands

| Command                                      | Alias | Does                                                         |
|----------------------------------------------|-------|--------------------------------------------------------------|
| `wtm list [--json]`                          | `ls`  | List the worktrees, as a table or as JSON                    |
| `wtm create <name> [--base <b>] [--no-hook]` | `new` | Create a worktree on new branch `<name>` from branch `<b>` (default `main`), then run `post_create.sh` unless `--no-hook` |
| `wtm delete <worktree> [-y]`                 | `rm`  | Delete the worktree, its directory and its branch            |
| `wtm rename <worktree> <new-name>`           | `mv`  | Rename the worktree's directory and branch                   |
| `wtm prune`                                  |       | `git worktree prune`: forget worktrees whose directory is gone |
| `wtm skill`                                  |       | Print this text                                              |
| `wtm skill install <agent>`                  |       | Install a skill that tells an agent to run `wtm skill`       |

`<worktree>` is a name as shown by `wtm list`, a path, or a branch name.
`<name>` is used for both the directory and the branch, and can contain
slashes, e.g. `feature/login`. `wtm <command> --help` shows a command's usage.

`wtm list --json` prints an array with one object per worktree:

```json
[
  {
    "name": "feature-x",
    "path": "/path/to/myproject/feature-x",
    "branch": "feature-x",
    "head": "63d7bba0c1f5e2a4b6d8f0a1c3e5b7d9f1a2c4e6",
    "main": false,
    "bare": false,
    "detached": false,
    "locked": false,
    "prunable": false
  }
]
```

`main` marks the repository's main worktree (the bare repository, in the
layout above), which can't be deleted or renamed. `branch` is `""` when
`detached` or `bare` is true. `prunable` means the directory is gone and
`wtm prune` would forget the worktree.

### Deleting

`wtm delete` refuses, and changes nothing, when the worktree has uncommitted
or untracked files. When the branch has commits that aren't in its upstream
(or in `HEAD`, when it has none) wtm prints a warning but still deletes it,
because squash-merged branches, and branches whose upstream hasn't been
fetched, look unmerged too. Tell the user about that warning rather than
assuming the commits are safe to lose.

## Configuration

wtm reads an optional `.wtm/` directory in the directory it's started from.

### `.wtm/config.toml`

```toml
# Where new worktrees are created, relative to this directory (default "./").
worktree_path = "trees"
```

`worktree_path` is the only option. Unknown options or invalid TOML stop wtm
with an error.

Worktrees are named by their path inside `worktree_path`, so with the setting
above, `wtm create feature-x` makes `./trees/feature-x` on branch `feature-x`.
Worktrees outside it are listed as `./path`, and renaming one of them moves it
into `worktree_path`.

### `.wtm/post_create.sh`

Runs after each successful create, from the TUI or `wtm create`. Use it to
set up a new worktree: copy untracked files such as `.env` into it, install
dependencies, open an editor.

- It's run with `bash`, so it doesn't need a shebang or to be executable.
- Its working directory is the new worktree.
- It's connected to the terminal, so its output is shown as it runs and it
  can start interactive programs.
- If it exits non-zero the worktree is kept and wtm reports an error. It
  isn't run again, so write it to be safe to re-run by hand.

It's given these environment variables:

| Variable            | Value                                                    |
|---------------------|----------------------------------------------------------|
| `WTM_WORKTREE_NAME` | Name of the new worktree, relative to `worktree_path`    |
| `WTM_WORKTREE_PATH` | Absolute path of the new worktree                        |
| `WTM_WORKTREE_ROOT` | Absolute path of `worktree_path`                         |
| `WTM_BRANCH`        | The new branch (currently the same as the worktree name) |
| `WTM_BASE_BRANCH`   | The branch it was created from                           |
| `WTM_DIR`           | The directory wtm was started from, which holds `.wtm/`  |

```bash
# e.g. share an env file and install dependencies
set -euo pipefail
cp "$WTM_DIR/.env" .env
npm install
```

Files that aren't tracked by git exist only in the worktree they were created
in, so copy or symlink shared ones from a fixed place such as `$WTM_DIR` or
`$WTM_WORKTREE_ROOT/$WTM_BASE_BRANCH`. The base branch may not have a
worktree, so check that a file exists before copying it from there.
