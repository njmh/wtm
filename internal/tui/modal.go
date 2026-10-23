package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/njmh/wtm/internal/config"
)

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
		style = style.BorderForeground(colorDanger)
		if len(m.deleting) > 1 {
			return m.deleteManyView(style, inner, hint("y", "delete all", "n/esc", "cancel"))
		}
		wt := m.deleting[0]
		branch := wt.Branch
		if branch == "" {
			branch = "none (detached HEAD)"
		}
		lines := []string{
			errorStyle.Bold(true).Render("Delete worktree?"),
			"",
			"This permanently removes:",
			"",
			"  " + mutedStyle.Render("directory  ") + labelStyle.Render(ansi.Truncate(m.shortPath(wt.Path), inner-13, "…")),
			"  " + mutedStyle.Render("branch     ") + labelStyle.Render(branch),
			"",
		}
		if m.warnings[0] != "" {
			lines = append(lines, warnStyle.Width(inner).Render("Warning: "+m.warnings[0]), "")
		}
		body := strings.Join(append(lines,
			mutedStyle.Render("Fails safely if there are uncommitted changes."),
			"",
			hint("y", "delete", "n/esc", "cancel"),
		), "\n")
		return style.Render(body)

	case modeBusy:
		// The spinner frame is two cells wide; the activity wraps beside it.
		activity := labelStyle.Width(inner - 2).Render(m.activity)
		return style.Render(lipgloss.JoinHorizontal(lipgloss.Top, m.spinner.View(), activity))

	case modeError:
		msg := "Unknown error"
		if m.err != nil {
			msg = m.err.Error()
			// Sentence-case plain errors; git output is shown verbatim.
			if msg != "" && !strings.HasPrefix(msg, "$") {
				msg = strings.ToUpper(msg[:1]) + msg[1:]
			}
		}
		text := lipgloss.NewStyle().Width(inner).Render(msg)
		// Cut off what doesn't fit on screen: 8 lines are border, padding,
		// title, hint and the blank lines between them.
		if room := m.height - 8; m.height > 0 && lipgloss.Height(text) > room {
			lines := strings.Split(text, "\n")[:max(room-1, 1)]
			text = strings.Join(append(lines, "…"), "\n")
		}
		body := strings.Join([]string{
			errorStyle.Bold(true).Render("Error"),
			"",
			text,
			"",
			hint("enter/esc", "dismiss"),
		}, "\n")
		return style.BorderForeground(colorDanger).Render(body)
	}
	return ""
}

// deleteManyView is the delete popover for several worktrees: a list of them,
// shortened to fit the screen, with one warning covering any that have
// unmerged commits.
func (m model) deleteManyView(style lipgloss.Style, inner int, hint string) string {
	head := []string{
		errorStyle.Bold(true).Render(fmt.Sprintf("Delete %d worktrees?", len(m.deleting))),
		"",
		"This permanently removes their directories and branches:",
		"",
	}
	tail := []string{""}
	anyUnmerged := false
	for _, w := range m.warnings {
		anyUnmerged = anyUnmerged || w != ""
	}
	if anyUnmerged {
		tail = append(tail, warnStyle.Width(inner).Render(
			"Warning: branches marked ! have commits that aren't in their upstream (or HEAD). "+
				"That's expected if they were squash-merged; otherwise those commits will be lost."), "")
	}
	tail = append(tail,
		mutedStyle.Render("Any with uncommitted changes are left alone."),
		"",
		hint,
	)

	nameW := 0
	for _, wt := range m.deleting {
		nameW = max(nameW, ansi.StringWidth(m.repo.Name(wt)))
	}
	nameW = min(nameW, inner/2)
	rows := make([]string, len(m.deleting))
	for i, wt := range m.deleting {
		mark := "  "
		if m.warnings[i] != "" {
			mark = warnStyle.Render("! ")
		}
		row := mark + labelStyle.Render(pad(m.repo.Name(wt), nameW)) + "  " + mutedStyle.Render(wt.BranchLabel())
		rows[i] = ansi.Truncate(row, inner, "…")
	}

	// Show as many rows as fit, replacing the rest with a count.
	if m.height > 0 {
		room := max(m.height-lipgloss.Height(style.Render(strings.Join(append(head, tail...), "\n"))), 1)
		if len(rows) > room {
			more := len(rows) - room + 1
			rows = append(rows[:room-1], mutedStyle.Render(fmt.Sprintf("  …and %d more", more)))
		}
	}

	return style.Render(strings.Join(append(append(head, rows...), tail...), "\n"))
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
