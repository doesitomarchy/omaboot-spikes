# iso-grub-patch

The official Omarchy ISO hides its GRUB menu (`timeout=0`, `timeout_style=hidden`), so you can't
add a kernel parameter. Some Macs need one: the
[iMac (21.5-inch, Late 2009) [iMac10,1]](https://doesitomarchy.com/mac/imac10-1) shows its panel
split into four distorted parts unless it boots with `nomodeset`.

`patch-iso-grub.py` makes a copy of the ISO with the menu shown for 15 seconds and a
**Omarchy, nomodeset (safe graphics)** entry (hotkey `s`) in place of the screen-reader entry.
It edits only `boot/grub/grub.cfg`, at the same length, so nothing else in the image moves. On
Omarchy 4.0.4 that's 65 bytes.

```sh
sha256sum -c omarchy-4.0.4.iso.sha256        # check the official download first
./patch-iso-grub.py omarchy-4.0.4.iso         # writes omarchy-4.0.4-menu-nomodeset.iso
```

Needs Python 3 and `bsdtar`. Write the result to a stick as you would the official ISO. Some
USB 3 sticks don't show up in the Option-key picker on 2009–2012 Macs; if yours is missing,
try another stick (a USB 2 stick worked on a MacBook Air that never showed our USB 3 one).
