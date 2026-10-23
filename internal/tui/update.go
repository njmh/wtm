package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/njmh/wtm/internal/config"
	"github.com/njmh/wtm/internal/git"
	"github.com/njmh/wtm/internal/hook"
)

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.clampScroll()
		return m, nil

	case worktreesMsg:
		if msg.err != nil {
			m.worktrees = nil
			clear(m.checked)
			m.showError(msg.err)
			return m, nil
		}
		m.worktrees = msg.list
		// Forget checks on worktrees that are gone, e.g. once they're deleted.
		listed := make(map[string]bool, len(m.worktrees))
		for _, wt := range m.worktrees {
			listed[wt.Path] = true
		}
		for path := range m.checked {
			if !listed[path] {
				delete(m.checked, path)
			}
		}
		if m.selectPath != "" {
			for i, wt := range m.worktrees {
				if wt.Path == m.selectPath {
					m.cursor = i
				}
			}
			m.selectPath = ""
		}
		m.clampScroll()
		return m, nil

	case opDoneMsg:
		return m.finish(msg)

	case deleteStepMsg:
		m.deleteErrs[msg.i] = msg.err
		if next := msg.i + 1; next < len(m.deleting) {
			m.activity = m.deleteActivity(next)
			return m, m.deleteStep(next)
		}
		return m.finish(m.deleteResult())

	case postCreateDoneMsg:
		m.endBusy()
		m.status = msg.status + " Ran " + config.PostCreateName + "."
		if msg.err != nil {
			m.status = ""
			m.showError(fmt.Errorf("the worktree was created, but %s failed: %w", config.PostCreateName, msg.err))
		}
		return m, nil

	case spinner.TickMsg:
		// Dropping the tick when idle stops the animation until the next startBusy.
		if m.mode != modeBusy {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		switch m.mode {
		case modeBusy:
			return m, nil
		case modeList:
			return m.updateList(msg)
		case modeCreate:
			return m.updateCreate(msg)
		case modeDelete:
			return m.updateDelete(msg)
		case modeRename:
			return m.updateRename(msg)
		case modeError:
			switch msg.String() {
			case "enter", "esc", "q", "space":
				m.mode = modeList
				m.err = nil
			}
			return m, nil
		}
	}

	// Anything else (e.g. cursor blinks) goes to the active text inputs.
	var cmd tea.Cmd
	switch m.mode {
	case modeCreate:
		var c1, c2 tea.Cmd
		m.nameInput, c1 = m.nameInput.Update(msg)
		m.baseInput, c2 = m.baseInput.Update(msg)
		cmd = tea.Batch(c1, c2)
	case modeRename:
		m.renameInput, cmd = m.renameInput.Update(msg)
	}
	return m, cmd
}

// finish ends the busy popover with an operation's result and reloads the list.
func (m model) finish(msg opDoneMsg) (tea.Model, tea.Cmd) {
	m.endBusy()
	m.status = msg.status
	m.selectPath = msg.selectPath
	if msg.err != nil {
		m.status = ""
		m.showError(msg.err)
	}
	// Reload even on failure: a partly applied operation still changes the list.
	if msg.err == nil && msg.postCreate != nil {
		status := msg.status
		return m, tea.Batch(m.loadWorktrees(), m.startBusy("Running "+config.PostCreateName+"…",
			tea.Exec(&pauseOnFailure{PostCreate: msg.postCreate}, func(err error) tea.Msg {
				return postCreateDoneMsg{status: status, err: err}
			})))
	}
	return m, m.loadWorktrees()
}

func (m model) updateList(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "Q":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.worktrees)-1 {
			m.cursor++
		}
	case "home", "g":
		m.cursor = 0
	case "end", "G":
		m.cursor = max(len(m.worktrees)-1, 0)
	case "space":
		// The main worktree can't be deleted, so it has no checkbox.
		wt, ok := m.selected()
		if !ok || m.repo.CheckChangeable(wt, "delete") != nil {
			return m, nil
		}
		if m.checked[wt.Path] {
			delete(m.checked, wt.Path)
		} else {
			m.checked[wt.Path] = true
		}
	case "esc":
		clear(m.checked)
	case "c", "C":
		m.mode = modeCreate
		m.formErr = ""
		m.nameInput.Reset()
		m.baseInput.SetValue("main")
		m.baseInput.Blur()
		return m, m.nameInput.Focus()
	case "d", "D":
		// Delete the checked worktrees, or the selected one if none are.
		targets := m.checkedWorktrees()
		if len(targets) == 0 {
			wt, ok := m.actionable("delete")
			if !ok {
				return m, nil
			}
			targets = []git.Worktree{wt}
		}
		m.deleting = targets
		m.warnings = make([]string, len(targets))
		for i, wt := range targets {
			m.warnings[i] = m.repo.UnmergedWarning(wt)
		}
		m.mode = modeDelete
	case "r", "R":
		// Rename acts on one worktree, so it's disabled while any are checked.
		if len(m.checked) > 0 {
			return m, nil
		}
		wt, ok := m.actionable("rename")
		if !ok {
			return m, nil
		}
		m.target = wt
		m.mode = modeRename
		m.formErr = ""
		// Renaming moves the worktree into the worktree path, so for one outside
		// it suggest just its folder name.
		name, ok := m.repo.InRoot(wt)
		if !ok {
			name = filepath.Base(wt.Path)
		}
		m.renameInput.SetValue(name)
		m.renameInput.CursorEnd()
		return m, m.renameInput.Focus()
	case "p", "P":
		g := m.repo
		return m, m.startBusy("Pruning worktrees…", func() tea.Msg {
			n, err := g.Prune()
			status := "Nothing to prune."
			if n > 0 {
				status = fmt.Sprintf("Pruned %d stale worktree %s.", n, plural(n, "entry", "entries"))
			}
			return opDoneMsg{status: status, err: err}
		})
	}
	m.clampScroll()
	return m, nil
}

// actionable returns the selected worktree if it can be deleted or renamed,
// and otherwise explains why not in the error dialog.
func (m *model) actionable(verb string) (git.Worktree, bool) {
	wt, ok := m.selected()
	if !ok {
		return wt, false
	}
	if err := m.repo.CheckChangeable(wt, verb); err != nil {
		m.showError(err)
		return wt, false
	}
	return wt, true
}

func (m model) updateCreate(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeList
		return m, nil
	case "tab", "shift+tab", "up", "down":
		if m.nameInput.Focused() {
			m.nameInput.Blur()
			return m, m.baseInput.Focus()
		}
		m.baseInput.Blur()
		return m, m.nameInput.Focus()
	case "enter":
		name := strings.TrimSpace(m.nameInput.Value())
		base := strings.TrimSpace(m.baseInput.Value())
		if name == "" {
			m.formErr = "Enter a name for the worktree."
			m.baseInput.Blur()
			return m, m.nameInput.Focus()
		}
		if base == "" {
			base = "main"
		}
		g := m.repo
		return m, m.startBusy(fmt.Sprintf("Creating %s from %s…", name, base), func() tea.Msg {
			err := g.Create(name, base)
			msg := opDoneMsg{
				status:     fmt.Sprintf("Created worktree %s from %s.", name, base),
				err:        err,
				selectPath: g.Path(name),
			}
			if script := config.PostCreateScript(g.Dir); err == nil && script != "" {
				msg.postCreate = hook.NewPostCreate(script, hook.Env{
					Dir:    g.Dir,
					Root:   g.WorktreeRoot(),
					Name:   name,
					Path:   g.Path(name),
					Branch: name,
					Base:   base,
				})
			}
			return msg
		})
	}
	var cmd tea.Cmd
	if m.nameInput.Focused() {
		m.nameInput, cmd = m.nameInput.Update(msg)
	} else {
		m.baseInput, cmd = m.baseInput.Update(msg)
	}
	m.formErr = ""
	return m, cmd
}

func (m model) updateDelete(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		m.deleteErrs = make([]error, len(m.deleting))
		return m, m.startBusy(m.deleteActivity(0), m.deleteStep(0))
	case "n", "N", "esc", "q":
		m.mode = modeList
	}
	return m, nil
}

// deleteStep deletes m.deleting[i]. Each step's deleteStepMsg starts the next,
// so they run one after another.
func (m model) deleteStep(i int) tea.Cmd {
	g, wt := m.repo, m.deleting[i]
	return func() tea.Msg {
		return deleteStepMsg{i: i, err: g.Delete(wt)}
	}
}

func (m model) deleteActivity(i int) string {
	name := m.repo.Name(m.deleting[i])
	if len(m.deleting) == 1 {
		return fmt.Sprintf("Deleting %s…", name)
	}
	return fmt.Sprintf("Deleting %s… (%d of %d)", name, i+1, len(m.deleting))
}

// deleteResult reports how a delete went. A failed single delete shows git's
// error as it is; when several were deleted, the failures are listed.
func (m model) deleteResult() opDoneMsg {
	n := len(m.deleting)
	if n == 1 {
		return opDoneMsg{status: fmt.Sprintf("Deleted worktree %s.", m.repo.Name(m.deleting[0])), err: m.deleteErrs[0]}
	}
	var failed []string
	for i, err := range m.deleteErrs {
		if err != nil {
			failed = append(failed, "• "+m.repo.Name(m.deleting[i])+": "+briefError(err))
		}
	}
	if len(failed) == 0 {
		return opDoneMsg{status: fmt.Sprintf("Deleted %d worktrees.", n)}
	}
	return opDoneMsg{err: fmt.Errorf("deleted %d of %d worktrees. %s:\n\n%s",
		n-len(failed), n, plural(len(failed), "This one wasn't", "These weren't"), strings.Join(failed, "\n"))}
}

// briefError is err on one line, without the command a git error starts
// with, for listing several errors together.
func briefError(err error) string {
	msg := err.Error()
	// A type assertion rather than errors.As: that would also find a git
	// error wrapped inside another, and drop the explanation around it.
	if gerr, ok := err.(*git.Error); ok && strings.TrimSpace(gerr.Output) != "" {
		msg = gerr.Output
	}
	return strings.Join(strings.Fields(msg), " ")
}

func (m model) updateRename(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeList
		return m, nil
	case "enter":
		wt := m.target
		oldName := m.repo.Name(wt)
		newName := strings.TrimSpace(m.renameInput.Value())
		if newName == "" {
			m.formErr = "Enter a new name."
			return m, nil
		}
		if newName == oldName && (wt.Branch == "" || wt.Branch == newName) {
			m.formErr = "That's already its name."
			return m, nil
		}
		g := m.repo
		return m, m.startBusy(fmt.Sprintf("Renaming %s to %s…", oldName, newName), func() tea.Msg {
			err := g.Rename(wt, newName)
			return opDoneMsg{
				status:     fmt.Sprintf("Renamed %s to %s.", oldName, newName),
				err:        err,
				selectPath: g.Path(newName),
			}
		})
	}
	var cmd tea.Cmd
	m.renameInput, cmd = m.renameInput.Update(msg)
	m.formErr = ""
	return m, cmd
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
