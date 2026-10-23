package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/njmh/wtm/internal/git"
	"github.com/njmh/wtm/internal/gittest"
)

// run runs wtm with args in dir, with stdin, and returns what it printed.
func run(t *testing.T, dir, stdin string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	t.Chdir(dir)
	cmd := newRootCmd()
	var out, errOut bytes.Buffer
	cmd.SetArgs(args)
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	err = cmd.Execute()
	return out.String(), errOut.String(), err
}

// mustRun is run for commands that should succeed, returning their output.
func mustRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, errOut, err := run(t, dir, "", args...)
	if err != nil {
		t.Fatalf("wtm %s: %v\nstdout:\n%s\nstderr:\n%s", strings.Join(args, " "), err, out, errOut)
	}
	return out
}

// worktrees lists dir's worktrees by name.
func worktrees(t *testing.T, dir string) map[string]git.Worktree {
	t.Helper()
	repo := git.Repo{Dir: dir}
	list, err := repo.List()
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]git.Worktree{}
	for _, wt := range list {
		byName[repo.Name(wt)] = wt
	}
	return byName
}

func TestList(t *testing.T) {
	dir := gittest.NewContainer(t)
	gittest.Run(t, dir, "git", "worktree", "add", "-q", "-b", "feature/a", "./feature/a", "main")
	head := worktrees(t, dir)["feature/a"].Head[:7]

	want := "WORKTREE   BRANCH     HEAD\n" +
		".bare      —" + strings.Repeat(" ", 10+9) + "bare\n" +
		"feature/a  feature/a  " + head + "\n"
	for _, args := range [][]string{{"list"}, {"ls"}} {
		if out := mustRun(t, dir, args...); out != want {
			t.Errorf("wtm %s:\n%s\nwant:\n%s", args[0], out, want)
		}
	}

	var got []worktreeJSON
	if err := json.Unmarshal([]byte(mustRun(t, dir, "list", "--json")), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !got[0].Bare || !got[0].Main || got[1].Name != "feature/a" ||
		got[1].Branch != "feature/a" || got[1].Path != filepath.Join(dir, "feature", "a") {
		t.Errorf("json = %+v", got)
	}
}

func TestUsageErrors(t *testing.T) {
	dir := gittest.NewContainer(t)
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"lst"}, "unknown command \"lst\" (did you mean \"list\"?)\nRun 'wtm --help' for usage."},
		{[]string{"list", "x"}, "unexpected argument \"x\"\nRun 'wtm list --help' for usage."},
		{[]string{"list", "--nope"}, "unknown flag: --nope\nRun 'wtm list --help' for usage."},
	} {
		_, _, err := run(t, dir, "", tc.args...)
		if err == nil || err.Error() != tc.want {
			t.Errorf("wtm %s: err = %v, want %q", strings.Join(tc.args, " "), err, tc.want)
		}
	}
}

func TestNotARepo(t *testing.T) {
	_, _, err := run(t, t.TempDir(), "", "list")
	if err == nil || !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("err = %v", err)
	}
}

func TestBadConfig(t *testing.T) {
	dir := gittest.NewContainer(t)
	gittest.WriteFile(t, filepath.Join(dir, ".wtm", "config.toml"), "nope = 1\n")
	_, _, err := run(t, dir, "", "list")
	if err == nil || !strings.Contains(err.Error(), `unknown option "nope"`) {
		t.Errorf("err = %v", err)
	}
}
