"""Build our own 512-glyph Terminus console fonts (omaboot-v16n/24n/32n.psf.gz) for the spike.

Terminus 'v' (512 glyphs) + from the CP437 'u' set: the half-blocks, the shade block, double box
lines, a circle and two arrows; plus our hand-drawn tick (U+2713) and filled triangle (U+25B2),
the same bitmaps as the OmaBoot? Live prototype. To stay at the console's 512-glyph limit it
drops the non-Russian Cyrillic letters from U+0492 up (Kazakh, Tatar, Bashkir, Mongolian...).

Usage: python3 -I mkpsf.py <terminus consolefonts dir> <out dir>
"""
import gzip, os, struct, sys

ADD_FROM_U = "▀▄▌▐▓═║╒╔╗╘╙╚╛╝╞╟╠╡╣╦╧╨╩╪╬○↕↔"
DROP_FROM = 0x0492  # Cyrillic-only glyphs at or above this go (keeps Ukrainian Ґґ, Serbian, Belarusian)


def parse(path):
    b = gzip.open(path).read()
    if b[:2] == b'\x36\x04':  # PSF1 (the 8-wide 16 px size): UCS-2 table, 0xFFFF ends a glyph
        mode, h = b[2], b[3]
        n = 512 if mode & 1 else 256
        glyphs = [b[4 + i * h: 4 + (i + 1) * h] for i in range(n)]
        cps, p = [], 4 + n * h
        for i in range(n):
            c, seq = [], True
            while True:
                u = struct.unpack('<H', b[p:p + 2])[0]; p += 2
                if u == 0xFFFF:
                    break
                if u == 0xFFFE:
                    seq = False
                elif seq:
                    c.append(u)
            cps.append(c)
        return 8, h, glyphs, cps
    magic, ver, hsize, flags, n, gsize, h, w = struct.unpack('<IIIIIIII', b[:32])
    assert magic == 0x864ab572 and flags & 1, path
    glyphs = [b[hsize + i * gsize: hsize + (i + 1) * gsize] for i in range(n)]
    cps = []  # per glyph: the code points it serves (sequences skipped)
    p = hsize + n * gsize
    for i in range(n):
        end = b.index(b'\xff', p)
        cps.append([ord(c) for c in b[p:end].split(b'\xfe')[0].decode('utf-8')])
        p = end + 1
    return w, h, glyphs, cps


def rows_to_bitmap(rows, w):
    bpr = (w + 7) // 8
    out = bytearray()
    for r in rows:
        out += (int(r.replace('#', '1').replace('.', '0'), 2) << (bpr * 8 - w)).to_bytes(bpr, 'big')
    return bytes(out)


def tri(w, h, top, bot):
    rows = []
    for r in range(h):
        if top <= r <= bot:
            half = round((r - top) / (bot - top) * ((w - 2) / 2))
            mid = (w - 1) / 2
            rows.append(''.join('#' if abs(c - mid) <= half + 0.01 else '.' for c in range(w)))
        else:
            rows.append('.' * w)
    return rows


TICK = {
    16: ["........", "........", "........", "........", "......#.", "......#.", ".....#..", ".#...#..", ".#..#...", "..#.#...", "...#....", "........", "........", "........", "........", "........"],
    24: ["............"] * 7 + [".........#..", "........#...", "........#...", ".......#....", ".......#....", ".#....#.....", ".#....#.....", "..#..#......", "...#.#......", "....#......."] + ["............"] * 7,
    32: ["................"] * 10 + ["............##..", "............##..", "...........##...", "...........##...", "..........##....", "..........##....", ".........##.....", "..##.....##.....", "..##.....##.....", "...##...##......", "....##..##......", ".....####.......", "......##........"] + ["................"] * 9,
}
TRI = {16: tri(8, 16, 4, 11), 24: tri(12, 24, 6, 17), 32: tri(16, 32, 9, 24)}


def build(src, size):
    w, h, glyphs, cps = parse(f'{src}/ter-v{size}n.psf.gz')
    _, _, ug, ucps = parse(f'{src}/ter-u{size}n.psf.gz')
    # drop rarely needed Cyrillic
    keep = [i for i, c in enumerate(cps) if not (c and all(DROP_FROM <= x < 0x0530 for x in c))]
    glyphs = [glyphs[i] for i in keep]
    cps = [list(cps[i]) for i in keep]
    have = {x for c in cps for x in c}
    for ch in ADD_FROM_U:
        cp = ord(ch)
        if cp in have:
            continue
        gi = next((i for i, c in enumerate(ucps) if cp in c), None)
        if gi is None:
            print(f'  ter-u{size}n has no {ch}, skipped')
            continue
        glyphs.append(ug[gi]); cps.append([cp]); have.add(cp)
    for cp, rows in ((0x2713, TICK[size]), (0x25B2, TRI[size])):
        assert len(rows) == h and all(len(r) == w for r in rows), (size, cp)
        for c in cps:  # take the code point off any glyph that had it (Terminus's ▲ is an arrow)
            if cp in c:
                c.remove(cp)
        glyphs.append(rows_to_bitmap(rows, w)); cps.append([cp])
    assert len(glyphs) <= 512, (size, len(glyphs))
    while len(glyphs) < 512:  # pad: the kernel wants 256 or 512 glyphs for the 9th-bit mode
        glyphs.append(bytes(len(glyphs[0]))); cps.append([])
    gsize = len(glyphs[0])
    out = struct.pack('<IIIIIIII', 0x864ab572, 0, 32, 1, len(glyphs), gsize, h, w)
    out += b''.join(glyphs)
    for c in cps:
        out += ''.join(chr(x) for x in c).encode('utf-8') + b'\xff'
    return out, len(keep), len(glyphs)


if __name__ == '__main__':
    src, dst = sys.argv[1], sys.argv[2]
    os.makedirs(dst, exist_ok=True)
    for size in (16, 24, 32):
        data, kept, n = build(src, size)
        with gzip.GzipFile(f'{dst}/omaboot-v{size}n.psf.gz', 'wb', mtime=0) as f:
            f.write(data)
        print(f'omaboot-v{size}n: {kept} Terminus v glyphs kept, {n} total')
