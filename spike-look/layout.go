package main

// Step 3: a Bubble Tea + Lip Gloss screen in the Pi-style layout (§5.2): header, animation band
// with Boot, the log of finished checks, the progress bar, the prompt line.

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tick time.Time

type layoutModel struct {
	start   time.Time
	w, h    int
	frames  int
	model   string
	pressed string
}

var (
	stHead  = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true)
	stQuiet = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	stText  = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	stPass  = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	stPart  = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	stFail  = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	stBoot  = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	stAcc   = lipgloss.NewStyle().Foreground(lipgloss.Color("13"))
	stBar   = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
)

var fakeLog = []struct{ mark, name, note string }{
	{"✓", "Identify", "iMac10,1 · board Mac-F2268CC8"},
	{"✓", "CPU", "Core 2 Duo E7600 · 2 cores · microcode ok"},
	{"✓", "Memory", "4 GiB · 1067 MHz DDR3"},
	{"~", "GPU", "Radeon HD 4670 · r600 · GLES 3.0"},
	{"✓", "Display", "eDP 1920×1080 · brightness 0–100"},
	{"×", "Wi-Fi", "BCM4331 · no driver in the live system"},
	{"✓", "Ethernet", "BCM57765 · link 1 Gb/s"},
	{"▲", "Sleep", "installed only"},
}

func (m layoutModel) Init() tea.Cmd { return doTick() }

func doTick() tea.Cmd {
	return tea.Tick(time.Second/15, func(t time.Time) tea.Msg { return tick(t) })
}

func (m layoutModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		m.pressed = msg.String()
		if m.pressed == "enter" || m.pressed == "q" || m.pressed == "ctrl+c" {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
	case tick:
		m.frames++
		return m, doTick()
	}
	return m, nil
}

func (m layoutModel) View() string {
	if m.w == 0 {
		return ""
	}
	t := time.Since(m.start).Seconds()
	var b strings.Builder
	head := stHead.Render(" iMac (21.5-inch, Late 2009) [iMac10,1]") + stQuiet.Render("  ·  config 1  ·  Omarchy 4.0.4  ·  ") + stPass.Render("ethernet")
	b.WriteString(head + "\n\n")
	// animation band: a small field beside Boot (drawn as plain lines here: Bubble Tea renders lines)
	sprite := bootRows(t, "neutral")
	band := 12 // the prototype's band height (tester preferred it to 7)
	fw := max(m.w-26, 10)
	for y := 0; y < band; y++ {
		var line strings.Builder
		line.WriteString(spriteLine(sprite, y))
		line.WriteString("  ")
		for x := 0; x < fw; x++ {
			fx, fy := float64(x), float64(y)*2
			v := sin(fx*0.13-t*2.0) + sin(fy*0.4+t) + sin((fx-fy)*0.05+t*0.7)
			i := min(max(int((v+3)/6*float64(len(ramp))), 0), len(ramp)-1)
			line.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(fmt.Sprint(rampCol[i]))).Render(string(ramp[i])))
		}
		b.WriteString(line.String() + "\n")
	}
	b.WriteString(stBoot.Render("  Boot: ") + stText.Render("Scanning the PCI bus. 14 devices, all accounted for.") + "\n\n")
	shown := min(int(t/0.8)+1, len(fakeLog))
	for _, l := range fakeLog[:shown] {
		st := map[string]lipgloss.Style{"✓": stPass, "~": stPart, "×": stFail, "▲": stPart}[l.mark]
		b.WriteString("  " + st.Render(l.mark) + " " + stText.Render(fmt.Sprintf("%-10s", l.name)) + stQuiet.Render(l.note) + "\n")
	}
	for i := shown; i < len(fakeLog); i++ {
		b.WriteString("\n")
	}
	b.WriteString("\n")
	p := min(t/10, 1)
	bw := max(m.w-24, 10)
	fill := int(p * float64(bw))
	b.WriteString("  " + stBar.Render(strings.Repeat("█", fill)) + stQuiet.Render(strings.Repeat("░", bw-fill)) +
		stText.Render(fmt.Sprintf(" %3.0f%%  ~%ds", p*100, int((1-p)*150))) + "\n\n")
	b.WriteString(stAcc.Render("  Enter") + stQuiet.Render(" continue · this screen is drawn by Bubble Tea at 15 fps"))
	return b.String()
}

func spriteLine(rows []string, y int) string {
	// one text row = two pixel rows, as in pixels(), but as a styled string
	top, bot := "", ""
	if 2*y < len(rows) {
		top = rows[2*y]
	}
	if 2*y+1 < len(rows) {
		bot = rows[2*y+1]
	}
	var s strings.Builder
	s.WriteString("  ")
	for i := 0; i < 16; i++ {
		a, b := -1, -1
		if i < len(top) {
			if v, ok := pix[top[i]]; ok {
				a = v
			}
		}
		if i < len(bot) {
			if v, ok := pix[bot[i]]; ok {
				b = v
			}
		}
		switch {
		case a < 0 && b < 0:
			s.WriteString(" ")
		case a == b:
			s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(fmt.Sprint(a))).Render("█"))
		default:
			if a < 0 {
				a = K
			}
			if b < 0 {
				b = K
			}
			if a > 7 && b > 7 {
				b -= 8
			}
			if a > 7 || b <= 7 {
				s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(fmt.Sprint(a))).Background(lipgloss.Color(fmt.Sprint(b))).Render("▀"))
			} else {
				s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(fmt.Sprint(b))).Background(lipgloss.Color(fmt.Sprint(a))).Render("▄"))
			}
		}
	}
	return s.String()
}

type LayoutResult struct {
	Seconds float64 `json:"seconds"`
	Frames  int     `json:"frames"`
	FPS     float64 `json:"fps"`
	Looks   string  `json:"looks"` // tester: ok, glitches
	Error   string  `json:"error,omitempty"`
}

func (s *Spike) layout() bool {
	if s.con.selftest {
		return true
	}
	s.con.setFont(s.ourFont())
	s.con.out(clear)
	s.con.cooked()
	start := time.Now()
	p := tea.NewProgram(layoutModel{start: start, model: s.res.Mac.Model}, tea.WithFPS(15))
	m, err := p.Run()
	s.con.raw()
	r := LayoutResult{Seconds: float64(int(time.Since(start).Seconds()*10)) / 10}
	if err != nil {
		r.Error = err.Error()
	}
	if lm, ok := m.(layoutModel); ok {
		r.Frames = lm.frames
		if r.Seconds > 0 {
			r.FPS = float64(int(float64(lm.frames)/r.Seconds*10)) / 10
		}
		if lm.pressed == "q" || lm.pressed == "ctrl+c" {
			s.res.Layout = r
			return false
		}
	}
	s.con.out("\x1b[?25l")
	_, rows := s.con.size()
	s.con.out(at(2, rows-4) + "\x1b[2K" + col(WHT, -1) + "Did that screen draw cleanly (no flicker, no torn lines, no stray characters)?" + reset)
	s.hint("y yes · n no · q quit")
	k := s.con.waitFor("y", "n")
	r.Looks = map[string]string{"y": "ok", "n": "glitches"}[k]
	s.res.Layout = r
	return k != "q"
}
