#!/usr/bin/env python3
"""Measures how C64 artists dither and writes internal/c64/ditherdata.go.

Input: VICE screenshots (384x272, picture at x=32, y=35) of multicolor
pictures. The pictures themselves are not part of the repository.

    python3 tools/ditheranalysis.py SCREENSHOT_DIR \
        --group scanlines=portrait,diamond,glitch,beavis,bombjack \
        --group hatching=beavis,diner,djcat \
        --group columns=birthday,faces,gameland \
        -o internal/c64/ditherdata.go

A group lists file name parts; its pictures are used to fit that style's
dither matrix. The "artist" matrix and the color pair tables always use all
pictures. Needs numpy, scipy and Pillow.

Method:
 1. Every screenshot is mapped back to C64 color numbers (the palette from
    internal/c64/palettes.txt that matches exactly) and to 160x200
    multicolor pixels (the left pixel of each pair).
 2. All 8x8 areas (step 2) that contain exactly two colors and look like a
    dither (both colors interleaved: at least two separate groups of the
    rarer color, at least 16 color changes, spread over 3+ rows and columns)
    are collected. Their central 4x4 pattern is counted, independent of
    position (all 16 cyclic shifts) and of which color is which.
 3. A 4x4 fill order (dither matrix) is fitted greedily: level by level the
    next pixel is the one whose resulting pattern was seen most often; ties
    go to the pixel farthest away from the others (2:1 pixel aspect).
 4. The color pairs of all dither areas and of all two-colored 4x4 windows
    are counted for the "color ramps".
"""
import argparse, collections, math, os, sys
import numpy as np
from PIL import Image
from scipy import ndimage

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
LUMA = {0: 0, 6: 1, 9: 1, 2: 2, 11: 2, 4: 3, 8: 3, 12: 4, 14: 4, 5: 5, 10: 5, 3: 6, 15: 6, 7: 7, 13: 7, 1: 8}


def palettes():
    pals = {}
    for l in open(os.path.join(ROOT, 'internal/c64/palettes.txt')):
        if ':' in l:
            n, v = l.split(':', 1)
            pals[n.strip()] = [tuple(int(h[i:i + 2], 16) for i in (0, 2, 4)) for h in v.strip().split(',')]
    return pals


def load(path, pals):
    a = np.array(Image.open(path).convert('RGB')).astype(int)
    cols = sorted(set(tuple(int(v) for v in c) for c in a.reshape(-1, 3)))
    best = min(pals.items(), key=lambda kv: sum(min(sum((c[k] - q[k]) ** 2 for k in range(3)) for q in kv[1]) for c in cols))
    p = best[1]
    idx = np.zeros(a.shape[:2], dtype=int)
    for c in cols:
        j = min(range(16), key=lambda j: sum((c[k] - p[j][k]) ** 2 for k in range(3)))
        idx[(a == c).all(-1)] = j
    return idx[35:235, 32:352][:, 0::2]


def canon(bits):
    if bits.sum() > 8:
        bits = 1 - bits
    return min(tuple(np.roll(np.roll(bits, dy, 0), dx, 1).flatten()) for dy in range(4) for dx in range(4))


def collect(pics):
    pats = collections.Counter()
    pairs = collections.Counter()
    pairs4 = collections.Counter()
    for M in pics:
        H, W = M.shape
        for y in range(0, H - 3, 2):
            for x in range(0, W - 3, 2):
                u = np.unique(M[y:y + 4, x:x + 4])
                if len(u) == 2:
                    pairs4[tuple(sorted(u))] += 1
        for y in range(0, H - 7, 2):
            for x in range(0, W - 7, 2):
                w = M[y:y + 8, x:x + 8]
                u = np.unique(w)
                if len(u) != 2:
                    continue
                a, b = sorted(u, key=lambda c: (LUMA[c], c))
                bits = (w == b).astype(int)
                m = bits.sum()
                if m > 32:
                    bits, m = 1 - bits, 64 - m
                if m < 2:
                    continue
                T = (bits[:, 1:] != bits[:, :-1]).sum() + (bits[1:, :] != bits[:-1, :]).sum()
                if T < 16 or ndimage.label(bits)[1] < 2:
                    continue
                if len(np.where(bits.any(1))[0]) < 3 or len(np.where(bits.any(0))[0]) < 3:
                    continue
                pairs[tuple(sorted((a, b)))] += 1
                pats[canon(bits[2:6, 2:6])] += 1
    return pats, pairs, pairs4


def spread(on, p):
    if not on:
        return 0
    py, px = divmod(p, 4)
    best = 99
    for q in on:
        qy, qx = divmod(q, 4)
        dx = min(abs(px - qx), 4 - abs(px - qx)) * 2
        dy = min(abs(py - qy), 4 - abs(py - qy))
        best = min(best, dx * dx + dy * dy)
    return best


def fit(pats):
    on = []
    for n in range(1, 16):
        best = None
        for p in range(16):
            if p in on:
                continue
            b = np.zeros(16, int)
            b[on + [p]] = 1
            key = (pats.get(canon(b.reshape(4, 4)), 0), spread(on, p), -p)
            if best is None or key > best[0]:
                best = (key, p)
        on.append(best[1])
    on.append([p for p in range(16) if p not in on][0])
    m = [0] * 16
    for i, p in enumerate(on):
        m[p] = i
    return m


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('dir')
    ap.add_argument('--group', action='append', default=[])
    ap.add_argument('-o', '--out')
    args = ap.parse_args()
    pals = palettes()
    files = sorted(f for f in os.listdir(args.dir) if f.lower().endswith('.png'))
    pics = {f: load(os.path.join(args.dir, f), pals) for f in files}
    pats, pairs, pairs4 = collect(list(pics.values()))
    print('pictures:', len(pics), ' dither areas:', sum(pairs.values()))
    steps = collections.Counter()
    for (a, b), v in pairs4.items():
        steps[abs(LUMA[a] - LUMA[b])] += v
    tot = sum(steps.values())
    print('brightness steps between mixed colors:', {k: round(v / tot, 3) for k, v in sorted(steps.items())})
    print('artist matrix:', fit(pats))
    for g in args.group:
        name, parts = g.split('=', 1)
        sel = [M for f, M in pics.items() if any(p in f for p in parts.split(','))]
        print(f'{name} matrix ({len(sel)} pictures):', fit(collect(sel)[0]))
    if args.out:
        def tab(cnt, name):
            s = f'var {name} = [16][16]uint16{{\n'
            for a in range(16):
                s += '\t{' + ', '.join(str(cnt.get(tuple(sorted((a, b))), 0) if a != b else 0) for b in range(16)) + '},\n'
            return s + '}\n'
        print('Write the matrices into styleMatrix by hand, the pair tables go to', args.out)
        with open(args.out + '.pairs.txt', 'w') as f:
            f.write(tab(pairs, 'pairCountDither') + '\n' + tab(pairs4, 'pairCountAll'))


if __name__ == '__main__':
    main()
