#!/bin/bash
# OmaBoot? Live spike: load the patched applesmc (and, on the iMac10,1, radeon) from this stick.
#   mount -L BOOTLIVE /mnt && /mnt/run-modules.sh
D=$(dirname "$(readlink -f "$0")")
[ "$(id -u)" = 0 ] || { echo "Run this as root (the live ISO's tty2 login is root)."; exit 1; }
case "$(tty)" in /dev/tty[0-9]*) ;; *) echo "Run this on a text console (Option+→ from the installer)."; exit 1 ;; esac
[ "$(uname -r)" = 7.2.4-arch1-Watanare-T2-3-t2 ] || { echo "These modules are built for kernel 7.2.4-arch1-Watanare-T2-3-t2 (the Omarchy 4.0.4 ISO); this is $(uname -r)."; exit 1; }
mkdir -p "$D/results"
"$D/spike-look" --modules --stick "$D" "$@"
sync
