// spike-look: OmaBoot? Live spikes 1 and 4 on the official Omarchy ISO (see ../README.md).
// Fonts, palette, a Bubble Tea screen and redraw speed on the Mac's own console; then what the
// kernel's graphics stack shows and whether Mesa can draw. Results go to the stick as JSON.
//
// Run as root on a VT (tty2 of the live ISO): ./run.sh   ·   laptop check: spike-look --selftest
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"syscall"
	"time"
)

const version = "spike-look 1 (2026-10-08)"

type Results struct {
	Spike       string         `json:"spike"`
	Started     string         `json:"started"`
	Finished    string         `json:"finished,omitempty"`
	Completed   bool           `json:"completed"`
	Mac         SysInfo        `json:"mac"`
	StartGrid   [2]int         `json:"start_grid"` // cols, rows with the font the ISO left us
	Fonts       []FontRating   `json:"fonts"`
	Palette     PaletteAnswers `json:"palette"`
	Layout      LayoutResult   `json:"bubbletea_screen"`
	Bench       []BenchResult  `json:"redraw"`
	BenchSmooth string         `json:"redraw_looked"`
	DRM         DRMInfo        `json:"drm"`
	EGL         EGLResult      `json:"egl"`
}

type Spike struct {
	con    *Console
	res    Results
	stick  string
	outDir string
}

func main() {
	selftest := flag.Bool("selftest", false, "no console: collect what can be collected and write results (for the laptop)")
	mods := flag.Bool("modules", false, "load the patched applesmc/radeon modules from the stick and test them")
	stick := flag.String("stick", ".", "the stick's folder (fonts/, egl/, results/)")
	flag.Parse()

	s := &Spike{stick: *stick}
	s.res.Spike = version
	s.res.Started = time.Now().Format(time.RFC3339)
	s.res.Mac = sysInfo()
	name := regexp.MustCompile(`[^A-Za-z0-9,._-]+`).ReplaceAllString(s.res.Mac.Model, "_")
	if name == "" {
		name = "unknown"
	}
	kind := ""
	if *mods {
		kind = "_modules"
	}
	s.outDir = filepath.Join(*stick, "results", name+kind+"_"+time.Now().Format("20060102-150405"))
	if err := os.MkdirAll(s.outDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "can't write results:", err)
		os.Exit(1)
	}
	tmp, _ := os.MkdirTemp("", "spike-look")
	con, err := openConsole(*selftest, tmp)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	s.con = con
	c, r := con.size()
	s.res.StartGrid = [2]int{c, r}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGHUP)
	go func() { <-sig; s.finish(); os.Exit(1) }()

	s.con.setPalette(palettes[0].c)
	if *mods {
		s.modules()
		s.con.restore()
		fmt.Printf("Results: %s\n", s.outDir)
		fmt.Println("Done. Type: sync; umount /mnt   then unplug the stick.")
		return
	}
	if s.intro() && s.fonts() && s.palette() && s.layout() && s.bench() && s.gpu() {
		s.res.Completed = true
	}
	if *selftest {
		s.res.DRM = drmInfo()
	}
	s.finish()
	fmt.Printf("Results: %s\n", s.outDir)
	if s.res.Completed {
		fmt.Println("All done. Type: sync; umount /mnt   then unplug the stick.")
	} else {
		fmt.Println("Stopped early; partial results saved. Run ./run.sh again any time.")
	}
}

func (s *Spike) finish() {
	s.con.restore()
	s.res.Finished = time.Now().Format(time.RFC3339)
	b, _ := json.MarshalIndent(s.res, "", "  ")
	os.WriteFile(filepath.Join(s.outDir, "result.json"), append(b, '\n'), 0o644)
	exec := func(name string, lines []string) {
		f, err := os.Create(filepath.Join(s.outDir, name))
		if err != nil {
			return
		}
		for _, l := range lines {
			fmt.Fprintln(f, l)
		}
		f.Close()
	}
	exec("kmsg-gpu.txt", s.res.DRM.Kmsg)
	syscall.Sync()
}

func (s *Spike) intro() bool {
	if s.con.selftest {
		return true
	}
	s.con.setFont(s.fontList()[1].path)
	cols, rows := s.con.size()
	s.header("start", "about 5 minutes")
	var lines = []string{
		"Six short steps. Most need one key from you; two run by themselves.",
		"",
		"1  fonts       how big text looks from where you sit",
		"2  palette     our 16 colours on this screen",
		"3  layout      a mock-up of the real tester screen",
		"4  redraw      how fast this console can animate (hands off)",
		"5  graphics    what the kernel sees",
		"6  Mesa        draw on each GPU and in software (hands off)",
		"",
		"Nothing is written to this Mac's disks. Results go to the stick.",
	}
	for i, l := range lines {
		s.con.out(at(4, 3+i) + col(GRY, -1) + l + reset)
	}
	if rows > 30 {
		s.con.out(pixels(cols-24, 3, bootRows(0.5, "pleased")))
	}
	s.hint("Enter start · q quit at any step (results so far are kept)")
	return s.con.waitFor("enter") != "q"
}
