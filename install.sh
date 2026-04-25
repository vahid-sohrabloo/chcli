#!/bin/sh
# Install chcli — Modern interactive ClickHouse client for the terminal.
#
# Usage:
#   curl -sSL https://raw.githubusercontent.com/vahid-sohrabloo/chcli/main/install.sh | sh
#   curl -sSL https://raw.githubusercontent.com/vahid-sohrabloo/chcli/main/install.sh | VERSION=v0.1.0 sh
#   curl -sSL https://raw.githubusercontent.com/vahid-sohrabloo/chcli/main/install.sh | PREFIX=$HOME/.local sh
#
# Env:
#   VERSION   Tag to install (default: latest)
#   PREFIX    Install prefix — binary goes to $PREFIX/bin (default: /usr/local)
set -eu

REPO="vahid-sohrabloo/chcli"
BIN="chcli"
PREFIX="${PREFIX:-/usr/local}"
VERSION="${VERSION:-latest}"

die() { echo "install: $*" >&2; exit 1; }

# Detect OS
os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
  linux)  os=linux ;;
  darwin) os=darwin ;;
  *) die "unsupported OS: $os (supported: linux, darwin)" ;;
esac

# Detect arch
arch=$(uname -m)
case "$arch" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) die "unsupported arch: $arch (supported: amd64, arm64)" ;;
esac

# Resolve latest version
if [ "$VERSION" = "latest" ]; then
  VERSION=$(curl -sSL -H "Accept: application/json" "https://github.com/$REPO/releases/latest" \
    | sed -E 's/.*"tag_name":"([^"]+)".*/\1/')
  [ -n "$VERSION" ] || die "could not resolve latest version"
fi

url="https://github.com/$REPO/releases/download/$VERSION/${BIN}_${os}_${arch}.tar.gz"
echo "Downloading $url"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
curl -sSLf "$url" -o "$tmp/chcli.tar.gz" || die "download failed"

tar -xzf "$tmp/chcli.tar.gz" -C "$tmp"
[ -f "$tmp/$BIN" ] || die "archive did not contain $BIN"

dest="$PREFIX/bin/$BIN"

# Decide whether to use sudo. We need it when the destination dir either
# doesn't exist and we can't create it, OR exists and isn't writable. If we
# already run as root, sudo is a no-op. If sudo is required but unavailable,
# fall back to a clear error pointing at PREFIX=$HOME/.local.
sudo=""
if [ "$(id -u)" -ne 0 ]; then
  needs_sudo=0
  if [ -d "$PREFIX/bin" ]; then
    [ -w "$PREFIX/bin" ] || needs_sudo=1
  else
    mkdir -p "$PREFIX/bin" 2>/dev/null || needs_sudo=1
  fi
  if [ "$needs_sudo" -eq 1 ]; then
    if command -v sudo >/dev/null 2>&1; then
      echo "Elevating with sudo to write to $PREFIX/bin"
      sudo=sudo
    else
      die "no write access to $PREFIX/bin and sudo not available; re-run with PREFIX=\$HOME/.local"
    fi
  fi
fi

$sudo mkdir -p "$PREFIX/bin"
$sudo mv "$tmp/$BIN" "$dest"
$sudo chmod +x "$dest"
echo "Installed $BIN $VERSION to $dest"
"$dest" --version
