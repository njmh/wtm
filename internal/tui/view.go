package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/njmh/wtm/internal/git"
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
	if m.status != "" {
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
		branchW = max(branchW, ansi.StringWidth(wt.BranchLabel()))
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
		row := cell(names[i], nameW) + "  " + cell(wt.BranchLabel(), branchW) + "  " + cell(wt.ShortHead(), 7)
		tags := wt.Tags()

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

func tagStyle(tag string) lipgloss.Style {
	switch tag {
	case "locked":
		return lipgloss.NewStyle().Foreground(colorWarn)
	case "prunable":
		return lipgloss.NewStyle().Foreground(colorDanger)
	}
	return lipgloss.NewStyle().Foreground(colorInfo)
}
