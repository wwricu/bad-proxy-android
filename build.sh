#!/bin/sh
# Builds the APK: Go engine -> AAR -> Gradle.
#
# Prerequisites:
#   * Go, the Android NDK and `gomobile` (go install golang.org/x/mobile/cmd/gomobile)
#   * bad-proxy-core checked out next to this repo (go.mod replaces it with
#     ../bad-proxy-core, so the engine and the core stay in lockstep)
#   * ANDROID_NDK_HOME and ANDROID_HOME set
#
# Usage: ./build.sh [debug|release]   (default: debug)
set -eu
cd "$(dirname "$0")"

BUILD=${1:-debug}
case "$BUILD" in
  debug)   TASK=assembleDebug ;;
  release) TASK=assembleRelease ;;
  *) echo "usage: $0 [debug|release]" >&2; exit 2 ;;
esac

API=24
ASSETS=android/app/src/main/assets
RULES=$ASSETS/rules.dat

# arm64 for devices, amd64 (x86_64) for emulators on dev hosts.
gomobile bind -target=android/arm64,android/amd64 -androidapi "$API" \
  -o android/app/libs/vpn.aar ./vpn

# The routing dataset is not in git; the app unpacks it from assets on first
# run, so a missing file means split routing silently degrades to "all direct".
if [ ! -s "$RULES" ]; then
  mkdir -p "$ASSETS"
  curl -fsSL -o "$RULES" \
    https://github.com/wwricu/domain-list-community/releases/latest/download/rules.dat
fi

(cd android && ./gradlew "$TASK")
echo "apk: android/app/build/outputs/apk/$BUILD/app-$BUILD.apk"
