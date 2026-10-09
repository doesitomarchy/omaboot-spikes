# spike-look

Runs on the **official Omarchy ISO**, from a second USB stick, on the Mac's text console. It
checks what a console test tool can look like on real Macs and whether the live system can use
the GPU. Nothing is written to the Mac's disks; results go to the stick.

## What it does (about 5 minutes)

1. **Fonts:** loads console fonts at 16, 24 and 32 px (our `omaboot-v` build, stock Terminus,
   Spleen up to 32×64); you rate each one too small, good or too big.
2. **Palette:** sets a 16-colour palette and asks whether bright text and bright backgrounds
   look brighter.
3. **Layout:** a [Bubble Tea](https://github.com/charmbracelet/bubbletea) mock-up of the real
   tool's screen at 15 fps.
4. **Redraw speed** (hands off): times a full-screen repaint and an animation band at
   10, 15, 30 fps and flat out, in each of our three font sizes.
5. **Graphics:** DRM driver, connectors and display EDID (no serial numbers), plus kernel log
   lines about graphics.
6. **Mesa** (hands off): the live system has no Mesa, so `egl/egl.sh` unpacks it from the ISO's
   own offline package mirror into RAM. `egl/egl-probe` then opens GBM + EGL on each GPU and on
   llvmpipe (software), checks a drawn triangle, and times a desktop-like 1080p load (wallpaper
   plus two blended windows).

`omaboot-v` is a 512-glyph build of Terminus (OFL) with half-blocks, double lines, a hand-drawn
✓ and a filled ▲ added and some Cyrillic dropped. Terminus's licence reserves its name, hence
ours. Built by `tools/mkpsf.py` from the `terminus-font` package.

## Build

Needs Go, gcc, and the Mesa and libglvnd headers (`mesa`, `libglvnd` on Arch).

```sh
./build.sh          # → dist/BOOTLIVE/
```

Format a stick as FAT32 with the label `BOOTLIVE` and copy the contents of `dist/BOOTLIVE/` to it.

## Run

1. Boot the Mac from the Omarchy ISO stick (hold Option, pick **EFI Boot**). If the screen comes
   up distorted, use a stick made with [`../iso-grub-patch`](../iso-grub-patch/) and pick the
   nomodeset entry.
2. At the installer, press **Option+→** to switch to tty2 (or Ctrl+Option+Fn+F2), and log in
   as `root` (no password).
3. Plug in the BOOTLIVE stick and run `mount -L BOOTLIVE /mnt && /mnt/run.sh`.
4. When it's done: `sync; umount /mnt`.

## Results (2026-10-08, Omarchy 4.0.4 ISO, kernel 7.2.4-arch1-Watanare-T2-3-t2)

Raw results are in [`results/`](results/). The `tester-note.txt` files correct one answer:
"choppy" on the iMac10,1 meant flicker between steps, not during animation.

| Mac | Console | Font rated good | Animation frame | GPU, 1080p load | llvmpipe |
|---|---|---|---|---|---|
| [iMac (21.5-inch, Late 2009) [iMac10,1]](https://doesitomarchy.com/mac/imac10-1), `nomodeset` | simpledrm, 1920×1080 | 24 px | 1.5–2.4 ms | none (radeon off) | 34 fps |
| [MacBook Air (13-inch, Mid 2012) [MacBookAir5,2]](https://doesitomarchy.com/mac/macbookair5-2) | i915, 1440×900 | 16 px | 1.6–2.4 ms | HD 4000, GLES 3.0: 206 fps | 38 fps |
| [iMac (Retina 5K, 27-inch, 2017) [iMac18,3]](https://doesitomarchy.com/mac/imac18-3) | amdgpu, 3840×2160 | 24–32 px | 3.7–4.7 ms | Radeon Pro 57x, GLES 3.2: 1,626 fps | 98 fps |

What we learned:

- **Pick the font from the screen's physical size.** "Good" came out at about 5 mm per text
  line on the iMacs and about 4 mm on the laptop. The display's EDID gives its size, so the
  tool can choose 16, 24 or 32 px by itself.
- **16 colours work as planned.** Bright text works with a 512-glyph font on all three consoles;
  bright backgrounds never do.
- **Animation is cheap everywhere.** A full-width animation band costs 1–5 ms per frame even on
  the unaccelerated simpledrm console, so 15 fps uses under 8% of a frame. The only flicker came
  from changing fonts (the console resizes and clears), so the tool should set its font once,
  before the first screen.
- **The GPU can be tested from the live ISO.** Unpacking Mesa into RAM takes 0.6–4 s and about
  0.9 GiB of `/tmp`, which is tight on 2 GB Macs. GBM + EGL worked on i915, radeonsi and
  llvmpipe.
- **Name GPUs by subsystem ID.** Mesa calls the iMac18,3's Radeon Pro 57x "AMD Radeon RX 470
  Graphics" from its PCI ID (1002:67df); the subsystem ID (106b:0163) tells them apart.
- **The 5K iMac's console runs at 3840×2160**, scaled by the panel, the same as installed Omarchy.
- **Some USB 3 sticks don't show up** in the Option-key picker: our USB 3 stick took several
  tries on the iMac10,1 and never appeared on the MacBook Air. A USB 2 stick worked on the Air
  and the iMac18,3.

## Licences

Code: MIT (see [`../LICENSE`](../LICENSE)). Fonts: `omaboot-v*` are modified Terminus, SIL OFL
1.1 ([`fonts/LICENSE-terminus.txt`](fonts/LICENSE-terminus.txt)); `spleen-*` are Spleen 2.2.0,
BSD 2-Clause ([`fonts/LICENSE-spleen.txt`](fonts/LICENSE-spleen.txt)).
