#!/usr/bin/env bash
# Sanity check: module builds, vet passes, and the client package compiles.
set -euo pipefail
cd "$(dirname "$0")/.."
go build -o /dev/null .
go vet ./...
echo "Go module build: OK"
