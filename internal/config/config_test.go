package config

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/njmh/wtm/internal/gittest"
)

func TestLoad(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, FileName)

	// No .wtm directory: defaults.
	cfg, err := Load(dir)
	if err != nil || cfg.WorktreePath != "./" || cfg.Root(dir) != dir {
		t.Fatalf("default config = %+v, root %q, err %v", cfg, cfg.Root(dir), err)
	}

	// An empty file also gives the defaults.
	gittest.WriteFile(t, cfgPath, "")
	if cfg, err := Load(dir); err != nil || cfg.Root(dir) != dir {
		t.Fatalf("empty config = %+v, err %v", cfg, err)
	}

	gittest.WriteFile(t, cfgPath, "# where worktrees go\nworktree_path = \"trees\"\n")
	cfg, err = Load(dir)
	if err != nil || cfg.Root(dir) != filepath.Join(dir, "trees") {
		t.Fatalf("config = %+v, root %q, err %v", cfg, cfg.Root(dir), err)
	}

	gittest.WriteFile(t, cfgPath, "worktree_path = \"../sibling\"\n")
	if cfg, _ := Load(dir); cfg.Root(dir) != filepath.Join(filepath.Dir(dir), "sibling") {
		t.Errorf("root = %q", cfg.Root(dir))
	}

	for content, want := range map[string]string{
		"worktree_path = ":               "config.toml",
		"worktree_pth = \"trees\"":       `unknown option "worktree_pth"`,
		"worktree_path = \"\"":           "can't be empty",
		"worktree_path = 3":              "config.toml",
		"[section]\nworktree_path=\"x\"": "unknown option",
	} {
		gittest.WriteFile(t, cfgPath, content)
		if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("config %q: err = %v, want it to mention %q", content, err, want)
		}
	}
}

func TestPostCreateScript(t *testing.T) {
	dir := t.TempDir()
	if PostCreateScript(dir) != "" {
		t.Fatal("found a post_create.sh that doesn't exist")
	}
	gittest.WriteFile(t, filepath.Join(dir, PostCreateName, "nested"), "")
	if PostCreateScript(dir) != "" {
		t.Fatal("a directory named post_create.sh was treated as the script")
	}
	other := t.TempDir()
	script := filepath.Join(other, PostCreateName)
	gittest.WriteFile(t, script, "true\n")
	if PostCreateScript(other) != script {
		t.Fatal("post_create.sh not found")
	}
}
