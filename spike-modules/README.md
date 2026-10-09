# spike-modules

Can a test stick carry per-Mac kernel fixes and load them into the official ISO's live system,
without rebuilding the ISO? Two real cases:

- **applesmc** (any pre-T2 Mac): the ISO blacklists applesmc because t2linux patch 3001 makes
  it oops while registering the keyboard-backlight LED: `applesmc_brightness_set()` takes
  `dev_get_drvdata(led_cdev->dev)`, which is the LED class device, not the SMC, and the default
  `nand-disk` trigger calls it during registration. [`patches/0001-applesmc-led-container_of.patch`](patches/0001-applesmc-led-container_of.patch)
  gets the SMC with `container_of` instead, as the driver's work function already does. Without
  applesmc the live system has no fan speeds, temperatures, light sensor or keyboard backlight.
- **radeon** (iMac (21.5-inch, Late 2009) [iMac10,1]): the ATOM BIOS gives the LG LM215WF3-SLA1
  panel a 108 MHz pixel clock where its EDID says 138.5 MHz, so the panel shows four distorted
  parts. The fix, from [choyer/omarchy-vintage-intel-imac-fix](https://github.com/choyer/omarchy-vintage-intel-imac-fix),
  corrects the driver's RAM copy for that GPU and panel only. Booted with `nomodeset`, radeon
  still loads when given `modeset=1`, so the stick can bring it up after boot.

`build-modules.sh ISO WORKDIR` builds both for the Omarchy 4.0.4 ISO's kernel
(`7.2.4-arch1-Watanare-T2-3-t2`): headers from the ISO's own offline package mirror, sources from
Arch's `v7.2.4-arch1` tag plus t2linux's patches as of the package build. It first rebuilds each
module unpatched and checks it against the one the ISO ships (applesmc by `srcversion`, radeon's
`.text` byte for byte), so the only difference in ours is the fix. Output goes to
`../spike-look/modules/`; `../spike-look/build.sh` puts it on the stick, and
`run-modules.sh` (`spike-look --modules`) loads and tests it.

## Results (2026-10-09, Omarchy 4.0.4 ISO)

Raw results are in [`results/`](results/). Both fixes worked on every Mac they apply to, with no
oops in the kernel log.

| Mac | applesmc (fixed) | radeon (panel fix) |
|---|---|---|
| [iMac (21.5-inch, Late 2009) [iMac10,1]](https://doesitomarchy.com/mac/imac10-1) | loads; 3 fans (ODD, HDD, CPU), 31 temperature sensors, light sensor | **panel right** (tester: y): LVDS 1920×1080 on radeondrmfb; RV730 GLES 3.0 / GL 3.3, test triangle correct, 1080p desktop-like load 464 fps (llvmpipe 32) |
| [MacBook Air (13-inch, Mid 2012) [MacBookAir5,2]](https://doesitomarchy.com/mac/macbookair5-2) | loads **with the keyboard-backlight key** (the case that crashed): backlight blinked (tester: y); 1 fan, 30 sensors, light sensor | n/a |
| [iMac (Retina 5K, 27-inch, 2017) [iMac18,3]](https://doesitomarchy.com/mac/imac18-3) | loads; 1 fan, 93 sensors, light sensor | n/a |

So a test stick can carry per-Mac kernel fixes into the official ISO's live system: the live
system gains the SMC (fans, temperatures, light sensor, keyboard backlight) on pre-T2 Macs, and
the iMac10,1 gets its own panel and GPU instead of `nomodeset`. The modules are unsigned, so the
kernel marks itself tainted; that's expected for out-of-tree modules.
