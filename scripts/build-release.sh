#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

version="$(tr -d '\r\n' < VERSION)"
if [ -z "$version" ]; then
  echo "VERSION is empty" >&2
  exit 1
fi

goos="${GOOS:-$(go env GOOS)}"
goarch="${GOARCH:-$(go env GOARCH)}"
ext=""
if [ "$goos" = "windows" ]; then
  ext=".exe"
fi

release_root="$root/release"
stage="$release_root/goflow-$version-$goos-$goarch"
archive="$release_root/goflow-$version-$goos-$goarch.tar.gz"

rm -rf "$stage"
mkdir -p "$stage" "$release_root"

go_packages=(. ./config ./internal/... ./plugins/examples)
go test "${go_packages[@]}"
go vet "${go_packages[@]}"

(
  cd ui
  npm ci
  npm run build
)

GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$stage/goflow$ext" main.go static_embed.go

cp README.md NODES.md MCP_HTTP.md PLUGINS.md BACKUP.md ROADMAP.md CLI_MCP_ROADMAP.md ROADMAP_PROGRESS.md RELEASE.md CHANGELOG.md COMMERCIAL.md TRADEMARK.md LICENSE VERSION "$stage/"
cp -R plugins "$stage/"
cp -R templates "$stage/"
mkdir -p "$stage/scripts"
cp scripts/mcp-smoke-test.mjs scripts/mcp-http-smoke-test.mjs "$stage/scripts/"

tar -czf "$archive" -C "$release_root" "$(basename "$stage")"
sha256sum "$archive" > "$archive.sha256"

echo "Created $archive"
