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
| D          | Delete the checked worktrees, or the selected one if none are checked, with their directories and branches, then run `post_delete.sh` for each |
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
| `wtm delete <worktree> [-y] [--no-hook]`     | `rm`  | Delete the worktree, its directory and its branch, then run `post_delete.sh` unless `--no-hook` |
| `wtm rename <worktree> <new-name>`           | `mv`  | Rename the worktree's directory and branch                   |
| `wtm prune`                                  |       | `git worktree prune`                                         |
| `wtm skill`                                  |       | Describe wtm to an AI agent (see [AI agents](#ai-agents))    |
| `wtm skill install <agent>`                  |       | Install a skill that tells an agent to run `wtm skill`       |

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

## AI agents

`wtm skill` prints a description of wtm written for an AI coding agent: the
commands, the configuration and how `post_create.sh` and `post_delete.sh` are
run. Put it in an
agent's context before asking for help with wtm:

```sh
claude "$(wtm skill) Write a post_create.sh for this project."
```

`wtm skill install <agent>` saves you doing that each time. It installs a
small skill called `wtm` in the agent's personal skills, which tells the agent
to run `wtm skill` itself when you ask it about wtm. Because the skill only
points at the command, it doesn't go out of date when wtm is upgraded.

| Agent      |                    | Installed in                            |
|------------|--------------------|-----------------------------------------|
| `claude`   | Claude Code        | `~/.claude/skills/wtm/SKILL.md`         |
| `codex`    | Codex              | `~/.agents/skills/wtm/SKILL.md`         |
| `copilot`  | GitHub Copilot CLI | `~/.copilot/skills/wtm/SKILL.md`        |
| `cursor`   | Cursor             | `~/.cursor/skills/wtm/SKILL.md`         |
| `gemini`   | Gemini CLI         | `~/.gemini/skills/wtm/SKILL.md`         |
| `opencode` | opencode           | `~/.config/opencode/skills/wtm/SKILL.md` |

For another agent, or to install into one project, give the skills directory
with `--dir`, e.g. `wtm skill install --dir .claude/skills`. Installing again
replaces the file.

The description lives in `internal/cli/skill.md`; update it along with this
README when wtm's behaviour changes.

## Configuration

wtm reads an optional `.wtm/` directory in the directory it's started from:

```
myproject/
├── .wtm/
│   ├── config.toml
│   ├── post_create.sh
│   └── post_delete.sh
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
`wtm create`, so you see its output and it can start interactive programs. It
runs inside the new worktree, with the [variables](#hook-variables) below.

```bash
# e.g. share an env file and install dependencies
cp "$WTM_DIR/.env" .env
npm install
```

If the script exits non-zero the worktree is kept and wtm reports an error.
In the TUI, wtm first waits for Enter so you can read the script's output.

### `.wtm/post_delete.sh`

Runs with `bash` after each successful delete, from the TUI or `wtm delete`,
and is connected to the terminal in the same way. The worktree's directory and
branch are already gone, so it runs in the directory wtm was started from.
When several worktrees are deleted together in the TUI, it runs once for each,
straight after that worktree is deleted. It isn't run for a worktree that
couldn't be deleted, or by `wtm prune`.

```bash
# e.g. remove what post_create.sh set up outside the worktree
docker compose -p "$WTM_WORKTREE_NAME" down --volumes
```

If the script exits non-zero the worktree stays deleted and wtm reports an
error, after waiting for Enter in the TUI. `wtm delete --no-hook` skips it.

### Hook variables

Both scripts are given these environment variables:

| Variable            | `post_create.sh`                                    | `post_delete.sh`                                  |
|---------------------|-----------------------------------------------------|---------------------------------------------------|
| `WTM_WORKTREE_NAME` | Name of the new worktree, relative to `worktree_path` | Name of the deleted worktree, as `wtm list` showed it |
| `WTM_WORKTREE_PATH` | Absolute path of the new worktree                   | Absolute path it had; the directory is gone       |
| `WTM_WORKTREE_ROOT` | Absolute path of `worktree_path`                    | The same                                          |
| `WTM_BRANCH`        | The new branch (currently the same as the worktree name) | The deleted branch; empty if `HEAD` was detached |
| `WTM_BASE_BRANCH`   | The branch it was created from                      | Empty                                             |
| `WTM_DIR`           | The directory wtm was started from                  | The same                                          |

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
