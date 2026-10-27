#!/usr/bin/env bash
# Point the Flutter Android app at clickspirit.aar and minSdk 24.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
GRADLE="$ROOT/app/android/app/build.gradle.kts"
if [[ ! -f "$GRADLE" ]]; then
  echo "skip gradle patch; missing $GRADLE"
  exit 0
fi
if ! grep -q 'clickspirit.aar' "$GRADLE" && ! grep -q 'fileTree' "$GRADLE"; then
  cat >>"$GRADLE" <<'EOF'

dependencies {
    implementation(fileTree(mapOf("dir" to "libs", "include" to listOf("*.aar"))))
}
EOF
fi
if grep -q 'minSdk = flutter.minSdkVersion' "$GRADLE"; then
  tmp="$(mktemp)"
  sed 's/minSdk = flutter.minSdkVersion/minSdk = 24/' "$GRADLE" >"$tmp"
  mv "$tmp" "$GRADLE"
elif grep -q 'minSdkVersion flutter.minSdkVersion' "$GRADLE"; then
  tmp="$(mktemp)"
  sed 's/minSdkVersion flutter.minSdkVersion/minSdkVersion 24/' "$GRADLE" >"$tmp"
  mv "$tmp" "$GRADLE"
fi
echo "patched $GRADLE"
