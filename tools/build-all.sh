#!/bin/sh
# Builds and packages C64 DPainter for Windows, macOS and Linux into dist/.
# Works on any OS with Go (no cgo), Python 3 with Pillow, tar and zip.
#
#   tools/build-all.sh            build everything
#   DIST=out tools/build-all.sh   use another output folder
set -eu
cd "$(dirname "$0")/.."
DIST=${DIST:-dist}
WORK="$DIST/.work"
VERSION=$(sed -n 's/.*AppVersion *= *"\([^"]*\)".*/\1/p' internal/app/*.go | head -n 1)
VNUM=$(echo "$VERSION" | awk '{print $1}')
echo "C64 DPainter $VERSION"
rm -rf "$DIST"
mkdir -p "$WORK"
export CGO_ENABLED=0
LD="-s -w"

echo "- Windows (x86-64)"
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-H windowsgui $LD" -o "$DIST/C64DPainter.exe" ./cmd/c64dpainter

for arch in x86_64:amd64 arm64:arm64; do
	name=${arch%%:*}
	goarch=${arch##*:}
	echo "- Linux ($name)"
	pkg="$WORK/c64dpainter-linux-$name"
	mkdir -p "$pkg"
	GOOS=linux GOARCH=$goarch go build -trimpath -ldflags "$LD" -o "$pkg/c64dpainter" ./cmd/c64dpainter
	cp packaging/linux/README.txt packaging/linux/c64dpainter.desktop packaging/linux/install.sh "$pkg/"
	cp res/c64dpainter.png "$pkg/c64dpainter.png"
	chmod 755 "$pkg/c64dpainter" "$pkg/install.sh"
	tar -C "$WORK" -czf "$DIST/c64dpainter-linux-$name.tar.gz" "c64dpainter-linux-$name"
done

for arch in Silicon:arm64 Intel:amd64; do
	name=${arch%%:*}
	goarch=${arch##*:}
	if [ "$name" = Silicon ]; then
		cpu="Apple silicon / M chips"
		other="For Macs with an Intel processor, download C64DPainter-macOS-Intel.zip."
	else
		cpu="Intel processor"
		other="For Macs with Apple silicon (M chips), download C64DPainter-macOS-Silicon.zip."
	fi
	echo "- macOS ($name)"
	GOOS=darwin GOARCH=$goarch go build -trimpath -ldflags "$LD" -o "$WORK/C64DPainter_$goarch" ./cmd/c64dpainter
	python3 tools/macbundle.py "$WORK/C64DPainter_$goarch" res/c64dpainter.png "$WORK/mac-$name" "$VNUM" >/dev/null
	sed -e "s|{CPU}|$cpu|" -e "s|{OTHER}|$other|" "packaging/macos/How to open.txt" >"$WORK/mac-$name/How to open.txt"
	(cd "$WORK/mac-$name" && zip -qry "../../C64DPainter-macOS-$name.zip" .)
done

rm -rf "$WORK"
echo "Done:"
ls -l "$DIST"
