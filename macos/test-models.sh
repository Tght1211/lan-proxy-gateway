#!/bin/bash
# Foundation-only regression checks; does not require the Xcode XCTest bundle.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")" && pwd)"
TEMP="$(mktemp -d)"
trap 'rm -rf "$TEMP"' EXIT
SDK="${SDKROOT:-$(xcrun --show-sdk-path)}"
SOURCE="$ROOT/Sources/LANProxyGatewayApp"
swiftc -sdk "$SDK" -parse-as-library \
 "$SOURCE/Models.swift" "$SOURCE/ConnectionModels.swift" \
 "$ROOT/Tests/ModelDecodingTests.swift" -o "$TEMP/test-models"
"$TEMP/test-models"
