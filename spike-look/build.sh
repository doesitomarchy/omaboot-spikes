#!/bin/bash
# Builds the stick folder dist/BOOTLIVE/. Copy its contents to a FAT32 stick labelled BOOTLIVE.
set -euo pipefail
cd "$(dirname "$0")"
rm -rf dist
D=dist/BOOTLIVE
mkdir -p "$D/fonts" "$D/egl" "$D/results"
CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o "$D/spike-look" .
gcc -O2 -Wall -o "$D/egl/egl-probe" egl/egl-probe.c -lEGL -lGLESv2 -lgbm
cp egl/egl.sh "$D/egl/"
cp fonts/* "$D/fonts/"
cp run.sh STICK-README.txt "$D/"
chmod +x "$D/run.sh" "$D/spike-look" "$D/egl/egl.sh" "$D/egl/egl-probe"
du -sh "$D"
