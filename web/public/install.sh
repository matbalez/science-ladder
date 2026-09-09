#!/bin/sh
# Science Ladder CLI installer. MIT. Downloads a versioned binary; never builds source.
set -eu
version=${SL_VERSION:-0.3.0}
case "$version" in *[!0-9.]*|'') echo 'SL_VERSION must be a release number such as 0.3.0' >&2; exit 1;; esac
case "$(uname -s)" in Darwin) platform=darwin;; Linux) platform=linux;; *) echo 'Supported systems: macOS and Linux.' >&2; exit 1;; esac
case "$(uname -m)" in arm64|aarch64) arch=arm64;; x86_64|amd64) arch=amd64;; *) echo 'Supported processors: ARM64 and AMD64.' >&2; exit 1;; esac
command -v curl >/dev/null || { echo 'curl is required.' >&2; exit 1; }
install_dir=${SL_INSTALL_DIR:-"$HOME/.local/bin"}
asset="sl_${platform}_${arch}"
base="https://github.com/matbalez/science-ladder/releases/download/cli-v${version}"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
curl --proto '=https' --tlsv1.2 -fsSL "$base/$asset" -o "$tmp/$asset"
curl --proto '=https' --tlsv1.2 -fsSL "$base/SHA256SUMS" -o "$tmp/SHA256SUMS"
expected=$(awk -v name="$asset" '$2 == name {print $1}' "$tmp/SHA256SUMS")
[ ${#expected} -eq 64 ] || { echo 'Missing or invalid release checksum.' >&2; exit 1; }
if command -v sha256sum >/dev/null; then
 actual=$(sha256sum "$tmp/$asset" | awk '{print $1}')
elif command -v shasum >/dev/null; then
 actual=$(shasum -a 256 "$tmp/$asset" | awk '{print $1}')
else
 echo 'sha256sum or shasum is required.' >&2; exit 1
fi
[ "$actual" = "$expected" ] || { echo 'Checksum mismatch; refusing to install.' >&2; exit 1; }
mkdir -p "$install_dir"
# Copy to a fresh file on the destination filesystem, then replace atomically.
staged=$(mktemp "$install_dir/.sl-install.XXXXXX")
trap 'rm -rf "$tmp"; rm -f "$staged"' EXIT HUP INT TERM
cp "$tmp/$asset" "$staged"
chmod 755 "$staged"
mv -f "$staged" "$install_dir/sl"
"$install_dir/sl" version
printf 'Installed to %s/sl\nAdd it to this shell: export PATH="%s:$PATH"\n' "$install_dir" "$install_dir"
