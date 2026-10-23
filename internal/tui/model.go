// Package tui is wtm's terminal interface, built with Bubble Tea: a list of
// worktrees with popovers to create, rename and delete them.
package tui

import (
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
)

type model struct {
	repo      git.Repo
	worktrees []git.Worktree
	cursor    int
	offset    int // first visible row when the list scrolls
	width     int
	height    int

	mode   mode
	target git.Worktree // the worktree being deleted or renamed

	nameInput   textinput.Model
	baseInput   textinput.Model
	renameInput textinput.Model
	formErr     string // inline validation message in the create/rename popovers

	err        error
	status     string
	busy       bool
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
		nameInput:   newInput("feature-x"),
		baseInput:   newInput("main"),
		renameInput: newInput("new-name"),
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

func (m *model) showError(err error) {
	m.err = err
	m.mode = modeError
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
