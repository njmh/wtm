package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
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

func TestCreate(t *testing.T) {
	dir := gittest.NewContainer(t)
	gittest.WriteFile(t, filepath.Join(dir, ".wtm", "config.toml"), "worktree_path = \"trees\"\n")

	if out := mustRun(t, dir, "create", "feature/a"); out != "Created worktree feature/a from main at ./trees/feature/a.\n" {
		t.Errorf("output = %q", out)
	}
	gittest.Run(t, filepath.Join(dir, "trees", "feature", "a"), "git", "commit", "-q", "--allow-empty", "-m", "on a")
	mustRun(t, dir, "new", "b", "--base", "feature/a")
	got := worktrees(t, dir)
	if got["trees/b"].Branch != "b" || got["trees/b"].Head != got["trees/feature/a"].Head {
		t.Errorf("b = %+v, want branch b at feature/a's head", got["trees/b"])
	}

	for _, args := range [][]string{{"create", "feature/a"}, {"create", "bad name"}, {"create", "c", "-b", "nope"}} {
		if _, _, err := run(t, dir, "", args...); err == nil {
			t.Errorf("wtm %s succeeded", strings.Join(args, " "))
		}
	}
	if _, _, err := run(t, dir, "", "create"); err == nil || !strings.HasPrefix(err.Error(), "missing <name>\n") {
		t.Errorf("no name: err = %v", err)
	}
}

func TestCreateRunsPostCreateHook(t *testing.T) {
	dir := gittest.NewContainer(t)
	gittest.WriteFile(t, filepath.Join(dir, ".wtm", "post_create.sh"), `echo "in $(pwd) for $WTM_WORKTREE_NAME from $WTM_BASE_BRANCH"
read answer
echo "got $answer"
echo oops >&2
[ "$WTM_BRANCH" != fail ] || exit 3
`)
	out, errOut, err := run(t, dir, "yes\n", "create", "feat")
	if err != nil {
		t.Fatal(err)
	}
	want := "Created worktree feat from main at ./feat.\nRunning .wtm/post_create.sh…\nin " + filepath.Join(dir, "feat") + " for feat from main\ngot yes\n"
	if out != want || errOut != "oops\n" {
		t.Errorf("stdout:\n%s\nwant:\n%s\nstderr: %q", out, want, errOut)
	}

	out, _, err = run(t, dir, "", "create", "quiet", "--no-hook")
	if err != nil || out != "Created worktree quiet from main at ./quiet.\n" {
		t.Errorf("--no-hook: err = %v, output %q", err, out)
	}

	_, _, err = run(t, dir, "", "create", "fail")
	if err == nil || err.Error() != "the worktree was created, but .wtm/post_create.sh failed: exit status 3" {
		t.Errorf("failing hook: err = %v", err)
	}
	if _, ok := worktrees(t, dir)["fail"]; !ok {
		t.Error("worktree was not kept after the hook failed")
	}
}

// pretendTerminal makes stdin count as a terminal for the rest of the test.
func pretendTerminal(t *testing.T) {
	old := isTerminal
	isTerminal = func(io.Reader) bool { return true }
	t.Cleanup(func() { isTerminal = old })
}

func TestDelete(t *testing.T) {
	dir := gittest.NewContainer(t)
	for _, name := range []string{"a", "b", "c"} {
		mustRun(t, dir, "create", name)
	}

	// Without a terminal, --yes is required.
	_, _, err := run(t, dir, "y\n", "delete", "a")
	if err == nil || !strings.Contains(err.Error(), "use --yes") {
		t.Errorf("err = %v", err)
	}
	if out := mustRun(t, dir, "delete", "a", "--yes"); out != "Deleted worktree a.\n" {
		t.Errorf("output = %q", out)
	}

	pretendTerminal(t)
	out, prompt, err := run(t, dir, "n\n", "rm", "b")
	if err != nil || out != "Nothing was deleted.\n" {
		t.Errorf("declined: err = %v, output %q", err, out)
	}
	if want := "Delete worktree b? This permanently removes:\n\n  directory  ./b\n  branch     b\n\n[y/N] "; prompt != want {
		t.Errorf("prompt:\n%s\nwant:\n%s", prompt, want)
	}
	if _, _, err := run(t, dir, "", "rm", "b"); err != nil {
		t.Errorf("no answer: %v", err)
	}
	if _, ok := worktrees(t, dir)["b"]; !ok {
		t.Fatal("b was deleted without a yes")
	}
	if _, _, err := run(t, dir, "Yes\n", "rm", "b"); err != nil {
		t.Fatal(err)
	}
	// Found by path too.
	mustRun(t, dir, "delete", "./c/", "-y")
	if got := worktrees(t, dir); len(got) != 1 {
		t.Errorf("worktrees left: %v", got)
	}

	// Errors come before the prompt.
	for args, want := range map[string]string{
		"rm .bare": "the bare repository (.bare) can't be deleted",
		"rm nope":  `no worktree named "nope"`,
	} {
		_, prompt, err := run(t, dir, "y\n", strings.Fields(args)...)
		if err == nil || err.Error() != want || prompt != "" {
			t.Errorf("wtm %s: err = %v, prompt %q", args, err, prompt)
		}
	}
}

func TestDeleteRefusesToLoseWork(t *testing.T) {
	dir := gittest.NewContainer(t)
	mustRun(t, dir, "create", "dirty")
	gittest.WriteFile(t, filepath.Join(dir, "dirty", "f.txt"), "x")
	if _, _, err := run(t, dir, "", "delete", "dirty", "-y"); err == nil {
		t.Fatal("deleted a worktree with untracked files")
	}
	if _, ok := worktrees(t, dir)["dirty"]; !ok {
		t.Error("worktree was removed")
	}
}

func TestRename(t *testing.T) {
	dir := gittest.NewContainer(t)
	mustRun(t, dir, "create", "old")
	if out := mustRun(t, dir, "rename", "old", "feature/new"); out != "Renamed old to feature/new.\n" {
		t.Errorf("output = %q", out)
	}
	// Found by branch.
	mustRun(t, dir, "mv", "feature/new", "newer")
	got := worktrees(t, dir)
	if got["newer"].Branch != "newer" || len(got) != 2 {
		t.Errorf("worktrees = %+v", got)
	}

	for args, want := range map[string]string{
		"mv newer newer": "newer is already named newer",
		"mv .bare x":     "the bare repository (.bare) can't be renamed",
		"mv nope x":      `no worktree named "nope"`,
		"mv newer":       "missing <new-name>\nRun 'wtm rename --help' for usage.",
	} {
		_, _, err := run(t, dir, "", strings.Fields(args)...)
		if err == nil || err.Error() != want {
			t.Errorf("wtm %s: err = %v, want %q", args, err, want)
		}
	}
}

func TestPrune(t *testing.T) {
	dir := gittest.NewContainer(t)
	if out := mustRun(t, dir, "prune"); out != "Nothing to prune.\n" {
		t.Errorf("output = %q", out)
	}
	for _, name := range []string{"a", "b"} {
		mustRun(t, dir, "create", name)
		if err := os.RemoveAll(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	if out := mustRun(t, dir, "prune"); out != "Pruned 2 stale worktree entries.\n" {
		t.Errorf("output = %q", out)
	}
}

func TestCompletion(t *testing.T) {
	dir := gittest.NewContainer(t)
	mustRun(t, dir, "create", "feat")
	gittest.Run(t, dir, "git", "branch", "other")

	for args, want := range map[string]string{
		"rm ":              "feat\tfeat\n:4\n",
		"mv feat ":         ":4\n",
		"create x --base ": "feat\nmain\nother\n:4\n",
	} {
		// The trailing space makes the last word "": the word being completed.
		out := mustRun(t, dir, append([]string{"__complete"}, strings.Split(args, " ")...)...)
		if out != want {
			t.Errorf("complete %q:\n%s\nwant:\n%s", args, out, want)
		}
	}
}
