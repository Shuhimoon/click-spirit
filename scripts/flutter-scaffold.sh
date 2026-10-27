#!/usr/bin/env bash
# Fill in Flutter runner projects without overwriting hand-written files.
# `flutter create` skips a platform directory that already exists, so incomplete
# platform dirs are moved aside, created, then copied back on top.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
APP="$ROOT/app"
cd "$APP"

if ! command -v flutter >/dev/null 2>&1; then
  echo "flutter is not on PATH" >&2
  exit 1
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

move_if_missing() {
  local dir="$1"
  shift
  local ready=0
  local marker
  for marker in "$@"; do
    if [[ -e "$APP/$marker" ]]; then
      ready=1
    fi
  done
  if [[ "$ready" -eq 0 && -d "$APP/$dir" ]]; then
    mv "$APP/$dir" "$tmp/$dir"
  fi
}

move_if_missing android android/settings.gradle.kts android/settings.gradle
move_if_missing windows windows/CMakeLists.txt
move_if_missing macos macos/Runner.xcodeproj

flutter create --platforms=android,windows,macos --org com.clickspirit --project-name click_spirit .

restore() {
  local dir="$1"
  if [[ -d "$tmp/$dir" ]]; then
    mkdir -p "$APP/$dir"
    cp -a "$tmp/$dir/." "$APP/$dir/"
  fi
}
restore android
restore windows
restore macos

bash "$ROOT/scripts/patch-android-gradle.sh"
echo "flutter scaffolding ready"
