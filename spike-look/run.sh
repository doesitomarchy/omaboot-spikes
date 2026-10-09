#!/bin/bash
# OmaBoot? Live spike: run as root on tty2 of the official Omarchy ISO.
#   mount -L BOOTLIVE /mnt && /mnt/run.sh
D=$(dirname "$(readlink -f "$0")")
[ "$(id -u)" = 0 ] || { echo "Run this as root (the live ISO's tty2 login is root)."; exit 1; }
case "$(tty)" in /dev/tty[0-9]*) ;; *) echo "Run this on a text console (Ctrl+Alt+F2), not over SSH or in a window."; exit 1 ;; esac
mkdir -p "$D/results"
"$D/spike-look" --stick "$D" "$@"
sync
