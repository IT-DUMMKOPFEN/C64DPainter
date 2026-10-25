# Builds "C64 DPainter.app" for Apple silicon (arm64) or Intel (x86_64) Macs on any OS.
# Usage: macbundle.py <binary> <icon-png> <out-dir> <version>
import os, shutil, struct, sys
from PIL import Image

arm, icon, out, version = sys.argv[1:5]

def macho_cpu(path):
    d = open(path, 'rb').read(12)
    magic, cpu, sub = struct.unpack('<Iii', d)
    assert magic == 0xFEEDFACF, 'not a 64-bit Mach-O'
    return cpu, sub

app = os.path.join(out, 'C64 DPainter.app')
shutil.rmtree(app, ignore_errors=True)
os.makedirs(os.path.join(app, 'Contents', 'MacOS'))
os.makedirs(os.path.join(app, 'Contents', 'Resources'))
cpu, _ = macho_cpu(arm)
assert cpu in (0x0100000C, 0x01000007), 'the macOS binary must be arm64 (Apple silicon) or x86_64 (Intel)'
exe = os.path.join(app, 'Contents', 'MacOS', 'C64DPainter')
shutil.copyfile(arm, exe)
os.chmod(exe, 0o755)

# icon: pixel art scaled up with a margin like other macOS icons
src = Image.open(icon).convert('RGBA')
canvas = Image.new('RGBA', (1024, 1024), (0, 0, 0, 0))
art = src.resize((832, 832), Image.NEAREST)
canvas.paste(art, (96, 96), art)
canvas.save(os.path.join(app, 'Contents', 'Resources', 'AppIcon.icns'),
            sizes=[(16, 16), (32, 32), (64, 64), (128, 128), (256, 256), (512, 512), (1024, 1024)])

doc_types = [
    ('C64 DPainter project', ['c64dp'], 'Editor'),
    ('Koala Painter picture', ['kla', 'koa'], 'Editor'),
    ('PNG image', ['png'], 'Viewer'),
    ('BMP image', ['bmp'], 'Viewer'),
]
dt = ''
for name, exts, role in doc_types:
    e = ''.join(f'<string>{x}</string>' for x in exts)
    dt += f'''
		<dict>
			<key>CFBundleTypeName</key><string>{name}</string>
			<key>CFBundleTypeRole</key><string>{role}</string>
			<key>CFBundleTypeExtensions</key><array>{e}</array>
			<key>LSHandlerRank</key><string>Alternate</string>
		</dict>'''

plist = f'''<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleDevelopmentRegion</key><string>en</string>
	<key>CFBundleExecutable</key><string>C64DPainter</string>
	<key>CFBundleIconFile</key><string>AppIcon</string>
	<key>CFBundleIdentifier</key><string>org.c64dpainter.app</string>
	<key>CFBundleInfoDictionaryVersion</key><string>6.0</string>
	<key>CFBundleName</key><string>C64 DPainter</string>
	<key>CFBundleDisplayName</key><string>C64 DPainter</string>
	<key>CFBundlePackageType</key><string>APPL</string>
	<key>CFBundleShortVersionString</key><string>{version}</string>
	<key>CFBundleVersion</key><string>{version}</string>
	<key>LSMinimumSystemVersion</key><string>11.0</string>
	<key>LSApplicationCategoryType</key><string>public.app-category.graphics-design</string>
	<key>NSHighResolutionCapable</key><true/>
	<key>NSPrincipalClass</key><string>NSApplication</string>
	<key>NSHumanReadableCopyright</key><string>C64 DPainter</string>
	<key>CFBundleDocumentTypes</key>
	<array>{dt}
	</array>
</dict>
</plist>
'''
open(os.path.join(app, 'Contents', 'Info.plist'), 'w').write(plist)
open(os.path.join(app, 'Contents', 'PkgInfo'), 'w').write('APPL????')
print('ok', app)
