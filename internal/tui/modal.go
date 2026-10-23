package tui

import (
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
		branch := m.target.Branch
		if branch == "" {
			branch = "none (detached HEAD)"
		}
		lines := []string{
			errorStyle.Bold(true).Render("Delete worktree?"),
			"",
			"This permanently removes:",
			"",
			"  " + mutedStyle.Render("directory  ") + labelStyle.Render(ansi.Truncate(m.shortPath(m.target.Path), inner-13, "…")),
			"  " + mutedStyle.Render("branch     ") + labelStyle.Render(branch),
			"",
		}
		if m.warning != "" {
			lines = append(lines, warnStyle.Width(inner).Render("Warning: "+m.warning), "")
		}
		body := strings.Join(append(lines,
			mutedStyle.Render("Fails safely if there are uncommitted changes."),
			"",
			hint("y", "delete", "n/esc", "cancel"),
		), "\n")
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
