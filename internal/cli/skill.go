package cli

import (
	_ "embed" // for the go:embed lines below
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// skillContext is what "wtm skill" prints: a description of wtm for an AI
// agent. go:embed copies the file into the binary when it's built, so wtm
// doesn't need the file at run time.
//
//go:embed skill.md
var skillContext string

// skillFile is the SKILL.md that "wtm skill install" writes. It only points
// at "wtm skill", so an installed skill never goes out of date.
//
//go:embed skill_install.md
var skillFile string

// skillName is the installed skill's directory. Agents require it to match
// the name in skillFile's front matter.
const skillName = "wtm"

// skillAgent is an AI agent that loads skills from <dir>/<name>/SKILL.md.
type skillAgent struct {
	name  string
	title string
	// dir returns the agent's directory of personal skills, given the
	// user's home directory.
	dir func(home string) string
}

// skillAgents are the agents "wtm skill install" knows, in the order they're
// listed in its help.
var skillAgents = []skillAgent{
	{"claude", "Claude Code", func(home string) string {
		return filepath.Join(envOr("CLAUDE_CONFIG_DIR", filepath.Join(home, ".claude")), "skills")
	}},
	{"codex", "Codex", func(home string) string {
		return filepath.Join(home, ".agents", "skills")
	}},
	{"copilot", "GitHub Copilot CLI", func(home string) string {
		return filepath.Join(home, ".copilot", "skills")
	}},
	{"cursor", "Cursor", func(home string) string {
		return filepath.Join(home, ".cursor", "skills")
	}},
	{"gemini", "Gemini CLI", func(home string) string {
		return filepath.Join(home, ".gemini", "skills")
	}},
	{"opencode", "opencode", func(home string) string {
		return filepath.Join(envOr("XDG_CONFIG_HOME", filepath.Join(home, ".config")), "opencode", "skills")
	}},
}

// envOr is the environment variable key, or fallback when it isn't set.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func newSkillCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Describe wtm to an AI agent",
		Long: `Print a description of wtm for an AI agent: its commands, its configuration
and how .wtm/post_create.sh and .wtm/post_delete.sh are run. Add it to an
agent's context before asking for help with wtm, such as writing a
post_create.sh for your project.

"wtm skill install" sets an agent up to run this by itself.`,
		Example: `  wtm skill | pbcopy
  claude "$(wtm skill) Write a post_create.sh for this project."
  wtm skill install claude`,
		Args:              exactArgs(),
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := io.WriteString(cmd.OutOrStdout(), skillContext)
			return err
		},
	}
	cmd.AddCommand(newSkillInstallCmd())
	return cmd
}

func newSkillInstallCmd() *cobra.Command {
	var dir string
	var names, help []string
	for _, a := range skillAgents {
		names = append(names, a.name)
		help = append(help, fmt.Sprintf("  %-9s %-19s %s", a.name, a.title, tildePath(a.dir("~"))))
	}
	cmd := &cobra.Command{
		Use:   "install <agent>",
		Short: "Install a wtm skill for an AI agent",
		Long: `Install a skill called wtm in an agent's personal skills, so it's available in
every project. The skill is one small file, wtm/SKILL.md, that tells the agent
to run "wtm skill" when you ask it about wtm. Installing again replaces it.

The agents, and the skills directory each one reads:

` + strings.Join(help, "\n") + `

For any other agent that reads skills as <name>/SKILL.md, give its skills
directory with --dir. That also installs into one project, e.g.
--dir .claude/skills.`,
		Example: `  wtm skill install claude
  wtm skill install --dir ~/.config/agent/skills`,
		Args: func(cmd *cobra.Command, args []string) error {
			if dir != "" {
				if len(args) > 0 {
					return usageError(cmd, errors.New("give an agent or --dir, not both"))
				}
				return nil
			}
			return exactArgs("<agent>")(cmd, args)
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			var agents []cobra.Completion
			for _, a := range skillAgents {
				agents = append(agents, cobra.CompletionWithDesc(a.name, a.title))
			}
			return agents, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			skills, title := dir, ""
			if skills == "" {
				agent, ok := findSkillAgent(args[0])
				if !ok {
					return usageError(cmd, fmt.Errorf("unknown agent %q (wtm knows %s; use --dir for others)", args[0], strings.Join(names, ", ")))
				}
				home, err := os.UserHomeDir()
				if err != nil {
					return err
				}
				skills, title = agent.dir(home), " for "+agent.title
			}
			path := filepath.Join(skills, skillName, "SKILL.md")
			verb, err := writeSkill(path)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s the wtm skill%s at %s.\n", verb, title, tildePath(path))
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "install into this skills `directory` instead of an agent's")
	return cmd
}

func findSkillAgent(name string) (skillAgent, bool) {
	for _, a := range skillAgents {
		if a.name == strings.ToLower(name) {
			return a, true
		}
	}
	return skillAgent{}, false
}

// writeSkill writes the skill to path, creating its directories, and reports
// what it did: "Installed", or "Updated" when it replaced a file.
func writeSkill(path string) (verb string, err error) {
	verb = "Installed"
	if _, err := os.Stat(path); err == nil {
		verb = "Updated"
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(skillFile), 0o644); err != nil {
		return "", err
	}
	return verb, nil
}

// tildePath shows paths inside the home directory as ~/path.
func tildePath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if rel, err := filepath.Rel(home, path); err == nil && filepath.IsLocal(rel) {
		return filepath.Join("~", rel)
	}
	return path
}
