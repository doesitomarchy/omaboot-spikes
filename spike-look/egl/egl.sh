#!/bin/bash
# egl.sh <egl-probe> <out-dir>
# The live ISO has no Mesa. Unpack Mesa and whatever it needs from the ISO's own offline package
# mirror into RAM (/tmp), then run egl-probe on every render node with those libraries, and once
# more on Mesa's software renderer (llvmpipe), which needs no GPU (works under nomodeset too).
# Writes <out-dir>/egl.json (one JSON object per node) and <out-dir>/egl.log. Touches no disk.
set -u
probe=$1 out=$2
R=${GPUROOT:-/tmp/bootlive-gpu}
MIRROR=${MIRROR:-/var/cache/omarchy/mirror/offline}
LOCALDB=${LOCALDB:-/var/lib/pacman/local}  # overridable for a laptop dry run
log() { echo "$*" >>"$out/egl.log"; }
: >"$out/egl.log"; : >"$out/egl.json"

shopt -s nullglob
nodes=(/dev/dri/renderD*)
[ ${#nodes[@]} -gt 0 ] || log "no render nodes (no GPU driver): software renderer only"

# probe [env...]: hardware nodes, then llvmpipe
probe_all() {
	if [ ${#nodes[@]} -gt 0 ]; then
		env "$@" "$probe" "${nodes[@]}" >>"$out/egl.json" 2>>"$out/egl.log"
		log "probe (gpu) exit $?"
	fi
	env "$@" LIBGL_ALWAYS_SOFTWARE=1 "$probe" --software >>"$out/egl.json" 2>>"$out/egl.log"
	log "probe (software) exit $?"
}

if [ -z "${FORCE_UNPACK:-}" ] && pacman -Q mesa >/dev/null 2>&1; then
	log "mesa already installed: $(pacman -Q mesa)"
	probe_all
	exit 0
fi
[ -f "$MIRROR/offline.db" ] || { log "no offline mirror at $MIRROR"; exit 4; }
avail=$(df --output=avail -BM /tmp | tail -1 | tr -dc 0-9)
log "tmp free: ${avail} MiB"
[ "${avail:-0}" -ge 900 ] || { log "not enough RAM in /tmp for Mesa (need 900 MiB)"; exit 5; }

t0=$(date +%s%N)
mkdir -p "$R/db/sync" "$R/root"
[ -e "$R/db/local" ] || ln -s "$LOCALDB" "$R/db/local"
cp "$MIRROR/offline.db" "$R/db/sync/offline.db"  # the repo db is the sync db: no pacman -Sy needed
cat >"$R/pacman.conf" <<EOF
[options]
Architecture = auto
SigLevel = Never
[offline]
Server = file://$MIRROR
EOF
pc=(pacman --config "$R/pacman.conf" --dbpath "$R/db" --noconfirm)
mapfile -t urls < <("${pc[@]}" -Sp --needed --print-format '%l' mesa libglvnd 2>>"$out/egl.log")
[ ${#urls[@]} -gt 0 ] || { log "pacman found nothing to unpack"; exit 7; }
for u in "${urls[@]}"; do
	f=${u#file://}
	log "unpack $(basename "$f")"
	bsdtar -xf "$f" -C "$R/root" --exclude '.PKGINFO' --exclude '.BUILDINFO' --exclude '.MTREE' --exclude '.INSTALL' 2>>"$out/egl.log" || log "  failed"
done
log "unpacked ${#urls[@]} packages, $(du -sm "$R/root" | cut -f1) MiB, in $(( ($(date +%s%N) - t0) / 1000000 )) ms"

L=$R/root/usr/lib
probe_all LD_LIBRARY_PATH="$L" \
	__EGL_VENDOR_LIBRARY_DIRS="$R/root/usr/share/glvnd/egl_vendor.d" \
	LIBGL_DRIVERS_PATH="$L/dri" GBM_BACKENDS_PATH="$L/gbm" \
	EGL_LOG_LEVEL=info
exit 0
