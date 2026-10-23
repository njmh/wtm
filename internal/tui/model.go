// Package tui is wtm's terminal interface, built with Bubble Tea: a list of
// worktrees with popovers to create, rename and delete them.
package tui

import (
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/njmh/wtm/internal/git"
	"github.com/njmh/wtm/internal/hook"
)

type mode int

const (
	modeList mode = iota
	modeCreate
	modeDelete
	modeRename
	modeError
	// modeBusy shows a popover while an operation runs and ignores keys, so
	// nothing else can start until it has finished.
	modeBusy
)

type (
	worktreesMsg struct {
		list []git.Worktree
		err  error
	}
	opDoneMsg struct {
		status string
		err    error
		// selectPath is the worktree to focus once the list is reloaded.
		selectPath string
		// postCreate is set after a successful create when there is a
		// .wtm/post_create.sh to run.
		postCreate *hook.PostCreate
	}
	postCreateDoneMsg struct {
		status string
		err    error
	}
	// deleteStepMsg is the result of deleting m.deleting[i]. Worktrees are
	// deleted one at a time so the busy popover can show progress.
	deleteStepMsg struct {
		i   int
		err error
	}
)

type model struct {
	repo      git.Repo
	worktrees []git.Worktree
	cursor    int
	offset    int // first visible row when the list scrolls
	width     int
	height    int

	mode    mode
	target  git.Worktree    // the worktree being renamed
	checked map[string]bool // paths of the worktrees checked for deleting together

	deleting   []git.Worktree // the worktrees in the delete popover, then being deleted
	warnings   []string       // UnmergedWarning for each of deleting
	deleteErrs []error        // the result of deleting each of deleting

	nameInput   textinput.Model
	baseInput   textinput.Model
	renameInput textinput.Model
	formErr     string // inline validation message in the create/rename popovers

	err        error
	status     string // the last operation's result, shown under the menu
	activity   string // what's running, shown in the busy popover
	spinner    spinner.Model
	selectPath string
}

// Run shows the worktree manager for repo until the user quits.
func Run(repo git.Repo) error {
	_, err := tea.NewProgram(newModel(repo)).Run()
	return err
}

func newModel(g git.Repo) model {
	return model{
		repo:        g,
		checked:     map[string]bool{},
		nameInput:   newInput("feature-x"),
		baseInput:   newInput("main"),
		renameInput: newInput("new-name"),
		spinner:     spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(keyStyle)),
	}
}

func newInput(placeholder string) textinput.Model {
	ti := textinput.New()
	ti.Prompt = "› "
	ti.Placeholder = placeholder
	ti.CharLimit = 200
	return ti
}

func (m model) Init() tea.Cmd {
	return m.loadWorktrees()
}

func (m model) loadWorktrees() tea.Cmd {
	g := m.repo
	return func() tea.Msg {
		list, err := g.List()
		return worktreesMsg{list: list, err: err}
	}
}

func (m model) selected() (git.Worktree, bool) {
	if m.cursor < 0 || m.cursor >= len(m.worktrees) {
		return git.Worktree{}, false
	}
	return m.worktrees[m.cursor], true
}

// checkedWorktrees returns the checked worktrees in list order.
func (m model) checkedWorktrees() []git.Worktree {
	var list []git.Worktree
	for _, wt := range m.worktrees {
		if m.checked[wt.Path] {
			list = append(list, wt)
		}
	}
	return list
}

func (m *model) showError(err error) {
	m.err = err
	m.mode = modeError
}

// startBusy shows activity in the busy popover and returns a command that
// runs op and animates the spinner. Keys are ignored until endBusy.
func (m *model) startBusy(activity string, op tea.Cmd) tea.Cmd {
	m.mode = modeBusy
	m.activity = activity
	m.status = ""
	return tea.Batch(op, m.spinner.Tick)
}

// endBusy closes the busy popover, unless an error has already replaced it.
func (m *model) endBusy() {
	if m.mode == modeBusy {
		m.mode = modeList
	}
}

// clampScroll keeps the cursor in range and visible.
func (m *model) clampScroll() {
	if m.cursor >= len(m.worktrees) {
		m.cursor = len(m.worktrees) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	rows := m.listHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+rows {
		m.offset = m.cursor - rows + 1
	}
	m.offset = max(0, min(m.offset, len(m.worktrees)-rows))
}
