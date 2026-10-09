package main

// What Mac this is and how its console is driven. No serial numbers or UUIDs are read.

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

type SysInfo struct {
	Model       string   `json:"model"`        // product_name, e.g. MacBookAir5,2
	BoardID     string   `json:"board_id"`     // board_name, e.g. Mac-66F35F19FE2A0D05
	Vendor      string   `json:"vendor"`       // sys_vendor
	Firmware    string   `json:"firmware"`     // bios_version
	CPU         string   `json:"cpu"`          // model name
	MemMiB      int      `json:"mem_mib"`      // MemTotal
	Kernel      string   `json:"kernel"`       // uname -r
	Cmdline     string   `json:"cmdline"`      // /proc/cmdline
	EFIBits     string   `json:"efi_bits"`     // fw_platform_size
	Omarchy     string   `json:"omarchy_iso"`  // /etc/os-release or the ISO version file, when present
	Framebuffer []FB     `json:"framebuffers"` // /sys/class/graphics/fb*
	GPUModules  []string `json:"gpu_modules"`
}

type FB struct {
	Dev    string `json:"dev"`
	Name   string `json:"name"` // e.g. i915drmfb, radeondrmfb, simpledrmfb, efifb
	Size   string `json:"virtual_size"`
	Bpp    string `json:"bits_per_pixel"`
	Stride string `json:"stride"`
}

func readTrim(p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func sysInfo() SysInfo {
	d := "/sys/class/dmi/id/"
	s := SysInfo{Model: readTrim(d + "product_name"), BoardID: readTrim(d + "board_name"),
		Vendor: readTrim(d + "sys_vendor"), Firmware: readTrim(d + "bios_version"),
		Cmdline: uuidRe.ReplaceAllString(readTrim("/proc/cmdline"), "<uuid>"), EFIBits: readTrim("/sys/firmware/efi/fw_platform_size")}
	var u unix.Utsname
	if unix.Uname(&u) == nil {
		s.Kernel = unix.ByteSliceToString(u.Release[:])
	}
	if f, err := os.Open("/proc/cpuinfo"); err == nil {
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			if k, v, ok := strings.Cut(sc.Text(), ":"); ok && strings.TrimSpace(k) == "model name" {
				s.CPU = strings.TrimSpace(v)
				break
			}
		}
		f.Close()
	}
	if f, err := os.Open("/proc/meminfo"); err == nil {
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			var kb int
			if n, _ := fmt.Sscanf(sc.Text(), "MemTotal: %d kB", &kb); n == 1 {
				s.MemMiB = kb / 1024
				break
			}
		}
		f.Close()
	}
	for _, p := range []string{"/etc/omarchy-iso-version", "/usr/share/omarchy-iso/version"} {
		if v := readTrim(p); v != "" {
			s.Omarchy = v
			break
		}
	}
	fbs, _ := filepath.Glob("/sys/class/graphics/fb[0-9]*")
	for _, p := range fbs {
		s.Framebuffer = append(s.Framebuffer, FB{Dev: filepath.Base(p), Name: readTrim(p + "/name"),
			Size: readTrim(p + "/virtual_size"), Bpp: readTrim(p + "/bits_per_pixel"), Stride: readTrim(p + "/stride")})
	}
	gpu := regexp.MustCompile(`^(i915|xe|radeon|amdgpu|nouveau|nvidia\w*|apple_gmux|drm\w*|ttm|simpledrm|efifb|fbcon|gma500\w*|backlight|video|apple_bl|applesmc|mgag200|ast)$`)
	if f, err := os.Open("/proc/modules"); err == nil {
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			if m := strings.Fields(sc.Text()); len(m) > 0 && gpu.MatchString(m[0]) {
				s.GPUModules = append(s.GPUModules, m[0])
			}
		}
		f.Close()
	}
	return s
}

// DRM ------------------------------------------------------------------------

type DRMInfo struct {
	Cards  []Card   `json:"cards"`
	Kmsg   []string `json:"kmsg_gpu"` // kernel log lines about graphics, firmware and the SMC
	Errors int      `json:"kmsg_errors"`
}

type Card struct {
	Card       string      `json:"card"`
	PCI        string      `json:"pci"` // vendor:device
	Subsys     string      `json:"subsys"`
	BootVGA    string      `json:"boot_vga"`
	Driver     string      `json:"driver"`   // kernel module bound to the device
	DRMName    string      `json:"drm_name"` // DRM_IOCTL_VERSION
	DRMVersion string      `json:"drm_version"`
	DRMDesc    string      `json:"drm_desc"`
	RenderNode string      `json:"render_node"`
	Connectors []Connector `json:"connectors"`
	Error      string      `json:"error,omitempty"`
}

type Connector struct {
	Name    string   `json:"name"`
	Status  string   `json:"status"`
	Enabled string   `json:"enabled"`
	Modes   []string `json:"modes"`
	EDID    *EDID    `json:"edid,omitempty"`
}

type EDID struct {
	Maker     string `json:"maker"`   // PNP ID, e.g. APP
	Product   string `json:"product"` // hex product code
	Name      string `json:"name"`
	Year      int    `json:"year"`
	WidthCM   int    `json:"width_cm"`
	HeightCM  int    `json:"height_cm"`
	Preferred string `json:"preferred_mode"`
	Bytes     int    `json:"bytes"`
}

// drm_version as the kernel lays it out on amd64.
type drmVersion struct {
	major, minor, patch int32
	_                   int32
	nameLen             uint64
	name                *byte
	dateLen             uint64
	date                *byte
	descLen             uint64
	desc                *byte
}

const drmIoctlVersion = 0xC0406400 // _IOWR('d', 0x00, struct drm_version), 64 bytes

func drmVersionOf(dev string) (name, ver, desc string, err error) {
	fd, err := unix.Open(dev, unix.O_RDWR|unix.O_CLOEXEC, 0)
	if err != nil {
		return "", "", "", err
	}
	defer unix.Close(fd)
	var v drmVersion
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), drmIoctlVersion, uintptr(unsafe.Pointer(&v))); e != 0 {
		return "", "", "", e
	}
	nb, db, sb := make([]byte, v.nameLen+1), make([]byte, v.dateLen+1), make([]byte, v.descLen+1)
	v.name, v.date, v.desc = &nb[0], &db[0], &sb[0]
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), drmIoctlVersion, uintptr(unsafe.Pointer(&v))); e != 0 {
		return "", "", "", e
	}
	return string(nb[:v.nameLen]), fmt.Sprintf("%d.%d.%d (%s)", v.major, v.minor, v.patch, db[:v.dateLen]), string(sb[:v.descLen]), nil
}

func parseEDID(b []byte) *EDID {
	if len(b) < 128 || b[0] != 0 || b[1] != 0xff {
		return nil
	}
	m := uint16(b[8])<<8 | uint16(b[9])
	e := &EDID{Maker: string([]byte{byte(m>>10&31) + 64, byte(m>>5&31) + 64, byte(m&31) + 64}),
		Product: fmt.Sprintf("%04x", uint16(b[11])<<8|uint16(b[10])), Year: int(b[17]) + 1990,
		WidthCM: int(b[21]), HeightCM: int(b[22]), Bytes: len(b)}
	// bytes 12–15 (serial) and descriptor 0xFF (serial text) are never read
	for i := 0; i < 4; i++ {
		d := b[54+18*i : 72+18*i]
		if d[0] != 0 || d[1] != 0 {
			if i == 0 {
				clk := int(d[0]) | int(d[1])<<8
				ha := int(d[2]) | int(d[4]&0xf0)<<4
				va := int(d[5]) | int(d[7]&0xf0)<<4
				hb := int(d[3]) | int(d[4]&0x0f)<<8
				vb := int(d[6]) | int(d[7]&0x0f)<<8
				hz := 0.0
				if (ha+hb)*(va+vb) > 0 {
					hz = float64(clk) * 10000 / float64((ha+hb)*(va+vb))
				}
				e.Preferred = fmt.Sprintf("%dx%d@%.1f", ha, va, hz)
			}
			continue
		}
		if d[3] == 0xfc {
			e.Name = strings.TrimSpace(strings.TrimRight(string(d[5:18]), "\n "))
		}
	}
	return e
}

var kmsgRe = regexp.MustCompile(`(?i)drm|radeon|amdgpu|i915|nouveau|nvidia|fbcon|framebuffer|\bfb\d|efifb|simpledrm|firmware|\bgpu\b|vga|edid|backlight|gmux|applesmc|mesa|kms`)

func drmInfo() DRMInfo {
	var d DRMInfo
	cards, _ := filepath.Glob("/sys/class/drm/card[0-9]")
	cards2, _ := filepath.Glob("/sys/class/drm/card[0-9][0-9]")
	for _, p := range append(cards, cards2...) {
		c := Card{Card: filepath.Base(p)}
		dev := p + "/device/"
		c.PCI = strings.TrimPrefix(readTrim(dev+"vendor"), "0x") + ":" + strings.TrimPrefix(readTrim(dev+"device"), "0x")
		c.Subsys = strings.TrimPrefix(readTrim(dev+"subsystem_vendor"), "0x") + ":" + strings.TrimPrefix(readTrim(dev+"subsystem_device"), "0x")
		c.BootVGA = readTrim(dev + "boot_vga")
		if l, err := os.Readlink(dev + "driver"); err == nil {
			c.Driver = filepath.Base(l)
		}
		if rn, _ := filepath.Glob(dev + "drm/renderD*"); len(rn) > 0 {
			c.RenderNode = "/dev/dri/" + filepath.Base(rn[0])
		}
		var err error
		c.DRMName, c.DRMVersion, c.DRMDesc, err = drmVersionOf("/dev/dri/" + c.Card)
		if err != nil {
			c.Error = err.Error()
		}
		conns, _ := filepath.Glob(p + "/" + c.Card + "-*")
		for _, cp := range conns {
			cn := Connector{Name: strings.TrimPrefix(filepath.Base(cp), c.Card+"-"), Status: readTrim(cp + "/status"), Enabled: readTrim(cp + "/enabled")}
			if m := readTrim(cp + "/modes"); m != "" {
				cn.Modes = strings.Fields(m)
				if len(cn.Modes) > 6 {
					cn.Modes = cn.Modes[:6]
				}
			}
			if b, err := os.ReadFile(cp + "/edid"); err == nil {
				cn.EDID = parseEDID(b)
			}
			c.Connectors = append(c.Connectors, cn)
		}
		d.Cards = append(d.Cards, c)
	}
	// kernel log: read what is buffered, without blocking
	if fd, err := unix.Open("/dev/kmsg", unix.O_RDONLY|unix.O_NONBLOCK, 0); err == nil {
		buf := make([]byte, 8192)
		for {
			n, err := unix.Read(fd, buf)
			if err == unix.EPIPE {
				continue // overwritten records; keep reading
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
			if kmsgRe.MatchString(msg) {
				if pri&7 <= 3 {
					d.Errors++
				}
				d.Kmsg = append(d.Kmsg, fmt.Sprintf("[%6.2f] <%d> %s", float64(ts)/1e6, pri&7, msg))
			}
		}
		unix.Close(fd)
	}
	return d
}

// disk and partition UUIDs on the kernel command line are dropped from results
var uuidRe = regexp.MustCompile(`(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
