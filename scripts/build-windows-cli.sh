#!/usr/bin/env bash
# Cross-compile the Windows CLI. Run from anywhere.
set -euo pipefail
cd "$(dirname "$0")/.."
mkdir -p dist
GOOS=windows GOARCH=amd64 go build -o dist/clicker.exe ./cmd/clicker
