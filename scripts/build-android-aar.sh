#!/usr/bin/env bash
# Bind the mobile package to an Android AAR. Requires gomobile on PATH
# (`go install golang.org/x/mobile/cmd/gomobile@latest && gomobile init`).
set -euo pipefail
cd "$(dirname "$0")/.."
mkdir -p app/android/app/libs
gomobile bind -target=android -androidapi 24 -javapkg com.clickspirit -o app/android/app/libs/clickspirit.aar ./mobile
