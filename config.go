package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// configDir holds wtm's settings, in the directory wtm is started from.
const configDir = ".wtm"

// Config is read from .wtm/config.toml. Every option is optional.
type Config struct {
	// WorktreePath is where new worktrees are created, relative to the
	// directory wtm is started from.
	WorktreePath string `toml:"worktree_path"`
}

// loadConfig reads dir/.wtm/config.toml. A missing file gives the defaults.
func loadConfig(dir string) (Config, error) {
	cfg := Config{WorktreePath: "./"}
	path := filepath.Join(dir, configDir, "config.toml")
	meta, err := toml.DecodeFile(path, &cfg)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, nil
	}
	name := filepath.Join(configDir, "config.toml")
	if err != nil {
		return cfg, fmt.Errorf("%s: %w", name, err)
	}
	// Catch typos instead of silently ignoring them.
	if keys := meta.Undecoded(); len(keys) > 0 {
		return cfg, fmt.Errorf("%s: unknown option %q", name, keys[0].String())
	}
	if strings.TrimSpace(cfg.WorktreePath) == "" {
		return cfg, fmt.Errorf("%s: worktree_path can't be empty (use \"./\" for the current directory)", name)
	}
	return cfg, nil
}

// root resolves WorktreePath against dir.
func (c Config) root(dir string) string {
	root := c.WorktreePath
	if !filepath.IsAbs(root) {
		root = filepath.Join(dir, root)
	}
	// git reports resolved paths, so resolve symlinks for name comparisons.
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		return resolved
	}
	return filepath.Clean(root)
}

// postCreateScript is the path of dir/.wtm/post_create.sh, or "" when there
// is no such file.
func postCreateScript(dir string) string {
	path := filepath.Join(dir, configDir, "post_create.sh")
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		return ""
	}
	return path
}

// postCreateHook runs .wtm/post_create.sh with bash inside a new worktree. It
// takes over the terminal (see tea.Exec), so the script's output is shown as
// it runs and it can start interactive programs.
type postCreateHook struct {
	cmd *exec.Cmd
}

func newPostCreateHook(script string, g Git, name, branch, base string) *postCreateHook {
	path := g.Path(name)
	cmd := exec.Command("bash", script)
	cmd.Dir = path
	cmd.Env = append(os.Environ(),
		"WTM_DIR="+g.Dir,
		"WTM_WORKTREE_ROOT="+g.root(),
		"WTM_WORKTREE_NAME="+name,
		"WTM_WORKTREE_PATH="+path,
		"WTM_BRANCH="+branch,
		"WTM_BASE_BRANCH="+base,
	)
	return &postCreateHook{cmd: cmd}
}

func (h *postCreateHook) SetStdin(r io.Reader)  { h.cmd.Stdin = r }
func (h *postCreateHook) SetStdout(w io.Writer) { h.cmd.Stdout = w }
func (h *postCreateHook) SetStderr(w io.Writer) { h.cmd.Stderr = w }

// Run runs the script. When it fails, its output is still on screen, so wait
// for Enter before handing the terminal back to wtm.
func (h *postCreateHook) Run() error {
	err := h.cmd.Run()
	if err != nil {
		out := h.cmd.Stdout
		if out == nil {
			out = os.Stdout
		}
		fmt.Fprintf(out, "\nwtm: %s failed (%v). Press Enter to return to wtm.", filepath.Join(configDir, "post_create.sh"), err)
		if h.cmd.Stdin != nil {
			buf := make([]byte, 1)
			for {
				n, rerr := h.cmd.Stdin.Read(buf)
				if rerr != nil || (n == 1 && (buf[0] == '\n' || buf[0] == '\r')) {
					break
				}
			}
		}
	}
	return err
}
