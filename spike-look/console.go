package main

// Linux VT helpers: raw keys, fonts via setfont, the 16-colour palette, cursor moves.

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

type Console struct {
	fd       int
	saved    *term.State
	origFont string // setfont -O copy of the font we started with
	selftest bool
	cols     int
	rows     int
}

func openConsole(selftest bool, tmp string) (*Console, error) {
	c := &Console{fd: int(os.Stdin.Fd()), selftest: selftest, cols: 80, rows: 25}
	if selftest {
		return c, nil
	}
	if !term.IsTerminal(c.fd) {
		return nil, fmt.Errorf("stdin is not a terminal")
	}
	c.origFont = tmp + "/orig-font.psf"
	if out, err := exec.Command("setfont", "-O", c.origFont).CombinedOutput(); err != nil {
		c.origFont = ""
		fmt.Fprintf(os.Stderr, "setfont -O: %v %s\n", err, out)
	}
	st, err := term.MakeRaw(c.fd)
	if err != nil {
		return nil, err
	}
	c.saved = st
	c.size()
	c.out("\x1b[?25l\x1b[0m\x1b[2J\x1b[H")
	return c, nil
}

func (c *Console) restore() {
	if c.selftest {
		return
	}
	c.out("\x1b]R\x1b[0m\x1b[2J\x1b[H\x1b[?25h")
	if c.origFont != "" {
		exec.Command("setfont", c.origFont).Run()
	}
	if c.saved != nil {
		term.Restore(c.fd, c.saved)
	}
}

func (c *Console) cooked() { term.Restore(c.fd, c.saved) }
func (c *Console) raw()    { c.saved, _ = term.MakeRaw(c.fd) }

func (c *Console) size() (int, int) {
	if c.selftest {
		return c.cols, c.rows
	}
	if ws, err := unix.IoctlGetWinsize(c.fd, unix.TIOCGWINSZ); err == nil {
		c.cols, c.rows = int(ws.Col), int(ws.Row)
	}
	return c.cols, c.rows
}

// out writes straight to the tty (unbuffered: the write returns once the console has drawn it).
func (c *Console) out(s string) {
	if c.selftest {
		return
	}
	os.Stdout.WriteString(s)
}

func (c *Console) outTimed(b []byte) time.Duration {
	t := time.Now()
	if !c.selftest {
		os.Stdout.Write(b)
	}
	return time.Since(t)
}

// setFont loads a console font file; returns the new grid size.
func (c *Console) setFont(path string) (int, int, error) {
	if c.selftest {
		return c.cols, c.rows, nil
	}
	out, err := exec.Command("setfont", path).CombinedOutput()
	if err != nil {
		return 0, 0, fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	cols, rows := c.size()
	return cols, rows, nil
}

func (c *Console) setPalette(p [16]string) {
	var b strings.Builder
	for i, h := range p {
		fmt.Fprintf(&b, "\x1b]P%X%s", i, strings.TrimPrefix(h, "#"))
	}
	c.out(b.String())
}

// key waits for one key and names it.
func (c *Console) key() string {
	if c.selftest {
		return "enter"
	}
	buf := make([]byte, 16)
	n, err := os.Stdin.Read(buf)
	if err != nil || n == 0 {
		return "q"
	}
	b := buf[:n]
	switch {
	case b[0] == 3 || b[0] == 'q' || b[0] == 'Q':
		return "q"
	case b[0] == '\r' || b[0] == '\n':
		return "enter"
	case b[0] == 127 || b[0] == 8:
		return "bs"
	case b[0] == '\t':
		return "tab"
	case bytes.Equal(b, []byte("\x1b[A")):
		return "up"
	case bytes.Equal(b, []byte("\x1b[B")):
		return "down"
	case bytes.Equal(b, []byte("\x1b[C")):
		return "right"
	case bytes.Equal(b, []byte("\x1b[D")):
		return "left"
	case b[0] == 0x1b:
		return "esc"
	}
	return strings.ToLower(string(b[:1]))
}

// waitFor returns the first key that is one of keys (q always counts).
func (c *Console) waitFor(keys ...string) string {
	if c.selftest {
		return keys[0] // scripted: the first allowed answer
	}
	for {
		k := c.key()
		if k == "q" {
			return k
		}
		for _, w := range keys {
			if k == w {
				return k
			}
		}
	}
}

// Screen text helpers --------------------------------------------------------

func at(x, y int) string { return fmt.Sprintf("\x1b[%d;%dH", y+1, x+1) }

// col sets foreground (0–15) and background (0–7, or -1 to leave it).
func col(fg, bg int) string {
	s := "\x1b[0"
	if fg >= 8 {
		s += fmt.Sprintf(";%d", 90+fg-8)
	} else if fg >= 0 {
		s += fmt.Sprintf(";%d", 30+fg)
	}
	if bg >= 0 {
		s += fmt.Sprintf(";%d", 40+bg%8)
	}
	return s + "m"
}

const reset = "\x1b[0m"
const clear = "\x1b[0m\x1b[2J\x1b[H"

// VT colour roles, as in the prototype.
const (
	K = iota
	RED
	GRN
	AMB
	BLU
	VIO
	CYN
	GRY
	DIM
	BRED
	BGRN
	YEL
	BBLU
	BVIO
	BCYN
	WHT
)

var palettes = []struct {
	name string
	c    [16]string
}{
	{"OmaBoot?", [16]string{"#0b0d10", "#c2453f", "#3fa173", "#c98f3a", "#5276b8", "#8a78e6", "#4fa6ad", "#b4bcc2",
		"#4a535b", "#f06a63", "#5fd39b", "#ffd479", "#89aef0", "#b8a9ff", "#8fe3e8", "#eef2f4"}},
	{"Tokyo Night", [16]string{"#1a1b26", "#f7768e", "#9ece6a", "#e0af68", "#7aa2f7", "#bb9af7", "#7dcfff", "#a9b1d6",
		"#414868", "#f7768e", "#9ece6a", "#e0af68", "#7aa2f7", "#bb9af7", "#7dcfff", "#c0caf5"}},
	{"Hackerman", [16]string{"#0b0c16", "#ff5f6d", "#4fe88f", "#ffb86b", "#829dd4", "#7cf8f7", "#86a7df", "#b5c5db",
		"#6a6e95", "#ff8a94", "#82fb9c", "#f9f871", "#c4d2ed", "#d1fffe", "#c4d2ed", "#ddf7ff"}},
}
