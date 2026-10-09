package main

// --modules: load patched kernel modules from the stick into the live system and test them.
//   applesmc (any pre-T2 Mac): the LED container_of fix, then fans, temperatures, light sensor,
//     keyboard backlight.
//   radeon (iMac10,1 only, booted with nomodeset): the LVDS panel-clock fix, then the GPU tests.
// Results are written after every step, so a hang still leaves what came before.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

type ModulesResults struct {
	Spike    string        `json:"spike"`
	Started  string        `json:"started"`
	Mac      SysInfo       `json:"mac"`
	Applesmc *SMCResult    `json:"applesmc,omitempty"`
	Radeon   *RadeonResult `json:"radeon,omitempty"`
	Finished string        `json:"finished,omitempty"`
}

type SMCResult struct {
	AlreadyLoaded bool     `json:"already_loaded"`
	Insmod        string   `json:"insmod"` // "ok" or the error
	Oops          bool     `json:"oops"`
	Kmsg          []string `json:"kmsg"`
	SysfsDir      string   `json:"sysfs_dir"`
	Fans          []Fan    `json:"fans"`
	Temps         []string `json:"temps"` // "label=°C" for every sensor
	Light         string   `json:"light,omitempty"`
	KbdBacklight  bool     `json:"kbd_backlight"`
	KbdMax        int      `json:"kbd_max_brightness,omitempty"`
	KbdLit        string   `json:"kbd_lit,omitempty"` // tester: y / n
}

type Fan struct {
	Label string `json:"label"`
	RPM   int    `json:"rpm"`
	Min   int    `json:"min"`
	Max   int    `json:"max"`
}

type RadeonResult struct {
	Deps        string     `json:"deps"`
	Insmod      string     `json:"insmod"`
	Params      string     `json:"params"`
	Oops        bool       `json:"oops"`
	Kmsg        []string   `json:"kmsg"`
	RenderNode  string     `json:"render_node"`
	PanelLooks  string     `json:"panel_looks"` // tester: y (right) / n (distorted)
	Framebuffer []FB       `json:"framebuffers"`
	DRM         DRMInfo    `json:"drm"`
	EGL         *EGLResult `json:"egl,omitempty"`
}

var oopsRe = regexp.MustCompile(`(?i)\boops\b|BUG:|general protection|Call Trace|kernel NULL pointer|RIP:`)

// kmsgTail opens the kernel log positioned at its end; read() later returns only new records.
func kmsgTail() int {
	fd, err := unix.Open("/dev/kmsg", unix.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return -1
	}
	unix.Seek(fd, 0, unix.SEEK_END)
	return fd
}

func kmsgRead(fd int) []string {
	if fd < 0 {
		return nil
	}
	var out []string
	buf := make([]byte, 8192)
	for {
		n, err := unix.Read(fd, buf)
		if err == unix.EPIPE {
			continue
		}
		if err != nil || n <= 0 {
			break
		}
		hdr, msg, ok := strings.Cut(string(buf[:n]), ";")
		if !ok {
			continue
		}
		msg, _, _ = strings.Cut(msg, "\n")
		var pri, seq int
		var ts int64
		fmt.Sscanf(hdr, "%d,%d,%d", &pri, &seq, &ts)
		out = append(out, fmt.Sprintf("[%6.2f] <%d> %s", float64(ts)/1e6, pri&7, msg))
	}
	return out
}

func anyOops(lines []string) bool {
	for _, l := range lines {
		if oopsRe.MatchString(l) {
			return true
		}
	}
	return false
}

func moduleLoaded(name string) bool {
	_, err := os.Stat("/sys/module/" + name)
	return err == nil
}

func run(name string, args ...string) string {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return strings.TrimSpace(fmt.Sprintf("%v: %s", err, out))
	}
	return "ok"
}

func readInt(p string) int {
	v, _ := strconv.Atoi(readTrim(p))
	return v
}

func (s *Spike) saveModules(r *ModulesResults) {
	b, _ := json.MarshalIndent(r, "", "  ")
	os.WriteFile(filepath.Join(s.outDir, "modules.json"), append(b, '\n'), 0o644)
	unix.Sync()
}

func (s *Spike) modules() {
	r := &ModulesResults{Spike: version + " --modules", Started: time.Now().Format(time.RFC3339), Mac: s.res.Mac}
	s.saveModules(r)
	s.con.setFont(s.fontList()[1].path)
	s.header("modules", "patched drivers from the stick")
	lines := []string{
		"Loads two patched drivers into this live system (RAM only; a restart undoes it).",
		"",
		"1  applesmc   fans, temperatures, light sensor, keyboard backlight",
		"              (the stock driver is switched off on the ISO because it crashes)",
		"2  radeon     iMac10,1 only: the panel-clock fix, then the GPU test on r600",
		"",
		"Results are saved after each step. If the Mac freezes, hold the power button:",
		"what came before is on the stick.",
	}
	for i, l := range lines {
		s.con.out(at(4, 3+i) + col(GRY, -1) + l + reset)
	}
	s.hint("Enter start · q quit")
	if s.con.waitFor("enter") == "q" {
		return
	}
	if !s.smcStep(r) {
		return
	}
	s.radeonStep(r)
	r.Finished = time.Now().Format(time.RFC3339)
	s.saveModules(r)
}

func smcDir() string {
	for _, pat := range []string{"/sys/bus/acpi/devices/APP0001:*", "/sys/devices/platform/applesmc.*"} {
		ds, _ := filepath.Glob(pat)
		for _, d := range ds {
			if _, err := os.Stat(d + "/fan1_input"); err == nil {
				return d
			}
			if _, err := os.Stat(d + "/temp1_input"); err == nil {
				return d
			}
		}
	}
	return ""
}

func (s *Spike) smcStep(r *ModulesResults) bool {
	res := &SMCResult{AlreadyLoaded: moduleLoaded("applesmc")}
	r.Applesmc = res
	s.header("modules 1/2", "applesmc with the LED fix")
	kf := kmsgTail()
	if res.AlreadyLoaded {
		res.Insmod = "skipped: applesmc already loaded"
	} else {
		res.Insmod = run("insmod", filepath.Join(s.stick, "modules", "applesmc.ko"))
	}
	time.Sleep(2 * time.Second)
	res.Kmsg = kmsgRead(kf)
	res.Oops = anyOops(res.Kmsg)
	unix.Close(kf)
	s.saveModules(r)

	res.SysfsDir = smcDir()
	d := res.SysfsDir
	if d != "" {
		for i := 1; i <= 8; i++ {
			if _, err := os.Stat(fmt.Sprintf("%s/fan%d_input", d, i)); err != nil {
				break
			}
			res.Fans = append(res.Fans, Fan{Label: readTrim(fmt.Sprintf("%s/fan%d_label", d, i)),
				RPM: readInt(fmt.Sprintf("%s/fan%d_input", d, i)), Min: readInt(fmt.Sprintf("%s/fan%d_min", d, i)),
				Max: readInt(fmt.Sprintf("%s/fan%d_max", d, i))})
		}
		ts, _ := filepath.Glob(d + "/temp*_input")
		sort.Slice(ts, func(a, b int) bool {
			na, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(filepath.Base(ts[a]), "temp"), "_input"))
			nb, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(filepath.Base(ts[b]), "temp"), "_input"))
			return na < nb
		})
		for _, t := range ts {
			lab := readTrim(strings.TrimSuffix(t, "_input") + "_label")
			res.Temps = append(res.Temps, fmt.Sprintf("%s=%.1f", lab, float64(readInt(t))/1000))
		}
		res.Light = readTrim(d + "/light")
	}
	led := "/sys/class/leds/smc::kbd_backlight"
	if _, err := os.Stat(led); err == nil {
		res.KbdBacklight = true
		res.KbdMax = readInt(led + "/max_brightness")
	}
	s.saveModules(r)

	var b strings.Builder
	y := 2
	line := func(c int, t string) { b.WriteString(at(2, y) + col(c, -1) + t + reset); y++ }
	mark := func(ok bool) (string, int) {
		if ok {
			return "✓", BGRN
		}
		return "×", BRED
	}
	m, c := mark(res.Insmod == "ok" || res.AlreadyLoaded)
	line(c, fmt.Sprintf("%s insmod applesmc: %s", m, res.Insmod))
	m, c = mark(!res.Oops)
	line(c, fmt.Sprintf("%s kernel log: %d new lines, oops: %v", m, len(res.Kmsg), res.Oops))
	line(GRY, "  sysfs: "+d)
	for _, f := range res.Fans {
		line(WHT, fmt.Sprintf("  fan %-10s %5d rpm   (min %d, max %d)", f.Label, f.RPM, f.Min, f.Max))
	}
	line(WHT, fmt.Sprintf("  %d temperature sensors, light sensor: %q", len(res.Temps), res.Light))
	for i, t := range res.Temps {
		if i >= 6 {
			line(DIM, "  …")
			break
		}
		line(DIM, "  "+t)
	}
	if !res.KbdBacklight {
		line(GRY, "  no keyboard backlight on this Mac")
	}
	s.con.out(b.String())

	if res.KbdBacklight {
		s.con.out(at(2, y+1) + col(WHT, -1) + "Watch the keyboard: its backlight goes full, off, full, then back." + reset)
		s.hint("Enter start · q quit")
		if s.con.waitFor("enter") == "q" {
			return false
		}
		bp := led + "/brightness"
		orig := readTrim(bp)
		for _, v := range []int{res.KbdMax, 0, res.KbdMax, 0} {
			os.WriteFile(bp, []byte(strconv.Itoa(v)), 0o644)
			time.Sleep(1200 * time.Millisecond)
		}
		os.WriteFile(bp, []byte(orig), 0o644)
		s.con.out(at(2, y+1) + "\x1b[2K" + col(WHT, -1) + "Did the keyboard light up and go dark (twice)?" + reset)
		s.hint("y yes · n no · q quit")
		k := s.con.waitFor("y", "n")
		if k == "q" {
			return false
		}
		res.KbdLit = k
		s.saveModules(r)
	}
	s.hint("Enter next · q quit")
	return s.con.waitFor("enter") != "q"
}

// The iMac10,1 panel: Radeon HD 4670 1002:9488 with Apple subsystem 106b:00b6.
func imac101Radeon() bool {
	ds, _ := filepath.Glob("/sys/bus/pci/devices/*")
	for _, d := range ds {
		if readTrim(d+"/vendor") == "0x1002" && readTrim(d+"/device") == "0x9488" &&
			readTrim(d+"/subsystem_vendor") == "0x106b" && readTrim(d+"/subsystem_device") == "0x00b6" {
			return true
		}
	}
	return false
}

func (s *Spike) radeonStep(r *ModulesResults) {
	s.header("modules 2/2", "radeon with the iMac10,1 panel fix")
	if !imac101Radeon() {
		s.con.out(at(2, 3) + col(GRY, -1) + "Not an iMac10,1 with the Radeon HD 4670 panel: nothing to do here." + reset)
		s.hint("Enter finish")
		s.con.waitFor("enter")
		return
	}
	res := &RadeonResult{Params: "modeset=1 dpm=0 pcie_gen2=0 uvd=0"}
	r.Radeon = res
	if moduleLoaded("radeon") {
		res.Insmod = "skipped: radeon already loaded (boot with the nomodeset entry for this test)"
		s.saveModules(r)
		s.con.out(at(2, 3) + col(BRED, -1) + res.Insmod + reset)
		s.hint("Enter finish")
		s.con.waitFor("enter")
		return
	}
	lines := []string{
		"Next, the screen goes black for a moment while the driver takes over.",
		"Then this console should come back, sharp, filling the screen.",
		"",
		"After that, press:  y  if the screen looks right",
		"                    n  if it's split or distorted (press it even if you can't read anything)",
		"",
		"Then the GPU test runs by itself (~10 s) and Enter finishes.",
	}
	for i, l := range lines {
		s.con.out(at(4, 3+i) + col(WHT, -1) + l + reset)
	}
	s.hint("Enter load the driver · q quit")
	if s.con.waitFor("enter") == "q" {
		return
	}
	kf := kmsgTail()
	res.Deps = run("modprobe", "-a", "drm_ttm_helper", "drm_display_helper", "drm_exec", "ttm", "drm_suballoc_helper", "video", "i2c-algo-bit")
	args := append([]string{filepath.Join(s.stick, "modules", "radeon.ko")}, strings.Fields(res.Params)...)
	res.Insmod = run("insmod", args...)
	for i := 0; i < 50; i++ {
		if n, _ := filepath.Glob("/dev/dri/renderD*"); len(n) > 0 {
			res.RenderNode = n[0]
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	time.Sleep(2 * time.Second)
	res.Kmsg = kmsgRead(kf)
	res.Oops = anyOops(res.Kmsg)
	unix.Close(kf)
	s.saveModules(r)

	// the console may have changed hands (simpledrm → radeondrmfb): set the font again, redraw
	s.con.setFont(s.fontList()[1].path)
	s.header("modules 2/2", "radeon loaded: how does the screen look?")
	s.con.out(at(2, 3) + col(WHT, -1) + fmt.Sprintf("insmod: %s · render node: %s", res.Insmod, res.RenderNode) + reset)
	s.con.out(pixels(4, 6, bootRows(1.0, "pleased")))
	s.con.out(at(24, 8) + col(BCYN, -1) + "Boot: " + col(GRY, -1) + "if you can read this, the panel clock is right." + reset)
	s.hint("y screen looks right · n split or distorted")
	res.PanelLooks = s.con.waitFor("y", "n")
	if res.PanelLooks == "q" {
		res.PanelLooks = ""
	}
	s.saveModules(r)

	sys := sysInfo()
	res.Framebuffer = sys.Framebuffer
	res.DRM = drmInfo()
	s.saveModules(r)
	if res.RenderNode != "" {
		s.egl()
		e := s.res.EGL
		res.EGL = &e
		s.saveModules(r)
		return
	}
	s.hint("Enter finish")
	s.con.waitFor("enter")
}
