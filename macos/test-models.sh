#!/bin/bash
# Native model and updater regression checks; does not require the Xcode XCTest bundle.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")" && pwd)"
TEMP="$(mktemp -d)"
trap 'rm -rf "$TEMP"' EXIT
SDK="${SDKROOT:-$(xcrun --show-sdk-path)}"
SOURCE="$ROOT/Sources/LANProxyGatewayApp"
swiftc -sdk "$SDK" -parse-as-library \
 "$SOURCE/Models.swift" "$SOURCE/ConnectionModels.swift" "$SOURCE/PlayBridgeMetrics.swift" \
 "$ROOT/Tests/ModelDecodingTests.swift" -o "$TEMP/test-models"
"$TEMP/test-models"
swiftc -sdk "$SDK" -parse-as-library \
 "$SOURCE/Models.swift" "$SOURCE/ConnectionModels.swift" "$SOURCE/NetworkPresentation.swift" "$SOURCE/LearningRecordModels.swift" "$SOURCE/DeviceIdentification.swift" "$SOURCE/AppNavigationModels.swift" \
 "$ROOT/Tests/NetworkPresentationTests.swift" -o "$TEMP/test-network"
"$TEMP/test-network"
swiftc -sdk "$SDK" -parse-as-library \
 "$SOURCE/Models.swift" "$SOURCE/ConnectionModels.swift" "$SOURCE/LearningRecordModels.swift" \
 "$ROOT/Tests/LearningRecordTests.swift" -o "$TEMP/test-learning"
"$TEMP/test-learning"
swiftc -sdk "$SDK" -parse-as-library \
 "$SOURCE/Models.swift" "$SOURCE/ConnectionModels.swift" "$SOURCE/ConnectionRoutingModels.swift" \
 "$ROOT/Tests/ConnectionRoutingTests.swift" -o "$TEMP/test-connection-routing"
"$TEMP/test-connection-routing"
swiftc -sdk "$SDK" -parse-as-library \
 "$SOURCE/Models.swift" "$SOURCE/ConnectionModels.swift" "$SOURCE/RoutingEditorModels.swift" \
 "$ROOT/Tests/RoutingEditorTests.swift" -o "$TEMP/test-routing-editor"
"$TEMP/test-routing-editor"
swiftc -sdk "$SDK" -parse-as-library \
 "$SOURCE/Models.swift" "$SOURCE/ConnectionModels.swift" "$SOURCE/ConnectionRoutingModels.swift" \
 "$SOURCE/AppNavigationModels.swift" "$SOURCE/GatewayClient.swift" "$SOURCE/AppUpdateRelease.swift" "$SOURCE/AppUpdater.swift" \
 "$ROOT/Tests/AppUpdaterTests.swift" -o "$TEMP/test-updater"
"$TEMP/test-updater"
