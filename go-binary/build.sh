#!/bin/bash
# Build Go binaries for all supported platforms.
# Output goes to ../binaries/ relative to this script.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

OUT_DIR="../binaries"
mkdir -p "$OUT_DIR"

echo "Building mcp-agent binaries..."

GOOS=linux   GOARCH=amd64 go build -ldflags="-s -w" -o "$OUT_DIR/mcp-agent-linux-amd64"       ./cmd/mcp-agent
echo "  Built linux/amd64"

GOOS=darwin  GOARCH=amd64 go build -ldflags="-s -w" -o "$OUT_DIR/mcp-agent-darwin-amd64"      ./cmd/mcp-agent
echo "  Built darwin/amd64"

GOOS=darwin  GOARCH=arm64 go build -ldflags="-s -w" -o "$OUT_DIR/mcp-agent-darwin-arm64"      ./cmd/mcp-agent
echo "  Built darwin/arm64"

GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o "$OUT_DIR/mcp-agent-windows-amd64.exe" ./cmd/mcp-agent
echo "  Built windows/amd64"

echo "All binaries built in $OUT_DIR"
ls -lh "$OUT_DIR"/mcp-agent-*
