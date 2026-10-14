# Creates the program icon (own design, pixel style).
# Background: diagonal C64 rainbow stripes. Foreground: a big, clearly
# outlined paint brush (wooden handle, metal ferrule, bristles with paint).
import io, math, struct, sys, os
from PIL import Image, ImageDraw

N = 64  # design grid; scaled up with NEAREST so it stays pixel art


def quad(p0, p1, w0, w1):
    """Polygon around the segment p0→p1 with width w0 at p0 and w1 at p1."""
    (x0, y0), (x1, y1) = p0, p1
    dx, dy = x1 - x0, y1 - y0
    l = math.hypot(dx, dy)
    nx, ny = -dy / l, dx / l
    return [(x0 + nx * w0 / 2, y0 + ny * w0 / 2), (x1 + nx * w1 / 2, y1 + ny * w1 / 2),
            (x1 - nx * w1 / 2, y1 - ny * w1 / 2), (x0 - nx * w0 / 2, y0 - ny * w0 / 2)]


def lerp(a, b, t):
    return (a[0] + (b[0] - a[0]) * t, a[1] + (b[1] - a[1]) * t)


def art():
    im = Image.new('RGBA', (N, N), (0, 0, 0, 0))
    d = ImageDraw.Draw(im)
    d.rounded_rectangle((0, 0, N - 1, N - 1), radius=10, fill=(0x78, 0x69, 0xC4, 255))
    d.rounded_rectangle((3, 3, N - 4, N - 4), radius=7, fill=(0x35, 0x28, 0x79, 255))

    # C64 rainbow: five stripes rising to the right, drawn with wide
    # multicolor pixels (2×2 in the 64 grid)
    cols = [(0xC4, 0x6C, 0x71), (0xE8, 0x8E, 0x2E), (0xED, 0xF1, 0x71),
            (0x56, 0xAC, 0x4D), (0x70, 0x6D, 0xEB)]
    # the rainbow comes out of the brush tip like a painted trail
    for i, c in enumerate(cols):
        for x in range(10, 60, 2):
            y = 49 - (x - 10) * 0.42 + i * 3
            y = int(round(y))
            d.rectangle((x, y, x + 1, y + 2), fill=c + (255,))

    # brush axis from top right (handle end) to bottom left (bristle tip)
    end = (55.0, 8.0)
    tip = (9.0, 55.0)
    p_ferrule = lerp(end, tip, 0.56)   # handle → ferrule
    p_bristle = lerp(end, tip, 0.74)   # ferrule → bristles
    outline = (0x10, 0x10, 0x18, 255)

    # 1) black outline (everything a bit thicker)
    d.polygon(quad(end, p_ferrule, 8, 10), fill=outline)
    d.polygon(quad(p_ferrule, p_bristle, 12, 12), fill=outline)
    d.polygon(quad(p_bristle, tip, 13, 4), fill=outline)
    d.ellipse((end[0] - 4, end[1] - 4, end[0] + 4, end[1] + 4), fill=outline)
    # 2) wooden handle with highlight
    d.polygon(quad(end, p_ferrule, 5, 7), fill=(0xC8, 0x78, 0x30, 255))
    d.ellipse((end[0] - 2.5, end[1] - 2.5, end[0] + 2.5, end[1] + 2.5), fill=(0xC8, 0x78, 0x30, 255))
    d.line([lerp(end, p_ferrule, 0.05), lerp(end, p_ferrule, 0.9)], fill=(0xF0, 0xB0, 0x60, 255), width=1)
    # 3) metal ferrule (light grey with darker rings)
    d.polygon(quad(p_ferrule, p_bristle, 9, 9), fill=(0xC8, 0xC8, 0xD0, 255))
    for t in (0.3, 0.7):
        a = lerp(p_ferrule, p_bristle, t)
        d.polygon(quad(lerp(p_ferrule, p_bristle, t - 0.06), a, 9, 9), fill=(0x80, 0x80, 0x90, 255))
    d.line([lerp(p_ferrule, p_bristle, 0.05), lerp(p_ferrule, p_bristle, 0.95)], fill=(0xFF, 0xFF, 0xFF, 255), width=1)
    # 4) bristles: dark at the ferrule, wet paint (white/light red) at the tip
    d.polygon(quad(p_bristle, tip, 10, 2), fill=(0x2A, 0x22, 0x20, 255))
    pt = lerp(p_bristle, tip, 0.45)
    d.polygon(quad(pt, tip, 7, 2), fill=(0xF4, 0xF4, 0xF4, 255))
    d.polygon(quad(lerp(p_bristle, tip, 0.7), tip, 4, 1.5), fill=(0xFF, 0x80, 0x88, 255))
    return im


base = art()
out = sys.argv[1]  # path of the .ico
sizes = [16, 24, 32, 48, 64, 128, 256]
imgs = []
for s in sizes:
    if s >= 64:
        imgs.append(base.resize((s, s), Image.NEAREST))
    else:
        imgs.append(base.resize((s, s), Image.LANCZOS))
entries, datas = [], []
for im in imgs:
    b = io.BytesIO(); im.save(b, 'PNG'); datas.append(b.getvalue())
off = 6 + 16 * len(imgs)
hdr = struct.pack('<HHH', 0, 1, len(imgs))
for im, data in zip(imgs, datas):
    w = im.width if im.width < 256 else 0
    hdr += struct.pack('<BBBBHHII', w, w, 0, 0, 1, 32, len(data), off)
    off += len(data)
open(out, 'wb').write(hdr + b''.join(datas))
base.resize((256, 256), Image.NEAREST).save(out.replace('.ico', '.png'))
# small logos for the program itself (menu bar and About window)
if len(sys.argv) > 2:
    d = sys.argv[2]
    base.save(os.path.join(d, 'logo64.png'))
    base.resize((16, 16), Image.LANCZOS).save(os.path.join(d, 'logo16.png'))
print('ok')
