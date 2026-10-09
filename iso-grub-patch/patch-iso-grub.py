#!/usr/bin/env python3
"""Make a copy of the official Omarchy ISO whose GRUB menu is visible, with a nomodeset entry.

The official ISO boots straight into the installer (GRUB timeout=0, timeout_style=hidden), so
there is no way to add a kernel parameter. Some Macs need `nomodeset` to get a usable screen
(the iMac10,1's LVDS panel comes up split into four distorted parts). This script edits
boot/grub/grub.cfg inside a copy of the ISO, in place and at the same length, so nothing else
in the image moves:

  - timeout=0 / timeout_style=hidden  ->  timeout=15 / timeout_style=menu
  - the "speakup screen reader" entry ->  "Omarchy, nomodeset (safe graphics)" (hotkey s),
    its accessibility=on parameter    ->  nomodeset

Usage: patch-iso-grub.py omarchy-X.Y.Z.iso [out.iso]
Needs bsdtar (libarchive) to read grub.cfg back out of the ISO. Verify the official ISO's
.sha256 first; the patched copy gets its own checksum, printed at the end.
"""
import hashlib, subprocess, sys

EDITS = [
    (b'timeout=0\ntimeout_style=hidden', b'timeout=15\ntimeout_style=menu'),
    (b'menuentry "Omarchy with speakup screen reader (', b'menuentry "Omarchy, nomodeset (safe graphics) ('),
    (b' accessibility=on ', b' nomodeset '),
]


def grub_cfg(iso):
    return subprocess.run(['bsdtar', '-xOf', iso, 'boot/grub/grub.cfg'], check=True, capture_output=True).stdout


def main():
    if len(sys.argv) not in (2, 3):
        sys.exit(__doc__)
    src = sys.argv[1]
    dst = sys.argv[2] if len(sys.argv) == 3 else src.removesuffix('.iso') + '-menu-nomodeset.iso'
    cfg = grub_cfg(src)
    new = cfg
    for old, rep in EDITS:
        if new.count(old) != 1:
            sys.exit(f'grub.cfg has {new.count(old)} copies of {old!r}, expected 1: not the ISO layout this script knows')
        rep = rep.ljust(len(old), b' ')
        assert len(rep) == len(old)
        new = new.replace(old, rep)
    data = bytearray(open(src, 'rb').read())
    if data.count(cfg) != 1:
        sys.exit('could not find exactly one copy of grub.cfg in the image')
    at = data.find(cfg)
    data[at:at + len(cfg)] = new
    with open(dst, 'wb') as f:
        f.write(data)
    if grub_cfg(dst) != new:
        sys.exit('read-back of the patched grub.cfg did not match')
    changed = sum(a != b for a, b in zip(cfg, new))
    print(f'{dst}: {changed} bytes changed, all inside boot/grub/grub.cfg')
    print(f'sha256 {hashlib.sha256(data).hexdigest()}')


if __name__ == '__main__':
    main()
