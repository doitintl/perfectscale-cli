#!/usr/bin/env bash
# Install the latest pscli CLI release for the current host (macOS / Linux).
# For native Windows shells, use scripts/install.ps1 instead.
#
# Usage:
#   scripts/install.sh                          # install to ~/.local/bin
#   pscli_INSTALL_DIR=/usr/local/bin scripts/install.sh
#   pscli_VERSION=v1.2.3 scripts/install.sh
#
# Env:
#   pscli_REPO         GitHub "owner/repo" (default: perfectscale/poc-cli)
#   pscli_VERSION      Release tag (default: latest)
#   pscli_INSTALL_DIR  Install destination (default: $HOME/.local/bin)

set -euo pipefail

REPO="${pscli_REPO:-perfectscale/poc-cli}"
VERSION="${pscli_VERSION:-latest}"
INSTALL_DIR="${pscli_INSTALL_DIR:-$HOME/.local/bin}"

uname_s="$(uname -s)"
uname_m="$(uname -m)"

case "$uname_s" in
  Darwin)  os="darwin" ;;
  Linux)   os="linux" ;;
  MINGW*|MSYS*|CYGWIN*) os="windows" ;;
  *) echo "unsupported OS: $uname_s" >&2; exit 1 ;;
esac

case "$uname_m" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *) echo "unsupported arch: $uname_m" >&2; exit 1 ;;
esac

# Only the published combinations:
#   darwin/arm64, linux/amd64, linux/arm64, windows/amd64
case "$os/$arch" in
  darwin/arm64|linux/amd64|linux/arm64|windows/amd64) ;;
  *) echo "no published release for $os/$arch" >&2; exit 1 ;;
esac

if [ "$os" = "windows" ]; then
  asset="pscli-${os}-${arch}.zip"
  binary="pscli.exe"
else
  asset="pscli-${os}-${arch}.tar.gz"
  binary="pscli"
fi

if [ "$VERSION" = "latest" ]; then
  base_url="https://github.com/${REPO}/releases/latest/download"
else
  base_url="https://github.com/${REPO}/releases/download/${VERSION}"
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "Downloading ${base_url}/${asset}" >&2
curl -fsSL -o "${tmp}/${asset}" "${base_url}/${asset}"

cd "$tmp"
if [ "${asset##*.}" = "zip" ]; then
  unzip -q "$asset"
else
  tar -xzf "$asset"
fi

mkdir -p "$INSTALL_DIR"
mv "$binary" "$INSTALL_DIR/$binary"
chmod +x "$INSTALL_DIR/$binary"

echo "Installed $INSTALL_DIR/$binary" >&2
"$INSTALL_DIR/$binary" --version || true

case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) echo "Note: $INSTALL_DIR is not on \$PATH. Add it to use 'pscli' directly." >&2 ;;
esac
