package main

// The spike's steps: fonts, palette, Pi-style layout (Bubble Tea), redraw benchmark, GPU.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// header draws the step title line and returns the next free row.
func (s *Spike) header(step, title string) {
	cols, _ := s.con.size()
	mac := s.res.Mac.Model
	line := fmt.Sprintf(" OmaBoot? Live spike  ·  %s  ·  %s", step, title)
	s.con.out(clear + col(WHT, VIO%8) + pad(line, cols) + reset)
	s.con.out(at(cols-len(mac)-2, 0) + col(WHT, VIO%8) + mac + reset)
}

func pad(s string, w int) string {
	n := len([]rune(s))
	if n >= w {
		return string([]rune(s)[:w])
	}
	return s + strings.Repeat(" ", w-n)
}

func (s *Spike) hint(text string) {
	cols, rows := s.con.size()
	s.con.out(at(0, rows-1) + col(DIM, -1) + pad(" "+text, cols-1) + reset)
}

// Step 1: fonts -------------------------------------------------------------

type FontRating struct {
	Font    string `json:"font"`
	Ours    bool   `json:"ours"`
	Cols    int    `json:"cols"`
	Rows    int    `json:"rows"`
	Size    string `json:"verdict"`           // too_small, good, too_big
	Symbols string `json:"symbols,omitempty"` // ok, wrong (our fonts only)
	Error   string `json:"error,omitempty"`
}

type fontChoice struct {
	label, path string
	ours        bool
}

func (s *Spike) fontList() []fontChoice {
	sys := "/usr/share/kbd/consolefonts/"
	var l []fontChoice
	for _, n := range []string{"omaboot-v16n", "omaboot-v24n", "omaboot-v32n"} {
		l = append(l, fontChoice{n + " (ours)", filepath.Join(s.stick, "fonts", n+".psf.gz"), true})
	}
	for _, n := range []string{"ter-v16n", "ter-v24n", "ter-v32n", "ter-v32b"} {
		l = append(l, fontChoice{n, sys + n + ".psf.gz", false})
	}
	for _, n := range []string{"spleen-8x16", "spleen-12x24", "spleen-16x32", "spleen-32x64"} {
		l = append(l, fontChoice{n, filepath.Join(s.stick, "fonts", n+".psfu"), false})
	}
	return l
}

func (s *Spike) fonts() bool {
	list := s.fontList()
	s.res.Fonts = make([]FontRating, len(list))
	for i := 0; i < len(list); {
		f := list[i]
		r := FontRating{Font: f.label, Ours: f.ours}
		cols, rows, err := s.con.setFont(f.path)
		if err != nil {
			r.Error = err.Error()
			s.res.Fonts[i] = r
			i++
			continue
		}
		r.Cols, r.Rows = cols, rows
		s.header(fmt.Sprintf("1/6 fonts %d of %d", i+1, len(list)), f.label)
		s.fontSample(f, cols, rows)
		s.con.out(at(2, rows-4) + col(WHT, -1) + "How big is the text from where you sit?" + reset)
		s.hint("1 too small · 2 good · 3 too big · delete: back · q quit")
		k := s.con.waitFor("1", "2", "3", "bs")
		switch k {
		case "q":
			return false
		case "bs":
			i = max(i-1, 0)
			continue
		}
		r.Size = map[string]string{"1": "too_small", "2": "good", "3": "too_big"}[k]
		if f.ours {
			s.con.out(at(2, rows-4) + "\x1b[2K" + col(WHT, -1) + "Do the tick, the triangle and every block and line look right? " + reset)
			s.hint("y yes · n no, something looks wrong · q quit")
			k = s.con.waitFor("y", "n")
			if k == "q" {
				return false
			}
			r.Symbols = map[string]string{"y": "ok", "n": "wrong"}[k]
		}
		s.res.Fonts[i] = r
		i++
	}
	return true
}

func (s *Spike) fontSample(f fontChoice, cols, rows int) {
	var b strings.Builder
	y := 2
	line := func(c int, t string) { b.WriteString(at(2, y) + col(c, -1) + t + reset); y++ }
	line(GRY, fmt.Sprintf("Grid %d × %d", cols, rows))
	y++
	line(WHT, "The quick brown fox jumps over the lazy dog. 0123456789")
	line(GRY, "Äpfel, Größe, Übermut · café, garçon, niño · Жизнь, Ґанок · Ωmega")
	y++
	b.WriteString(at(2, y) + col(BGRN, -1) + "✓ pass" + reset + "   " + col(YEL, -1) + "~ partial" + reset + "   " +
		col(BRED, -1) + "× fail" + reset + "   " + col(YEL, -1) + "▲ installed only" + reset + "   " + col(DIM, -1) + "· skipped" + reset)
	y += 2
	line(GRY, "┌──┬──┐  ╔══╦══╗  ░░▒▒▓▓██  ▀▀▄▄▌▐")
	line(GRY, "├──┼──┤  ╠══╬══╣  ↑ ↓ ← → ↕ ↔ ○ √")
	line(GRY, "└──┴──┘  ╚══╩══╝  [■■■■■■····] 60%")
	y++
	if y+8 < rows-5 {
		b.WriteString(pixels(4, y, bootRows(1.0, "neutral")))
		b.WriteString(at(24, y+2) + col(BCYN, -1) + "Boot: " + reset + col(GRY, -1) + "GPU: Radeon HD 4670. r600 loaded." + reset)
	}
	s.con.out(b.String())
}

// Step 2: palette ------------------------------------------------------------

type PaletteAnswers struct {
	BrightBackgrounds string `json:"bright_backgrounds"` // do 100–107 differ from 40–47 (expected: no)
	BrightText        string `json:"bright_text"`        // do 90–97 look brighter than 30–37 with a 512-glyph font
	Liked             string `json:"liked"`              // the palette the tester stopped on
}

// ourFont is the tester's pick among our fonts (the first rated good), else omaboot-v24n.
func (s *Spike) ourFont() string {
	l := s.fontList()
	for i, r := range s.res.Fonts {
		if r.Ours && r.Size == "good" && i < len(l) {
			return l[i].path
		}
	}
	return l[1].path
}

func (s *Spike) palette() bool {
	s.con.setFont(s.ourFont()) // a 512-glyph font: does bright text survive the 9th glyph bit?
	pi := 0
	for {
		s.con.setPalette(palettes[pi].c)
		cols, rows := s.con.size()
		s.header("2/6 palette", palettes[pi].name+"   (Tab: next palette)")
		var b strings.Builder
		b.WriteString(at(2, 2) + col(GRY, -1) + "A  normal text 30–37:   " + reset)
		for i := 0; i < 8; i++ {
			b.WriteString(col(i, -1) + fmt.Sprintf(" %X text ", i))
		}
		b.WriteString(at(2, 3) + col(GRY, -1) + "B  bright text 90–97:   " + reset)
		for i := 8; i < 16; i++ {
			b.WriteString(col(i, -1) + fmt.Sprintf(" %X text ", i))
		}
		b.WriteString(reset + at(2, 5) + col(GRY, -1) + "C  backgrounds 40–47:   " + reset)
		for i := 0; i < 8; i++ {
			b.WriteString(fmt.Sprintf("\x1b[%dm   %d    ", 40+i, i))
		}
		b.WriteString(reset + at(2, 6) + col(GRY, -1) + "D  backgrounds 100–107: " + reset)
		for i := 0; i < 8; i++ {
			b.WriteString(fmt.Sprintf("\x1b[%dm   %d    ", 100+i, i+8))
		}
		b.WriteString(reset)
		b.WriteString(at(2, 8) + col(BGRN, -1) + "✓ pass  " + col(YEL, -1) + "~ partial  " + col(BRED, -1) + "× fail  " +
			col(YEL, -1) + "▲ installed only  " + col(DIM, -1) + "· skipped  " + col(BVIO, -1) + "accent" + reset)
		b.WriteString(pixels(4, 10, bootRows(2.0, "pleased")))
		b.WriteString(at(24, 12) + col(BCYN, -1) + "Boot: " + col(GRY, -1) + "half-blocks: one bright colour per cell." + reset)
		s.con.out(b.String())
		_ = cols
		s.con.out(at(2, rows-4) + col(WHT, -1) + "Look at rows B and D, then press Enter." + reset)
		s.hint("Tab next palette · Enter answer questions · q quit")
		k := s.con.waitFor("enter", "tab")
		if k == "q" {
			return false
		}
		if k == "tab" {
			pi = (pi + 1) % len(palettes)
			continue
		}
		s.res.Palette.Liked = palettes[pi].name
		break
	}
	_, rows := s.con.size()
	ask := func(q string) string {
		s.con.out(at(2, rows-4) + "\x1b[2K" + col(WHT, -1) + q + reset)
		s.hint("y yes · n no · q quit")
		return s.con.waitFor("y", "n")
	}
	k := ask("Row B: is that text brighter than row A? ")
	if k == "q" {
		return false
	}
	s.res.Palette.BrightText = k
	k = ask("Row D: are those boxes brighter than row C? ")
	if k == "q" {
		return false
	}
	s.res.Palette.BrightBackgrounds = k
	s.con.setPalette(palettes[0].c)
	return true
}

// Step 4: redraw benchmark ---------------------------------------------------

type BenchResult struct {
	Font        string      `json:"font"`
	Cols        int         `json:"cols"`
	Rows        int         `json:"rows"`
	FullRepaint Stats       `json:"full_repaint"` // every cell, new colour and character
	Band        []BandStats `json:"band"`         // animation band, 10 rows
	Error       string      `json:"error,omitempty"`
}

type BandStats struct {
	TargetFPS   int     `json:"target_fps"` // 0 = as fast as it goes
	AchievedFPS float64 `json:"achieved_fps"`
	Write       Stats   `json:"write_ms"`
	Bytes       int     `json:"bytes_per_frame"`
}

type Stats struct {
	N    int     `json:"n"`
	Mean float64 `json:"mean_ms"`
	P50  float64 `json:"p50_ms"`
	P95  float64 `json:"p95_ms"`
	Max  float64 `json:"max_ms"`
}

func stats(d []time.Duration) Stats {
	if len(d) == 0 {
		return Stats{}
	}
	ms := make([]float64, len(d))
	sum := 0.0
	for i, x := range d {
		ms[i] = float64(x.Microseconds()) / 1000
		sum += ms[i]
	}
	sort.Float64s(ms)
	r := func(v float64) float64 { return float64(int(v*100+0.5)) / 100 }
	return Stats{N: len(ms), Mean: r(sum / float64(len(ms))), P50: r(ms[len(ms)/2]), P95: r(ms[len(ms)*95/100]), Max: r(ms[len(ms)-1])}
}

func (s *Spike) bench() bool {
	fonts := []fontChoice{}
	for _, f := range s.fontList() {
		if f.ours {
			fonts = append(fonts, f)
		}
	}
	for _, f := range fonts {
		r := BenchResult{Font: f.label}
		cols, rows, err := s.con.setFont(f.path)
		if err != nil { // measure in whatever font is loaded, and say so
			r.Error = "setfont: " + err.Error()
			cols, rows = s.con.size()
		}
		r.Cols, r.Rows = cols, rows
		s.header("4/6 redraw speed", f.label+fmt.Sprintf(" · %d × %d · hands off, ~20 s", cols, rows))
		// full-screen repaints
		var full []time.Duration
		for n := 0; n < 6; n++ {
			var b strings.Builder
			for y := 1; y < rows-1; y++ {
				b.WriteString(at(0, y))
				for x := 0; x < cols; x++ {
					c := (x + y + n) % 15
					b.WriteString(col(1+c, (x/8+n)%8))
					b.WriteByte(byte('A' + (x+y*3+n)%26))
				}
			}
			b.WriteString(reset)
			full = append(full, s.con.outTimed([]byte(b.String())))
		}
		r.FullRepaint = stats(full)
		s.header("4/6 redraw speed", f.label+fmt.Sprintf(" · %d × %d · hands off, ~20 s", cols, rows))
		band := 10
		y0 := 3
		for _, fps := range []int{10, 15, 30, 0} {
			var d []time.Duration
			var bytesN int
			start := time.Now()
			frame := 0
			for time.Since(start) < 3*time.Second {
				t := time.Since(start).Seconds()
				var b strings.Builder
				field(&b, 0, y0, cols, band, t)
				b.WriteString(pixels(2, y0+band+1, bootRows(t, "neutral")))
				bytesN = b.Len()
				d = append(d, s.con.outTimed([]byte(b.String())))
				frame++
				if fps > 0 {
					next := start.Add(time.Duration(frame) * time.Second / time.Duration(fps))
					time.Sleep(time.Until(next))
				}
			}
			st := BandStats{TargetFPS: fps, Write: stats(d), Bytes: bytesN,
				AchievedFPS: float64(int(float64(frame)/time.Since(start).Seconds()*10)) / 10}
			r.Band = append(r.Band, st)
			label := fmt.Sprintf("%d fps", fps)
			if fps == 0 {
				label = "flat out"
			}
			s.con.out(at(24, y0+band+2+len(r.Band)) + col(GRY, -1) +
				fmt.Sprintf("%-9s → %5.1f fps, write %5.1f ms mean, %5.1f ms p95", label, st.AchievedFPS, st.Write.Mean, st.Write.P95) + reset)
		}
		s.res.Bench = append(s.res.Bench, r)
	}
	_, rows := s.con.size()
	s.con.out(at(2, rows-4) + col(WHT, -1) + "Did the animation look smooth at 10 and 15 fps?" + reset)
	s.hint("1 smooth · 2 a bit choppy · 3 choppy or flickering · q quit")
	k := s.con.waitFor("1", "2", "3")
	if k == "q" {
		return false
	}
	s.res.BenchSmooth = map[string]string{"1": "smooth", "2": "a_bit_choppy", "3": "choppy"}[k]
	return true
}

// Step 5 and 6: GPU ---------------------------------------------------------

func (s *Spike) gpu() bool {
	s.header("5/6 graphics", "what the kernel sees")
	s.res.DRM = drmInfo()
	var b strings.Builder
	y := 2
	line := func(c int, t string) { b.WriteString(at(2, y) + col(c, -1) + t + reset); y++ }
	for _, c := range s.res.DRM.Cards {
		line(WHT, fmt.Sprintf("%s  %s  driver %s  (%s %s)", c.Card, c.PCI, c.Driver, c.DRMName, c.DRMVersion))
		if c.Error != "" {
			line(BRED, "  "+c.Error)
		}
		for _, cn := range c.Connectors {
			if cn.Status != "connected" {
				continue
			}
			m := ""
			if len(cn.Modes) > 0 {
				m = cn.Modes[0]
			}
			e := ""
			if cn.EDID != nil {
				e = fmt.Sprintf("%s %s %q %d×%d cm", cn.EDID.Maker, cn.EDID.Product, cn.EDID.Name, cn.EDID.WidthCM, cn.EDID.HeightCM)
			}
			line(GRY, fmt.Sprintf("  %s connected  %s  %s", cn.Name, m, e))
		}
	}
	for _, f := range s.res.Mac.Framebuffer {
		line(GRY, fmt.Sprintf("%s  %s  %s  %s bpp", f.Dev, f.Name, f.Size, f.Bpp))
	}
	line(GRY, fmt.Sprintf("kernel log: %d graphics lines, %d errors", len(s.res.DRM.Kmsg), s.res.DRM.Errors))
	s.con.out(b.String())
	s.con.out(at(2, y+1) + col(WHT, -1) + "Next: unpack Mesa into RAM, then draw on each GPU and on the software renderer (llvmpipe)." + reset)
	s.hint("Enter go · q quit")
	if s.con.waitFor("enter") == "q" {
		return false
	}
	return s.egl()
}

type EGLResult struct {
	ExitCode int               `json:"exit_code"`
	Seconds  float64           `json:"seconds"`
	Nodes    []json.RawMessage `json:"nodes"`
	Log      []string          `json:"log"`
}

func (s *Spike) egl() bool {
	s.header("6/6 graphics", "Mesa in RAM + EGL probe · hands off")
	_, rows := s.con.size()
	s.con.out(at(2, 2) + col(GRY, -1) + "Unpacking Mesa from the ISO's offline mirror into RAM…" + reset)
	stop := make(chan bool)
	go func() {
		t := 0.0
		for {
			select {
			case <-stop:
				return
			case <-time.After(150 * time.Millisecond):
				t += 0.15
				s.con.out(pixels(4, 4, bootRows(t, "neutral")) + at(24, 6) + col(GRY, -1) + fmt.Sprintf("%5.1f s", t) + reset)
			}
		}
	}()
	start := time.Now()
	cmd := exec.Command("bash", filepath.Join(s.stick, "egl", "egl.sh"), filepath.Join(s.stick, "egl", "egl-probe"), s.outDir)
	err := cmd.Run()
	stop <- true
	r := EGLResult{Seconds: float64(int(time.Since(start).Seconds()*10)) / 10}
	if cmd.ProcessState != nil {
		r.ExitCode = cmd.ProcessState.ExitCode()
	} else if err != nil {
		r.ExitCode = -1
		r.Log = append(r.Log, err.Error())
	}
	if f, err := os.Open(filepath.Join(s.outDir, "egl.json")); err == nil {
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			if json.Valid(sc.Bytes()) {
				r.Nodes = append(r.Nodes, json.RawMessage(append([]byte(nil), sc.Bytes()...)))
			}
		}
		f.Close()
	}
	if b, err := os.ReadFile(filepath.Join(s.outDir, "egl.log")); err == nil {
		for _, l := range strings.Split(strings.TrimSpace(string(b)), "\n") {
			if !strings.HasPrefix(l, "libEGL info") && !strings.HasPrefix(l, "libEGL debug") {
				r.Log = append(r.Log, l)
			}
		}
	}
	s.res.EGL = r
	s.header("6/6 graphics", "Mesa in RAM + EGL probe")
	var b strings.Builder
	y := 2
	line := func(c int, t string) { b.WriteString(at(2, y) + col(c, -1) + t + reset); y++ }
	line(GRY, fmt.Sprintf("egl.sh exit %d after %.1f s", r.ExitCode, r.Seconds))
	for _, n := range r.Nodes {
		var v struct {
			Node, Error string
			OK          bool
			Contexts    []struct {
				API, Renderer, Version, Error string
				OK                            bool
				DrawOK                        bool    `json:"draw_ok"`
				FPS                           float64 `json:"bench_1080p_fps"`
			}
		}
		json.Unmarshal(n, &v)
		line(WHT, v.Node+" "+v.Error)
		for _, c := range v.Contexts {
			mark, cc := "✓", BGRN
			if !c.OK || (strings.HasPrefix(c.API, "gles") && !c.DrawOK) {
				mark, cc = "×", BRED
			}
			fps := ""
			if c.FPS > 0 {
				fps = fmt.Sprintf("  1080p desktop-like load: %.0f fps", c.FPS)
			}
			line(cc, fmt.Sprintf("  %s %-5s %s  %s %s%s", mark, c.API, c.Version, c.Renderer, c.Error, fps))
		}
	}
	for _, l := range r.Log {
		if y < rows-6 {
			line(DIM, l)
		}
	}
	s.con.out(b.String())
	s.hint("Enter finish · q quit")
	return s.con.waitFor("enter") != "q"
}
