#!/bin/sh
# Downloads the dotenvx CLI the benchmark compares against into bin/, and
# checks it against the release's checksums.
#
#   sh install-dotenvx.sh [version]
set -eu
version=${1:-1.75.1}
case "$(uname -m)" in
  x86_64 | amd64) arch=x86_64 ;;
  aarch64 | arm64) arch=aarch64 ;;
  *) echo "unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac
cd "$(dirname "$0")"
archive="dotenvx-$version-linux-$arch.tar.gz"
base="https://github.com/dotenvx/dotenvx/releases/download/v$version"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
curl -fsSL -o "$tmp/$archive" "$base/$archive"
curl -fsSL -o "$tmp/checksums.txt" "$base/checksums.txt"
(cd "$tmp" && grep " $archive\$" checksums.txt | sha256sum -c -)
mkdir -p bin
tar -xzf "$tmp/$archive" -C bin dotenvx 2>/dev/null || tar -xzf "$tmp/$archive" -C bin ./dotenvx
bin/dotenvx --version
