#!/usr/bin/env bash
# Build the c-shared library loaded by the Flutter desktop FFI engine.
#
# Windows (native, or cross from Linux/macOS):
#   CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc \
#     go build -buildmode=c-shared -o app/windows/libs/click_spirit.dll ./ffi
# macOS (on Darwin): arm64 + amd64, then lipo into a universal dylib and
#   install_name_tool -id @rpath/libclick_spirit.dylib
set -euo pipefail
cd "$(dirname "$0")/.."
mkdir -p app/windows/libs app/macos/libs

build_windows() {
  local cc="${CC:-}"
  if [[ -z "$cc" ]]; then
    if [[ "${OS:-}" == "Windows_NT" ]] || [[ "$(uname -s)" == MINGW* ]] || [[ "$(uname -s)" == MSYS* ]]; then
      cc="gcc"
    elif command -v x86_64-w64-mingw32-gcc >/dev/null 2>&1; then
      cc="x86_64-w64-mingw32-gcc"
    elif command -v x86_64-w64-mingw32-gcc-win32 >/dev/null 2>&1; then
      cc="x86_64-w64-mingw32-gcc-win32"
    else
      cc="x86_64-w64-mingw32-gcc"
    fi
  fi
  echo "building Windows DLL with CC=$cc"
  CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC="$cc" \
    go build -buildmode=c-shared -o app/windows/libs/click_spirit.dll ./ffi
}

build_macos() {
  if [[ "$(uname -s)" != "Darwin" ]]; then
    echo "skip macOS dylib: not running on Darwin"
    return 0
  fi
  local tmp
  tmp="$(mktemp -d)"
  CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -buildmode=c-shared -o "$tmp/libclick_spirit_arm64.dylib" ./ffi
  CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -buildmode=c-shared -o "$tmp/libclick_spirit_amd64.dylib" ./ffi
  lipo -create -output app/macos/libs/libclick_spirit.dylib \
    "$tmp/libclick_spirit_arm64.dylib" "$tmp/libclick_spirit_amd64.dylib"
  install_name_tool -id @rpath/libclick_spirit.dylib app/macos/libs/libclick_spirit.dylib
  rm -rf "$tmp"
  echo "wrote app/macos/libs/libclick_spirit.dylib"
}

target="${1:-all}"
case "$target" in
  windows) build_windows ;;
  macos) build_macos ;;
  all)
    build_windows
    build_macos
    ;;
  *)
    echo "usage: $0 [all|windows|macos]" >&2
    exit 2
    ;;
esac
