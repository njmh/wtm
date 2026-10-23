package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/njmh/wtm/internal/config"
	"github.com/njmh/wtm/internal/git"
)

var (
	colorAccent = lipgloss.Color("99")
	colorMuted  = lipgloss.Color("244")
	colorDanger = lipgloss.Color("203")
	colorWarn   = lipgloss.Color("214")
	colorOK     = lipgloss.Color("78")
	colorInfo   = lipgloss.Color("75")

	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	mutedStyle    = lipgloss.NewStyle().Foreground(colorMuted)
	headerStyle   = lipgloss.NewStyle().Foreground(colorMuted).Bold(true)
	selectedStyle = lipgloss.NewStyle().Bold(true).
			Foreground(lipgloss.Color("231")).Background(lipgloss.Color("238"))
	keyStyle   = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	okStyle    = lipgloss.NewStyle().Foreground(colorOK)
	busyStyle  = lipgloss.NewStyle().Foreground(colorWarn)
	errorStyle = lipgloss.NewStyle().Foreground(colorDanger)
	dimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	modalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorAccent).
			Padding(1, 2)
	labelStyle = lipgloss.NewStyle().Bold(true)
)

var menu = []struct{ key, label string }{
	{"C", "Create new worktree"},
	{"D", "Delete selected worktree"},
	{"R", "Rename selected worktree"},
	{"P", "Prune worktrees"},
	{"Q", "Quit"},
}

// Lines of the main screen that aren't list rows: title, blank, column
// header, blank, menu, blank, status.
const chromeHeight = 1 + 1 + 1 + 1 + 5 + 1 + 1

// listHeight is how many worktree rows fit on screen.
func (m model) listHeight() int {
	if m.height == 0 {
		return max(len(m.worktrees), 1)
	}
	return max(m.height-chromeHeight, 1)
}

func (m model) View() tea.View {
	content := ""
	if m.width > 0 {
		content = m.mainView()
		if modal := m.modalView(); modal != "" {
			content = m.overlay(dim(content), modal)
		}
	}
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (m model) mainView() string {
	var b strings.Builder

	title := " " + titleStyle.Render("Worktree Manager") + "  "
	note := ""
	if root := m.repo.WorktreeRoot(); root != m.repo.Dir {
		note = mutedStyle.Render("  →  worktrees in ") + m.shortPath(root)
	}
	// Shorten the directory from the left so the worktree path stays visible.
	dir := m.shortPath(m.repo.Dir)
	if room := max(m.width-ansi.StringWidth(title)-ansi.StringWidth(note), 10); ansi.StringWidth(dir) > room {
		dir = ansi.TruncateLeft(dir, ansi.StringWidth(dir)-room+1, "…")
	}
	b.WriteString(title + mutedStyle.Render(dir) + note + "\n\n")

	b.WriteString(m.listView())

	b.WriteString("\n")
	for _, item := range menu {
		b.WriteString("  " + keyStyle.Render(item.key) + "  " + item.label + "\n")
	}

	b.WriteString("\n ")
	switch {
	case m.busy:
		b.WriteString(busyStyle.Render(m.status))
	case m.status != "":
		b.WriteString(okStyle.Render(m.status))
	}

	lines := strings.Split(b.String(), "\n")
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, m.width, "…")
	}
	return strings.Join(lines, "\n")
}

func (m model) listView() string {
	var b strings.Builder

	names := make([]string, len(m.worktrees))
	nameW, branchW := len("WORKTREE"), len("BRANCH")
	for i, wt := range m.worktrees {
		names[i] = m.repo.Name(wt)
		nameW = max(nameW, ansi.StringWidth(names[i]))
		branchW = max(branchW, ansi.StringWidth(branchLabel(wt)))
	}
	nameW, branchW = min(nameW, 40), min(branchW, 40)

	cell := func(s string, w int) string {
		s = ansi.Truncate(s, w, "…")
		return s + strings.Repeat(" ", w-ansi.StringWidth(s))
	}

	header := "  " + cell("WORKTREE", nameW) + "  " + cell("BRANCH", branchW) + "  HEAD"
	if len(m.worktrees) > m.listHeight() {
		header += fmt.Sprintf("   (%d/%d)", m.cursor+1, len(m.worktrees))
	}
	b.WriteString(headerStyle.Render(header) + "\n")

	if len(m.worktrees) == 0 {
		b.WriteString(mutedStyle.Render("  No worktrees found.") + "\n")
		return b.String()
	}

	end := min(m.offset+m.listHeight(), len(m.worktrees))
	for i := m.offset; i < end; i++ {
		wt := m.worktrees[i]
		head := wt.Head
		if len(head) > 7 {
			head = head[:7]
		}
		row := cell(names[i], nameW) + "  " + cell(branchLabel(wt), branchW) + "  " + cell(head, 7)
		tags := tagsOf(wt)

		if i == m.cursor {
			line := "▸ " + row
			if len(tags) > 0 {
				line += "  " + strings.Join(tags, " ")
			}
			b.WriteString(selectedStyle.Width(max(m.width, 1)).Render(ansi.Truncate(line, m.width, "…")) + "\n")
			continue
		}
		line := "  " + row
		if len(tags) > 0 {
			styled := make([]string, len(tags))
			for j, t := range tags {
				styled[j] = tagStyle(t).Render(t)
			}
			line += "  " + strings.Join(styled, " ")
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

// shortPath shows paths inside the working directory as ./name and
// abbreviates the home directory to ~.
func (m model) shortPath(path string) string {
	if path != m.repo.Dir {
		if rel, ok := git.RelInside(m.repo.Dir, path); ok {
			return "./" + rel
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		if rel, err := filepath.Rel(home, path); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.Join("~", rel)
		}
	}
	return path
}

func branchLabel(wt git.Worktree) string {
	switch {
	case wt.Branch != "":
		return wt.Branch
	case wt.Bare:
		return "—"
	case wt.Detached:
		return "(detached)"
	}
	return ""
}

func tagsOf(wt git.Worktree) []string {
	var tags []string
	if wt.Bare {
		tags = append(tags, "bare")
	} else if wt.Main {
		tags = append(tags, "main")
	}
	if wt.Locked {
		tags = append(tags, "locked")
	}
	if wt.Prunable {
		tags = append(tags, "prunable")
	}
	return tags
}

func tagStyle(tag string) lipgloss.Style {
	switch tag {
	case "locked":
		return lipgloss.NewStyle().Foreground(colorWarn)
	case "prunable":
		return lipgloss.NewStyle().Foreground(colorDanger)
	}
	return lipgloss.NewStyle().Foreground(colorInfo)
}

// createNote says where new worktrees go, when that isn't the current
// directory, and whether post_create.sh will run.
func (m model) createNote() string {
	var notes []string
	if root := m.repo.WorktreeRoot(); root != m.repo.Dir {
		notes = append(notes, "Created in "+m.shortPath(root)+"/")
	}
	if config.PostCreateScript(m.repo.Dir) != "" {
		notes = append(notes, "runs "+config.PostCreateName)
	}
	if len(notes) == 0 {
		return ""
	}
	s := strings.Join(notes, ", then ") + "."
	return strings.ToUpper(s[:1]) + s[1:]
}

func (m model) modalWidth() int {
	return max(min(64, m.width-4), 20)
}

// modalView renders the active popover, or "" when there is none.
func (m model) modalView() string {
	inner := m.modalWidth() - 6 // border + horizontal padding
	style := modalStyle.Width(m.modalWidth())
	hint := func(pairs ...string) string {
		var parts []string
		for i := 0; i+1 < len(pairs); i += 2 {
			parts = append(parts, keyStyle.Render(pairs[i])+" "+mutedStyle.Render(pairs[i+1]))
		}
		return strings.Join(parts, mutedStyle.Render(" · "))
	}
	formErr := func() string {
		if m.formErr == "" {
			return ""
		}
		return "\n" + errorStyle.Render(m.formErr) + "\n"
	}

	switch m.mode {
	case modeCreate:
		name, base := m.nameInput, m.baseInput
		name.SetWidth(inner - 3)
		base.SetWidth(inner - 3)
		lines := []string{titleStyle.Render("Create worktree"), ""}
		if note := m.createNote(); note != "" {
			lines = append(lines, lipgloss.NewStyle().Width(inner).Render(mutedStyle.Render(note)), "")
		}
		body := strings.Join(append(lines,
			labelStyle.Render("Name")+mutedStyle.Render("  (directory and new branch)"),
			name.View(),
			"",
			labelStyle.Render("Base branch"),
			base.View(),
			formErr(),
			hint("enter", "create", "tab", "switch field", "esc", "cancel"),
		), "\n")
		return style.Render(body)

	case modeRename:
		in := m.renameInput
		in.SetWidth(inner - 3)
		current := mutedStyle.Render("directory ") + labelStyle.Render(m.repo.Name(m.target))
		if m.target.Branch != "" {
			current += mutedStyle.Render(" and branch ") + labelStyle.Render(m.target.Branch)
		}
		body := strings.Join([]string{
			titleStyle.Render("Rename worktree"),
			"",
			mutedStyle.Render("Renames ") + current,
			"",
			labelStyle.Render("New name"),
			in.View(),
			formErr(),
			hint("enter", "rename", "esc", "cancel"),
		}, "\n")
		return style.Render(body)

	case modeDelete:
		branch := m.target.Branch
		if branch == "" {
			branch = "none (detached HEAD)"
		}
		body := strings.Join([]string{
			errorStyle.Bold(true).Render("Delete worktree?"),
			"",
			"This permanently removes:",
			"",
			"  " + mutedStyle.Render("directory  ") + labelStyle.Render(ansi.Truncate(m.shortPath(m.target.Path), inner-13, "…")),
			"  " + mutedStyle.Render("branch     ") + labelStyle.Render(branch),
			"",
			mutedStyle.Render("Fails safely if there are uncommitted changes or unmerged commits."),
			"",
			hint("y", "delete", "n/esc", "cancel"),
		}, "\n")
		return style.BorderForeground(colorDanger).Render(body)

	case modeError:
		msg := "Unknown error"
		if m.err != nil {
			msg = m.err.Error()
			// Sentence-case plain errors; git output is shown verbatim.
			if msg != "" && !strings.HasPrefix(msg, "$") {
				msg = strings.ToUpper(msg[:1]) + msg[1:]
			}
		}
		body := strings.Join([]string{
			errorStyle.Bold(true).Render("Error"),
			"",
			lipgloss.NewStyle().Width(inner).Render(msg),
			"",
			hint("enter/esc", "dismiss"),
		}, "\n")
		return style.BorderForeground(colorDanger).Render(body)
	}
	return ""
}

// overlay draws modal centered on top of base, filling the whole screen.
func (m model) overlay(base, modal string) string {
	x := max((m.width-lipgloss.Width(modal))/2, 0)
	y := max((m.height-lipgloss.Height(modal))/2, 0)
	return lipgloss.NewCanvas(m.width, m.height).Compose(lipgloss.NewCompositor(
		lipgloss.NewLayer(base),
		lipgloss.NewLayer(modal).X(x).Y(y).Z(1),
	)).Render()
}

// dim fades the background behind a popover.
func dim(s string) string {
	lines := strings.Split(ansi.Strip(s), "\n")
	for i, line := range lines {
		lines[i] = dimStyle.Render(line)
	}
	return strings.Join(lines, "\n")
}
