# wtm — configuration and post-create hook

**Date:** 2026-10-06

## Goal

Add per-directory settings in a hidden `.wtm/` directory, found in the
directory wtm is started from:

- `config.toml` with one option, `worktree_path`: where worktrees are created
  and managed, relative to that directory (default `./`).
- `post_create.sh`: a bash script run after a new worktree is created, e.g. to
  install dependencies or open other programs.

## Decisions

| Question | Decision |
|----------|----------|
| TOML parser | `github.com/BurntSushi/toml` v1.6.0 |
| Missing `.wtm/` or files | Defaults are used and no script runs. Neither file is required |
| Invalid config | wtm exits before starting with `wtm: .wtm/config.toml: …`. Unknown keys are errors, so typos aren't ignored. An empty `worktree_path` is an error |
| Path forms | Relative paths are resolved against the start directory. `../trees` (outside it) and absolute paths also work. Symlinks are resolved, as for the start directory |
| How worktrees are named | By their path inside `worktree_path`. Worktrees outside it (e.g. the bare repo, or ones made before the setting) are shown as `./path` relative to the start directory, or in full |
| Renaming a worktree outside `worktree_path` | Moves it into `worktree_path`. The rename field is pre-filled with its folder name |
| How the script runs | `bash .wtm/post_create.sh`, so it doesn't need to be executable. It runs inside the new worktree |
| Script output | wtm hands over the terminal with `tea.Exec`, so output appears live, colours and progress bars work, and interactive programs can run. The screen returns to wtm when the script ends |
| Script failure | The worktree is kept. wtm prints the exit status and waits for Enter so the output can be read, then shows an error dialog. The script only runs after a successful create |
| When files are read | `config.toml` once at startup. `post_create.sh` is looked up at each create, so edits apply straight away |

Environment variables passed to the script: `WTM_WORKTREE_PATH`, `WTM_BRANCH`,
`WTM_BASE_BRANCH`, `WTM_DIR`.

## Changes

| File | Change |
|------|--------|
| `config.go` | New. `Config`, `loadConfig`, `postCreateScript`, and `postCreateHook`, which implements `tea.ExecCommand` |
| `git.go` | `Git` has a `Root` (worktree path, defaults to `Dir`). New `Path(name)` and `InRoot(wt)`. Create and rename go into `Root`. Names are relative to `Root`. Empty parent folders are cleaned up to `Root`, never removing `Root` itself |
| `model.go` | Create attaches the hook when the script exists. A new `postCreateDoneMsg` reports the result. The rename pre-fill handles worktrees outside the root |
| `view.go` | The title line shows `→ worktrees in ./trees` when it isn't the current directory. A long start directory is shortened from the left so this note stays visible. The create popover says where the worktree goes and that the script will run |
| `main.go` | Loads the config before starting |
| `config_test.go` | New. Config parsing and errors, create/rename/delete with a worktree path, the script's environment, working directory and failure, and the model flow |
| `README.md` | New *Configuration* section |

## Verification

- `go vet ./...` is clean and `go test ./...` passes.
- I ran the real binary in tmux with `worktree_path = "trees"` and a
  `post_create.sh`. A create showed the script's output, then returned to wtm
  with the status *Created worktree feat from main. Ran .wtm/post_create.sh.*
  A failing script showed its output and the Enter prompt, then the error
  dialog. Delete removed `trees/bad` and kept `trees/`. An unknown key and
  invalid TOML each gave a clear startup error.

## Known limitations

- Output from successful scripts isn't kept once wtm redraws. It's still in
  the terminal's normal screen after quitting.
- The config isn't reloaded while wtm is running.
