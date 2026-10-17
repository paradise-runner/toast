#!/usr/bin/env bash
# Download (once) the pinned ripgrep release for this platform and copy its
# `rg` binary to the given destination. Used by `make build` to embed ripgrep
# into toast and by scripts/build-libghostty-bundle.sh to ship it.
#
# Usage: scripts/fetch-ripgrep.sh <output-path>
# Version: TOAST_RIPGREP_VERSION overrides the pinned version (default 14.1.1).
# Cache:   the extracted binary is cached under TOAST_BUILD_DIR (default
#          build/libghostty-bundle), so repeat runs do not re-download.
set -euo pipefail

if [[ $# -ne 1 ]]; then
	echo "usage: $0 <output-path>" >&2
	exit 2
fi
out="$1"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
version="${TOAST_RIPGREP_VERSION:-14.1.1}"
build_dir="${TOAST_BUILD_DIR:-${repo_root}/build/libghostty-bundle}"
tools_dir="${build_dir}/tools"

for tool in curl tar; do
	if ! command -v "${tool}" >/dev/null 2>&1; then
		echo "missing required tool: ${tool}" >&2
		exit 1
	fi
done

# Ripgrep target triple, matching the naming of the BurntSushi/ripgrep GitHub
# release assets.
ripgrep_target() {
	local os arch
	case "$(uname -s)" in
		Darwin) os="apple-darwin" ;;
		Linux) os="linux" ;;
		*)
			echo "unsupported OS for ripgrep: $(uname -s)" >&2
			return 1
			;;
	esac
	case "$(uname -m)" in
		arm64 | aarch64) arch="aarch64" ;;
		x86_64 | amd64) arch="x86_64" ;;
		*)
			echo "unsupported architecture for ripgrep: $(uname -m)" >&2
			return 1
			;;
	esac
	# ripgrep publishes x86_64 Linux as musl and aarch64 Linux as gnu.
	case "${os}" in
		apple-darwin) echo "${arch}-apple-darwin" ;;
		linux)
			if [[ "${arch}" == "x86_64" ]]; then
				echo "x86_64-unknown-linux-musl"
			else
				echo "${arch}-unknown-linux-gnu"
			fi
			;;
	esac
}

target="$(ripgrep_target)"
archive="ripgrep-${version}-${target}.tar.gz"
url="https://github.com/BurntSushi/ripgrep/releases/download/${version}/${archive}"
rg_dir="${tools_dir}/ripgrep-${version}-${target}"
rg_bin="${rg_dir}/rg"

if [[ ! -x "${rg_bin}" ]]; then
	echo "downloading ripgrep ${version} (${target})" >&2
	mkdir -p "${rg_dir}"
	curl --fail --location --output "${tools_dir}/${archive}" "${url}"
	tar -C "${rg_dir}" --strip-components=1 -xf "${tools_dir}/${archive}"
	rm -f "${tools_dir}/${archive}"
fi
if [[ ! -x "${rg_bin}" ]]; then
	echo "ripgrep archive did not contain an rg binary: ${archive}" >&2
	exit 1
fi

mkdir -p "$(dirname "${out}")"
cp "${rg_bin}" "${out}"
chmod 755 "${out}"
printf '%s\n' "${out}"
