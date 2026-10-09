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
cp run.sh run-modules.sh STICK-README.txt "$D/"
if [ -f modules/applesmc.ko ]; then mkdir -p "$D/modules" && cp modules/*.ko "$D/modules/"; else echo "no modules/ (run ../spike-modules/build-modules.sh): --modules mode will have nothing to load"; fi
chmod +x "$D/run.sh" "$D/run-modules.sh" "$D/spike-look" "$D/egl/egl.sh" "$D/egl/egl-probe"
du -sh "$D"
