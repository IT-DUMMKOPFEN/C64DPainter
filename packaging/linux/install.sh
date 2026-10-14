#!/bin/sh
# Installs C64 DPainter for the current user (no root needed).
set -e
cd "$(dirname "$0")"
mkdir -p "$HOME/.local/bin" "$HOME/.local/share/applications" "$HOME/.local/share/icons/hicolor/256x256/apps"
install -m 755 c64dpainter "$HOME/.local/bin/c64dpainter"
install -m 644 c64dpainter.png "$HOME/.local/share/icons/hicolor/256x256/apps/c64dpainter.png"
sed "s|^Exec=c64dpainter|Exec=$HOME/.local/bin/c64dpainter|" c64dpainter.desktop > "$HOME/.local/share/applications/c64dpainter.desktop"
command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database "$HOME/.local/share/applications" || true
echo "C64 DPainter installed. Start it from your application menu or with: ~/.local/bin/c64dpainter"
