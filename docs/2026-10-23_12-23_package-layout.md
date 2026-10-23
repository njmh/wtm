# wtm — package layout

**Date:** 2026-10-23

## Goal

Move the Go code out of a single `package main` in the repository root into
packages with clear boundaries, keeping `main.go` at the root as the entry
point. No behaviour changes.

## Layout

```
wtm/
├── main.go                    # wiring only: find git, load config, run the TUI
└── internal/
    ├── git/                   # runs git; no UI imports
    │   ├── git.go             # Repo, run, Error
    │   ├── worktree.go        # Worktree, List, parseWorktrees
    │   ├── ops.go             # Create, Delete, Rename, Prune
    │   └── paths.go           # Name, InRoot, Path, RelInside, removeEmptyParents
    ├── gittest/gittest.go     # test fixtures: NewContainer, Run, WriteFile
    ├── config/config.go       # the .wtm directory: Load, Root, PostCreateScript
    ├── hook/postcreate.go     # runs post_create.sh (a tea.ExecCommand)
    └── tui/
        ├── model.go           # state, messages, Run
        ├── update.go          # key handling for each mode
        ├── view.go            # main screen and list
        ├── modal.go           # popovers and overlay
        └── styles.go          # colours and styles
```

Imports only point one way:

```
main ──► config, git, tui
tui  ──► config, git, hook
hook ──► config
git, config ──► (no internal packages)
```

## Decisions

| Question | Decision |
|----------|----------|
| Module path | `github.com/njmh/wtm`, matching the GitHub remote. This is the Go convention and makes `go install github.com/njmh/wtm@latest` work |
| Entry point | `main.go` stays in the root rather than `cmd/wtm/`, since there's only one binary |
| `internal/` | Go won't let code outside this module import packages under `internal/`, so they can change freely without being a public API |
| Type names | `git.Git` became `git.Repo` and `git.GitError` became `git.Error`. Go code reads as `package.Name`, so repeating the package name in the type ("stutter") is avoided |
| Hook inputs | `hook.NewPostCreate` takes an `Env` struct instead of the repository, so `hook` doesn't depend on `git` and its test needs no repository |
| `.wtm` paths | `config.FileName` and `config.PostCreateName` replace five copies of `filepath.Join(configDir, "post_create.sh")` |
| Test fixtures | `internal/gittest` returns a directory rather than a `git.Repo`. That lets `git`'s own tests use it: a package's tests can't import a package that imports it back (an import cycle) |
| TUI entry | `tui.Run(repo)` is the only exported function; the model stays unexported |

## Changes

| File | Change |
|------|--------|
| `go.mod` | Module path `github.com/njmh/wtm` |
| `git.go` → `internal/git/` | Split into four files; `Git`/`GitError` renamed; `root()` and `relInside` exported as `WorktreeRoot` and `RelInside` |
| `git_test.go` → `internal/git/git_test.go` | Uses `gittest`. Gains `TestWorktreePath`, which only tests git behaviour |
| `config.go` → `internal/config/`, `internal/hook/` | Config loading and the hook runner split apart |
| `config_test.go` | Split: `config.TestLoad`, a new `config.TestPostCreateScript`, `hook.TestPostCreate`, and the two UI tests in `internal/tui/config_test.go` |
| `model.go`, `view.go` → `internal/tui/` | Split into `model.go`, `update.go`, `view.go`, `modal.go` and `styles.go`. The model's `git` field is now `repo` |
| `model_test.go` → `internal/tui/` | Uses `gittest` |
| `main.go` | Calls `config.Load` and `tui.Run` |
| `README.md` | `go install` instructions |

## Verification

- `gofmt -l .` and `go vet ./...` are clean, and `go test ./...` passes. All 21
  original tests still run, plus the new `TestPostCreateScript`.
- I ran the real binary in tmux with `worktree_path = "trees"` and a
  `post_create.sh`. Create ran the hook (*Created worktree feat from main. Ran
  .wtm/post_create.sh.*), rename to `fix` and delete worked, and `trees/` was
  kept.
