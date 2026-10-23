package tui

import (
	"fmt"
	"io"
	"os"

	"github.com/njmh/wtm/internal/config"
	"github.com/njmh/wtm/internal/hook"
)

// pauseOnFailure runs the post-create hook with the terminal handed over by
// tea.Exec. When the script fails its output is still on screen, so it waits
// for Enter before handing the terminal back to wtm, which would hide it.
type pauseOnFailure struct {
	*hook.PostCreate
	in  io.Reader
	out io.Writer
}

func (p *pauseOnFailure) SetStdin(r io.Reader) {
	p.in = r
	p.PostCreate.SetStdin(r)
}

func (p *pauseOnFailure) SetStdout(w io.Writer) {
	p.out = w
	p.PostCreate.SetStdout(w)
}

func (p *pauseOnFailure) Run() error {
	err := p.PostCreate.Run()
	if err != nil {
		out := p.out
		if out == nil {
			out = os.Stdout
		}
		fmt.Fprintf(out, "\nwtm: %s failed (%v). Press Enter to return to wtm.", config.PostCreateName, err)
		if p.in != nil {
			buf := make([]byte, 1)
			for {
				n, rerr := p.in.Read(buf)
				if rerr != nil || (n == 1 && (buf[0] == '\n' || buf[0] == '\r')) {
					break
				}
			}
		}
	}
	return err
}
