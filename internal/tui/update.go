package tui

import (
	"fmt"
	"path/filepath"
	"strings"

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
			m.showError(msg.err)
			return m, nil
		}
		m.worktrees = msg.list
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
		m.busy = false
		m.status = msg.status
		m.selectPath = msg.selectPath
		if msg.err != nil {
			m.status = ""
			m.showError(msg.err)
		}
		// Reload even on failure: a partly applied operation still changes the list.
		if msg.err == nil && msg.postCreate != nil {
			m.busy = true
			m.status = "Running " + config.PostCreateName + "…"
			status := msg.status
			return m, tea.Batch(m.loadWorktrees(), tea.Exec(msg.postCreate, func(err error) tea.Msg {
				return postCreateDoneMsg{status: status, err: err}
			}))
		}
		return m, m.loadWorktrees()

	case postCreateDoneMsg:
		m.busy = false
		m.status = msg.status + " Ran " + config.PostCreateName + "."
		if msg.err != nil {
			m.status = ""
			m.showError(fmt.Errorf("the worktree was created, but %s failed: %w", config.PostCreateName, msg.err))
		}
		return m, nil

	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		switch m.mode {
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
	case "c", "C":
		if m.busy {
			return m, nil
		}
		m.mode = modeCreate
		m.formErr = ""
		m.nameInput.Reset()
		m.baseInput.SetValue("main")
		m.baseInput.Blur()
		return m, m.nameInput.Focus()
	case "d", "D":
		wt, ok := m.actionable("delete")
		if !ok || m.busy {
			return m, nil
		}
		m.target = wt
		m.mode = modeDelete
	case "r", "R":
		wt, ok := m.actionable("rename")
		if !ok || m.busy {
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
		if m.busy {
			return m, nil
		}
		m.busy = true
		m.status = "Pruning worktrees…"
		g := m.repo
		return m, func() tea.Msg {
			n, err := g.Prune()
			status := "Nothing to prune."
			if n > 0 {
				status = fmt.Sprintf("Pruned %d stale worktree %s.", n, plural(n, "entry", "entries"))
			}
			return opDoneMsg{status: status, err: err}
		}
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
	if wt.Main {
		kind := "main worktree"
		if wt.Bare {
			kind = "bare repository"
		}
		m.showError(fmt.Errorf("the %s (%s) can't be %sd", kind, m.repo.Name(wt), verb))
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
		m.mode = modeList
		m.busy = true
		m.status = fmt.Sprintf("Creating %s from %s…", name, base)
		g := m.repo
		return m, func() tea.Msg {
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
		}
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
		wt := m.target
		name := m.repo.Name(wt)
		m.mode = modeList
		m.busy = true
		m.status = fmt.Sprintf("Deleting %s…", name)
		g := m.repo
		return m, func() tea.Msg {
			err := g.Delete(wt)
			return opDoneMsg{status: fmt.Sprintf("Deleted worktree %s.", name), err: err}
		}
	case "n", "N", "esc", "q":
		m.mode = modeList
	}
	return m, nil
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
		m.mode = modeList
		m.busy = true
		m.status = fmt.Sprintf("Renaming %s to %s…", oldName, newName)
		g := m.repo
		return m, func() tea.Msg {
			err := g.Rename(wt, newName)
			return opDoneMsg{
				status:     fmt.Sprintf("Renamed %s to %s.", oldName, newName),
				err:        err,
				selectPath: g.Path(newName),
			}
		}
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
