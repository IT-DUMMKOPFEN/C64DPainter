C64 DPainter – a paint program for C64 bitmap pictures (multicolor, hires and hires free). Created by IT-Dummköpfen · Free for private use. All changes are in `CHANGELOG.txt`.

## Which file?

| File | System |
|---|---|
| `C64DPainter.exe` | Windows 11 (64 bit) |
| `C64DPainter-macOS-Silicon.zip` | macOS 11 or newer, Macs with Apple silicon (M chips) |
| `C64DPainter-macOS-Intel.zip` | macOS 11 or newer, Macs with an Intel processor |
| `c64dpainter-linux-x86_64.tar.gz` | Linux on PCs (64 bit) |
| `c64dpainter-linux-arm64.tar.gz` | Linux on ARM (e.g. Raspberry Pi 4/5, 64-bit system) |

## First start

The program is not signed with a paid developer certificate, so Windows and macOS warn the first time. This is normal for small free programs.

- **Windows:** if "Windows protected your PC" appears, click **More info**, then **Run anyway**. Windows remembers this.
- **macOS:** drag "C64 DPainter.app" into Applications. Open Terminal once and run `xattr -cr "/Applications/C64 DPainter.app"`, then start the app normally. Without Terminal: try to open the app, then go to **System Settings → Privacy & Security**, scroll down and click **Open Anyway**.
- **Linux:** unpack the archive (`tar xzf c64dpainter-linux-*.tar.gz`) and start `./c64dpainter`. `./install.sh` adds a menu entry and an icon.
