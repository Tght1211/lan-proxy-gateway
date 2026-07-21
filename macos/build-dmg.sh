#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
MACOS_DIR="$ROOT_DIR/macos"
DIST_DIR="$ROOT_DIR/dist"
VERSION="${VERSION:-$(git -C "$ROOT_DIR" describe --tags --always 2>/dev/null || echo dev)}"
PLIST_VERSION="${VERSION#v}"
BUILD_NUMBER="${BUILD_NUMBER:-$(git -C "$ROOT_DIR" rev-list --count HEAD 2>/dev/null || echo 1)}"
APP_NAME="LAN Proxy Gateway"
APP_BUNDLE="$DIST_DIR/$APP_NAME.app"
SIGN_IDENTITY="${SIGN_IDENTITY:--}"
TEMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/lan-proxy-gateway.XXXXXX")"

cleanup() {
    rm -rf "$TEMP_DIR"
}
trap cleanup EXIT

mkdir -p "$DIST_DIR"
rm -rf "$APP_BUNDLE"

echo "Building universal gateway core..."
GOOS=darwin GOARCH=arm64 go build -ldflags "-s -w -X github.com/tght/lan-proxy-gateway/cmd.Version=$VERSION" -o "$TEMP_DIR/gateway-arm64" "$ROOT_DIR"
GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w -X github.com/tght/lan-proxy-gateway/cmd.Version=$VERSION" -o "$TEMP_DIR/gateway-amd64" "$ROOT_DIR"
lipo -create "$TEMP_DIR/gateway-arm64" "$TEMP_DIR/gateway-amd64" -output "$TEMP_DIR/gateway"

echo "Building universal SwiftUI app..."
if xcodebuild -version >/dev/null 2>&1; then
    APP_ARCH="universal"
    swift build --package-path "$MACOS_DIR" --configuration release --arch arm64 --arch x86_64
    SWIFT_BINARY="$MACOS_DIR/.build/apple/Products/Release/LANProxyGatewayApp"
else
    APP_ARCH="$(uname -m)"
    echo "Full Xcode not found; building the SwiftUI shell for $APP_ARCH."
    swift build --package-path "$MACOS_DIR" --configuration release
    SWIFT_BINARY="$MACOS_DIR/.build/release/LANProxyGatewayApp"
fi
DMG_PATH="$DIST_DIR/LANProxyGateway-$VERSION-macos-$APP_ARCH.dmg"
rm -f "$DMG_PATH"
if [[ ! -x "$SWIFT_BINARY" ]]; then
    echo "Swift build did not produce $SWIFT_BINARY" >&2
    exit 1
fi

echo "Creating app bundle..."
mkdir -p "$APP_BUNDLE/Contents/MacOS" "$APP_BUNDLE/Contents/Resources"
cp "$SWIFT_BINARY" "$APP_BUNDLE/Contents/MacOS/LANProxyGatewayApp"
cp "$TEMP_DIR/gateway" "$APP_BUNDLE/Contents/Resources/gateway"
cp "$MACOS_DIR/Info.plist" "$APP_BUNDLE/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Set :CFBundleShortVersionString $PLIST_VERSION" "$APP_BUNDLE/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Set :CFBundleVersion $BUILD_NUMBER" "$APP_BUNDLE/Contents/Info.plist"

swift "$MACOS_DIR/scripts/generate_icon.swift" "$TEMP_DIR/icon-1024.png"
ICONSET="$TEMP_DIR/AppIcon.iconset"
mkdir -p "$ICONSET"
for spec in "16:icon_16x16.png" "32:icon_16x16@2x.png" "32:icon_32x32.png" "64:icon_32x32@2x.png" "128:icon_128x128.png" "256:icon_128x128@2x.png" "256:icon_256x256.png" "512:icon_256x256@2x.png" "512:icon_512x512.png" "1024:icon_512x512@2x.png"; do
    pixels="${spec%%:*}"
    filename="${spec#*:}"
    sips -z "$pixels" "$pixels" "$TEMP_DIR/icon-1024.png" --out "$ICONSET/$filename" >/dev/null
done
iconutil -c icns "$ICONSET" -o "$APP_BUNDLE/Contents/Resources/AppIcon.icns"

chmod 755 "$APP_BUNDLE/Contents/MacOS/LANProxyGatewayApp" "$APP_BUNDLE/Contents/Resources/gateway"
codesign --force --deep --options runtime --sign "$SIGN_IDENTITY" "$APP_BUNDLE"
codesign --verify --deep --strict "$APP_BUNDLE"

echo "Creating DMG..."
DMG_STAGE="$TEMP_DIR/dmg"
mkdir -p "$DMG_STAGE"
ditto "$APP_BUNDLE" "$DMG_STAGE/$APP_NAME.app"
ln -s /Applications "$DMG_STAGE/Applications"
hdiutil create -volname "$APP_NAME" -srcfolder "$DMG_STAGE" -ov -format UDZO "$DMG_PATH" >/dev/null

echo "Created $DMG_PATH"
