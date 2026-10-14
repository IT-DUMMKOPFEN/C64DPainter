# Rasterisiert DejaVu-Schriften zu Alpha-Glyphen (eigenes Binaerformat).
import struct, sys
from PIL import Image, ImageDraw, ImageFont

CHARS = list(range(32, 127)) + list(range(160, 256)) + [
    0x20AC, 0x2013, 0x2014, 0x201E, 0x201C, 0x201D, 0x201A, 0x2018, 0x2019,
    0x2026, 0x2190, 0x2191, 0x2192, 0x2193, 0x2022, 0x21BB, 0x21BA, 0x2713,
    0x25B8, 0x25BE, 0x25C2, 0x25B4, 0x2194, 0x2195, 0x2715, 0x2026, 0x2302, 0x2212, 0x2260, 0x2264, 0x2265]

def gen(path, size, out):
    f = ImageFont.truetype(path, size)
    asc, desc = f.getmetrics()
    glyphs = []
    seen = set()
    for c in CHARS:
        if c in seen: continue
        seen.add(c)
        ch = chr(c)
        adv = int(round(f.getlength(ch)))
        W, H = size * 3, size * 3
        im = Image.new('L', (W, H), 0)
        d = ImageDraw.Draw(im)
        ox, oy = size, size + asc
        d.text((ox, oy), ch, font=f, fill=255, anchor='ls')
        bb = im.getbbox()
        if bb is None:
            glyphs.append((c, adv, 0, 0, 0, 0, b''))
            continue
        x0, y0, x1, y1 = bb
        crop = im.crop(bb)
        glyphs.append((c, adv, x0 - ox, y0 - oy, x1 - x0, y1 - y0, crop.tobytes()))
    with open(out, 'wb') as fp:
        fp.write(struct.pack('<4sHHHH', b'GFNT', asc, desc, asc + desc, len(glyphs)))
        for c, adv, bx, by, w, h, data in glyphs:
            fp.write(struct.pack('<IhhhBB', c, adv, bx, by, w, h))
            fp.write(data)

D = '/usr/share/fonts/truetype/dejavu/'
o = sys.argv[1]
gen(D + 'DejaVuSans.ttf', 12, o + '/ui.bin')
gen(D + 'DejaVuSans-Bold.ttf', 12, o + '/uib.bin')
gen(D + 'DejaVuSans.ttf', 13, o + '/body.bin')
gen(D + 'DejaVuSans-Bold.ttf', 13, o + '/bodyb.bin')
gen(D + 'DejaVuSans-Bold.ttf', 15, o + '/h2.bin')
gen(D + 'DejaVuSans-Bold.ttf', 19, o + '/h1.bin')
gen(D + 'DejaVuSansMono.ttf', 12, o + '/mono.bin')
gen(D + 'DejaVuSansMono-Bold.ttf', 11, o + '/monob.bin')
print('ok')
