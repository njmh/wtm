---
name: wtm
description: Manage git worktrees with wtm (Worktree Manager). Use when the user mentions wtm, a .wtm directory, .wtm/config.toml or a post_create.sh script, or asks to list, create, rename or delete worktrees in a project that uses wtm.
---

# wtm

wtm is a command-line tool for managing a repository's git worktrees.

Before running wtm or writing anything under `.wtm/`, run:

```sh
wtm skill
```

and read all of its output. It describes the installed version's commands,
its configuration and the `post_create.sh` hook, and it replaces anything you
remember about wtm.

If the command isn't found, tell the user that wtm isn't on `PATH` rather
than guessing how it works.
