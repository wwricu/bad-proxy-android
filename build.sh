#!/bin/sh
# Builds the APK: Go engine -> AAR -> Gradle.
#
# Prerequisites:
#   * Go, the Android NDK and `gomobile` (go install golang.org/x/mobile/cmd/gomobile)
#   * bad-proxy-core checked out next to this repo (go.mod replaces it with
#     ../bad-proxy-core, so the engine and the core stay in lockstep)
#   * ANDROID_NDK_HOME and ANDROID_HOME set
set -eu
cd "$(dirname "$0")"

API=24
ASSETS=android/app/src/main/assets
RULES=$ASSETS/rules.dat

gomobile bind -target=android/arm64 -androidapi "$API" \
  -o android/app/libs/vpn.aar ./vpn

# The routing dataset is not in git; the app unpacks it from assets on first
# run, so a missing file means split routing silently degrades to "all direct".
if [ ! -s "$RULES" ]; then
  mkdir -p "$ASSETS"
  curl -fsSL -o "$RULES" \
    https://github.com/wwricu/domain-list-community/releases/latest/download/rules.dat
fi

(cd android && ./gradlew assembleDebug)
echo "apk: android/app/build/outputs/apk/debug/app-debug.apk"
