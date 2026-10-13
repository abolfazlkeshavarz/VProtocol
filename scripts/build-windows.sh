#!/usr/bin/env bash
# Builds the Windows VProtocol binary and places the Wintun driver DLL next
# to it (wintun.dll is a runtime dependency of the TUN device code and
# can't be embedded in the Go binary itself — wails build -clean wipes
# build/bin, so this must run after every build).
set -euo pipefail

cd "$(dirname "$0")/../app"

wails build -clean "$@"

cp resources/windows/wintun/amd64/wintun.dll build/bin/wintun.dll

echo "Done: build/bin/vprotocol.exe (+ wintun.dll)"
