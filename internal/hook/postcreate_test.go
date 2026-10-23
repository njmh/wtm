package hook

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/njmh/wtm/internal/gittest"
)

func TestPostCreate(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(dir, ".wtm", "post_create.sh")
	// Not executable: it's run with bash.
	gittest.WriteFile(t, script, `echo "pwd=$(pwd)"
echo "dir=$WTM_DIR root=$WTM_WORKTREE_ROOT name=$WTM_WORKTREE_NAME path=$WTM_WORKTREE_PATH branch=$WTM_BRANCH base=$WTM_BASE_BRANCH"
[ "$WTM_BRANCH" != fail ] || exit 3
`)
	root := filepath.Join(dir, "trees")
	path := filepath.Join(root, "feat", "x")
	gittest.WriteFile(t, filepath.Join(path, "README"), "")

	run := func(branch string) (string, error) {
		h := NewPostCreate(script, Env{Dir: dir, Root: root, Name: "feat/x", Path: path, Branch: branch, Base: "main"})
		if h.Dir() != path {
			t.Errorf("hook runs in %s", h.Dir())
		}
		var out bytes.Buffer
		h.SetStdout(&out)
		h.SetStderr(&out)
		err := h.Run()
		return out.String(), err
	}

	out, err := run("feat/x")
	if err != nil {
		t.Fatalf("hook failed: %v\n%s", err, out)
	}
	want := "pwd=" + path + "\ndir=" + dir + " root=" + root + " name=feat/x path=" + path + " branch=feat/x base=main\n"
	if out != want {
		t.Errorf("hook output:\n%s\nwant:\n%s", out, want)
	}

	out, err = run("fail")
	if err == nil || err.Error() != "exit status 3" || !strings.Contains(out, "branch=fail") {
		t.Errorf("err = %v, output:\n%s", err, out)
	}
}
