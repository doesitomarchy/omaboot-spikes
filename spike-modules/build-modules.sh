#!/bin/bash
# Build patched applesmc.ko and radeon.ko for the kernel of the Omarchy 4.0.4 ISO
# (linux-t2 7.2.4.arch1-3, uname 7.2.4-arch1-Watanare-T2-3-t2), for spike-look --modules.
#
#   ./build-modules.sh omarchy-4.0.4.iso WORKDIR
#
# Headers come from the ISO's own offline package mirror. Sources: Arch's v7.2.4-arch1 tag
# (radeon; t2linux doesn't patch it) and the same applesmc plus t2linux patches 3001-3009 as of
# the package's build (2026-09-15). Before patching, each module is rebuilt stock and checked
# against the one the ISO ships. Output: ../spike-look/modules/{applesmc,radeon}.ko
# Needs: bsdtar, unsquashfs (squashfs-tools), git, gcc, make, patch, modinfo, objcopy.
set -euo pipefail
ISO=$(readlink -f "$1")
W=$(readlink -f "${2:?usage: build-modules.sh ISO WORKDIR}")
HERE=$(cd "$(dirname "$0")" && pwd)
OUT=$HERE/../spike-look/modules
KVER=7.2.4-arch1-Watanare-T2-3-t2
PKG=linux-t2-7.2.4.arch1-3-x86_64.pkg.tar.zst
HPKG=linux-t2-headers-7.2.4.arch1-3-x86_64.pkg.tar.zst
ARCH_TAG=v7.2.4-arch1
T2_COMMIT=429917fba3      # t2linux/linux-t2-patches, last commit before the package build
IMACFIX_COMMIT=0d5a406a31 # choyer/omarchy-vintage-intel-imac-fix

mkdir -p "$W" "$OUT"
cd "$W"

echo "== packages from the ISO's offline mirror"
if [ ! -d sq ]; then
	bsdtar -xf "$ISO" arch/x86_64/airootfs.sfs
	unsquashfs -q -n -d sq arch/x86_64/airootfs.sfs "var/cache/omarchy/mirror/offline/$PKG" "var/cache/omarchy/mirror/offline/$HPKG"
	rm arch/x86_64/airootfs.sfs
fi
M=sq/var/cache/omarchy/mirror/offline
mkdir -p headers shipped
[ -d "headers/usr/lib/modules/$KVER/build" ] || tar --zstd -xf "$M/$HPKG" -C headers
H=$W/headers/usr/lib/modules/$KVER/build
tar --zstd -xf "$M/$PKG" -C shipped "usr/lib/modules/$KVER/kernel/drivers/hwmon/applesmc.ko.zst" "usr/lib/modules/$KVER/kernel/drivers/gpu/drm/radeon/radeon.ko.zst"
find shipped -name '*.zst' -exec zstd -q -d -f {} \;
SH=$W/shipped/usr/lib/modules/$KVER/kernel/drivers

echo "== sources"
[ -d ksrc ] || git clone -q --depth 1 --filter=blob:none --sparse --branch "$ARCH_TAG" https://github.com/archlinux/linux.git ksrc
git -C ksrc sparse-checkout set drivers/gpu/drm/radeon drivers/hwmon
[ -d t2p ] || git clone -q https://github.com/t2linux/linux-t2-patches.git t2p
git -C t2p checkout -q "$T2_COMMIT"
[ -d imacfix ] || git clone -q https://github.com/choyer/omarchy-vintage-intel-imac-fix.git imacfix
git -C imacfix checkout -q "$IMACFIX_COMMIT"

build() { make -s -j"$(nproc)" -C "$H" M="$1" modules 2>&1 | grep -v -i 'skipping BTF' || true; }

echo "== applesmc"
rm -rf applesmc && mkdir applesmc && cp ksrc/drivers/hwmon/applesmc.c applesmc/
(cd applesmc && for p in "$W"/t2p/300*.patch; do patch -s -p3 <"$p"; done && echo 'obj-m := applesmc.o' >Kbuild)
build "$W/applesmc"
[ "$(modinfo -F srcversion applesmc/applesmc.ko)" = "$(modinfo -F srcversion "$SH/hwmon/applesmc.ko")" ] || { echo "stock applesmc differs from the shipped one"; exit 1; }
echo "   stock rebuild matches the shipped module (srcversion)"
(cd applesmc && rm -f ./*.o ./*.ko ./*.mod* .*.cmd Module.symvers modules.order && patch -s -p3 <"$HERE/patches/0001-applesmc-led-container_of.patch")
build "$W/applesmc"
objcopy --strip-debug applesmc/applesmc.ko "$OUT/applesmc.ko"

echo "== radeon"
# radeon_trace.h includes itself by a path relative to the kernel tree
cp ksrc/drivers/gpu/drm/radeon/radeon_trace.h "$H/drivers/gpu/drm/radeon/"
rm -rf radeon && cp -r ksrc/drivers/gpu/drm/radeon radeon
build "$W/radeon"
cmp -s <(objcopy -O binary --only-section=.text radeon/radeon.ko /dev/stdout) <(objcopy -O binary --only-section=.text "$SH/gpu/drm/radeon/radeon.ko" /dev/stdout) ||
	{ echo "stock radeon .text differs from the shipped one"; exit 1; }
echo "   stock rebuild matches the shipped module (.text byte for byte)"
rm -rf radeon && cp -r ksrc/drivers/gpu/drm/radeon radeon
(cd radeon && patch -s -p5 <"$W/imacfix/patches/panel-encoder-combined.patch")
build "$W/radeon"
objcopy --strip-debug radeon/radeon.ko "$OUT/radeon.ko"

for m in applesmc radeon; do
	echo "$m.ko: $(modinfo -F vermagic "$OUT/$m.ko")  $(du -h "$OUT/$m.ko" | cut -f1)"
done
