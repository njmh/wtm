// Package config reads wtm's settings from the .wtm directory.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Dir holds wtm's settings, in the directory wtm is started from.
const Dir = ".wtm"

var (
	// FileName is the config file, relative to the directory wtm runs in.
	FileName = filepath.Join(Dir, "config.toml")
	// PostCreateName is the script run after each create, relative to the
	// directory wtm runs in.
	PostCreateName = filepath.Join(Dir, "post_create.sh")
)

// Config is read from .wtm/config.toml. Every option is optional.
type Config struct {
	// WorktreePath is where new worktrees are created, relative to the
	// directory wtm is started from.
	WorktreePath string `toml:"worktree_path"`
}

// Load reads dir/.wtm/config.toml. A missing file gives the defaults.
func Load(dir string) (Config, error) {
	cfg := Config{WorktreePath: "./"}
	meta, err := toml.DecodeFile(filepath.Join(dir, FileName), &cfg)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("%s: %w", FileName, err)
	}
	// Catch typos instead of silently ignoring them.
	if keys := meta.Undecoded(); len(keys) > 0 {
		return cfg, fmt.Errorf("%s: unknown option %q", FileName, keys[0].String())
	}
	if strings.TrimSpace(cfg.WorktreePath) == "" {
		return cfg, fmt.Errorf("%s: worktree_path can't be empty (use \"./\" for the current directory)", FileName)
	}
	return cfg, nil
}

// Root resolves WorktreePath against dir.
func (c Config) Root(dir string) string {
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

// PostCreateScript is the path of dir/.wtm/post_create.sh, or "" when there
// is no such file.
func PostCreateScript(dir string) string {
	path := filepath.Join(dir, PostCreateName)
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		return ""
	}
	return path
}
