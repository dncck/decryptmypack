#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DIST_DIR="$ROOT_DIR/dist"

rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR/src" "$DIST_DIR/assets"

cp "$ROOT_DIR/static/index.html" "$DIST_DIR/index.html"
cp "$ROOT_DIR/static/style.css" "$DIST_DIR/style.css"
cp "$ROOT_DIR/static/config.js" "$DIST_DIR/config.js"
cp "$ROOT_DIR/src/script.js" "$DIST_DIR/src/script.js"
cp -R "$ROOT_DIR/assets/." "$DIST_DIR/assets/"
