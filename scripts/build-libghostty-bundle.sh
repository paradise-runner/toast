#!/usr/bin/env bash
# Build a standalone toast app: the TUI editor bundled with a libghostty-based
# terminal window (Ghostling). Modeled on Terminal Empire's libghostty bundle.
#
# Output: dist/toast-libghostty — a single self-extracting executable. On
# first run it unpacks the terminal, the editor, and libghostty-vt to a temp
# directory, then opens a Ghostling window running toast. On macOS it also
# assembles dist/Toast.app, a proper application bundle wrapping the launcher
# (dock icon, name, Info.plist).
#
# Version: TOAST_VERSION (e.g. "v0.8.0") overrides the version baked into the
# launcher and Info.plist; defaults to cmd/toast/main.go's version var.
#
# Requirements: go, git, cmake, ninja, curl, tar. Zig is provisioned
# automatically (0.15.2) unless ZIG points at a working zig binary.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
build_dir="${TOAST_BUILD_DIR:-${repo_root}/build/libghostty-bundle}"
dist_dir="${TOAST_DIST_DIR:-${repo_root}/dist}"
payload_dir="${repo_root}/cmd/toastapp/payload"
ghostling_src="${build_dir}/ghostling-src"
ghostling_build="${build_dir}/ghostling-build"
toast_payload="${build_dir}/toast"
ghostling_payload="${ghostling_build}/ghostling"
ghostty_dylib="${ghostling_build}/_deps/ghostty-src/zig-out/lib/libghostty-vt.dylib"
bundle_path="${dist_dir}/toast-libghostty"
ghostling_esc_patch="${repo_root}/patches/ghostling-disable-raylib-esc-exit.patch"
ghostling_glyph_patch="${repo_root}/patches/ghostling-load-terminal-glyphs.patch"
ghostling_identity_patch="${repo_root}/patches/ghostling-toast-app-identity.patch"
ghostling_menu_patch="${repo_root}/patches/ghostling-native-menu.patch"
logo_src="${repo_root}/toast-logo.png"

# Ghostling revision the patches are tested against. Override with GHOSTLING_REF.
ghostling_ref="${GHOSTLING_REF:-32c2dd5}"
required_zig_version="${TOAST_ZIG_VERSION:-0.15.2}"
tools_dir="${build_dir}/tools"

for tool in go git cmake ninja curl tar; do
	if ! command -v "${tool}" >/dev/null 2>&1; then
		echo "missing required tool: ${tool}" >&2
		exit 1
	fi
done

mkdir -p "${build_dir}" "${dist_dir}" "${payload_dir}" "${tools_dir}"

zig_platform() {
	local os arch
	case "$(uname -s)" in
		Darwin) os="macos" ;;
		Linux) os="linux" ;;
		*)
			echo "unsupported OS for automatic Zig provisioning: $(uname -s)" >&2
			return 1
			;;
	esac
	case "$(uname -m)" in
		arm64 | aarch64) arch="aarch64" ;;
		x86_64 | amd64) arch="x86_64" ;;
		*)
			echo "unsupported architecture for automatic Zig provisioning: $(uname -m)" >&2
			return 1
			;;
	esac
	echo "${arch}-${os}"
}

zig_version() {
	"$1" version 2>/dev/null || true
}

ensure_zig() {
	local zig_bin="${ZIG:-}"
	if [[ -z "${zig_bin}" ]] && command -v zig >/dev/null 2>&1; then
		zig_bin="$(command -v zig)"
	fi
	if [[ -n "${zig_bin}" && "$(zig_version "${zig_bin}")" == "${required_zig_version}" ]]; then
		echo "using Zig ${required_zig_version}: ${zig_bin}" >&2
		printf '%s\n' "${zig_bin}"
		return 0
	fi

	local platform archive url extract_dir
	platform="$(zig_platform)"
	archive="zig-${platform}-${required_zig_version}.tar.xz"
	url="https://ziglang.org/download/${required_zig_version}/${archive}"
	extract_dir="${tools_dir}/zig-${platform}-${required_zig_version}"
	zig_bin="${extract_dir}/zig"

	if [[ ! -x "${zig_bin}" ]]; then
		echo "downloading Zig ${required_zig_version} for Ghostty" >&2
		curl --fail --location --output "${tools_dir}/${archive}" "${url}"
		tar -C "${tools_dir}" -xf "${tools_dir}/${archive}"
	fi
	if [[ "$(zig_version "${zig_bin}")" != "${required_zig_version}" ]]; then
		echo "downloaded Zig binary does not report ${required_zig_version}: ${zig_bin}" >&2
		exit 1
	fi
	echo "using Zig ${required_zig_version}: ${zig_bin}" >&2
	printf '%s\n' "${zig_bin}"
}

zig_bin="$(ensure_zig | tail -n 1)"
export PATH="$(dirname "${zig_bin}"):${PATH}"

# zig 0.15.x cannot link against macOS SDKs newer than ~26.2 (its tbd parser
# chokes on libSystem.tbd from e.g. Xcode 26.5). Probe with a trivial link;
# on failure, retry using the CommandLineTools SDK, which ships an older
# MacOSX.sdk (26.2) that zig handles fine.
# zig 0.15.x cannot link against macOS SDKs newer than ~26.2 (its tbd parser
# chokes on libSystem.tbd from e.g. Xcode 26.5). GitHub macOS runners ship
# several SDKs side by side (e.g. MacOSX15.4.sdk next to the default 26.5),
# so when the probe fails we find the newest SDK older than macOS 26 and put
# an `xcrun` shim on PATH: zig resolves the SDK via `xcrun --sdk macosx
# --show-sdk-path` (looked up on PATH), so the shim answers that one
# invocation with the older SDK and delegates everything else.
older_sdk_shim() {
	local sdk_root="$1" sdk name major newest=""
	for sdk in $(ls -1d "${sdk_root}"/MacOSX*.sdk 2>/dev/null | sort -V); do
		name="$(basename "${sdk}")"
		major="${name#MacOSX}"
		major="${major%%.*}"
		[[ "${major}" =~ ^[0-9]+$ ]] || continue
		[[ "${major}" -lt 26 ]] || continue
		newest="${sdk}"
	done
	[[ -z "${newest}" ]] && return 1
	mkdir -p "${tools_dir}/bin"
	cat > "${tools_dir}/bin/xcrun" <<EOF
#!/bin/bash
# Shim so zig 0.15.x resolves an older, parseable macOS SDK (its tbd parser
# cannot read libSystem.tbd from SDKs newer than ~26.2). Only intercepts the
# exact invocation zig's SDK detection makes; everything else delegates.
if [[ "\${1:-}" == "--sdk" && "\${2:-}" == "macosx" && "\${3:-}" == "--show-sdk-path" ]]; then
	echo "${newest}"
	exit 0
fi
exec /usr/bin/xcrun "\$@"
EOF
	chmod +x "${tools_dir}/bin/xcrun"
	printf '%s\n' "${tools_dir}/bin"
}

probe_zig_link() {
	local probe_dir
	probe_dir="$(mktemp -d)"
	printf 'const std = @import("std");\npub fn main() void { std.debug.print("ok\\n", .{}); }\n' > "${probe_dir}/probe.zig"
	if ( cd "${probe_dir}" && "$1" build-exe probe.zig >/dev/null 2>&1 ); then
		rm -rf "${probe_dir}"
		return 0
	fi
	rm -rf "${probe_dir}"
	return 1
}

if ! probe_zig_link "${zig_bin}"; then
	echo "zig link probe failed against the default macOS SDK" >&2
	if [[ -d /Library/Developer/CommandLineTools/SDKs ]]; then
		echo "retrying with the CommandLineTools SDK (DEVELOPER_DIR)" >&2
		export DEVELOPER_DIR=/Library/Developer/CommandLineTools
		if ! probe_zig_link "${zig_bin}"; then
			echo "zig link probe failed even with the CommandLineTools SDK; searching for an older macOS SDK" >&2
			if shim_dir="$(older_sdk_shim /Library/Developer/CommandLineTools/SDKs)"; then
				echo "retrying with an older macOS SDK via xcrun shim: ${shim_dir}" >&2
				export PATH="${shim_dir}:${PATH}"
				if probe_zig_link "${zig_bin}"; then
					echo "zig link probe passed with an older macOS SDK" >&2
				else
					echo "no usable macOS SDK found; install an older SDK or set ZIG to a working binary" >&2
					exit 1
				fi
			else
				echo "no older macOS SDK found; install an older SDK or set ZIG to a working binary" >&2
				exit 1
			fi
		fi
	else
		echo "no CommandLineTools SDK found; install one (or set ZIG to a working zig 0.15.2) and rerun" >&2
		exit 1
	fi
fi

echo "building toast payload"
(
	cd "${repo_root}"
	go build -o "${toast_payload}" ./cmd/toast
)

# The ghostling repo is tiny, so fetch full history: this lets the build
# pin a specific commit SHA (the default) rather than tracking a moving branch.
if [[ ! -d "${ghostling_src}/.git" ]]; then
	echo "cloning Ghostling"
	git clone https://github.com/ghostty-org/ghostling.git "${ghostling_src}"
else
	echo "updating Ghostling"
	git -C "${ghostling_src}" fetch --all --tags
fi

echo "checking out Ghostling ${ghostling_ref}"
git -C "${ghostling_src}" checkout --detach --force "${ghostling_ref}"
# Remove leftovers from previous builds (patch-added files such as menu.m
# are untracked, so checkout --force does not clean them and git apply would
# fail on re-runs).
git -C "${ghostling_src}" clean -fd

echo "patching Ghostling Raylib Esc behavior"
git -C "${ghostling_src}" apply "${ghostling_esc_patch}"

echo "patching Ghostling terminal glyph atlas"
git -C "${ghostling_src}" apply "${ghostling_glyph_patch}"

echo "patching Ghostling toast app identity (title + logo icon)"
git -C "${ghostling_src}" apply "${ghostling_identity_patch}"

echo "patching Ghostling native menu bar (File/Edit/View)"
git -C "${ghostling_src}" apply "${ghostling_menu_patch}"

# The identity patch embeds assets/toast-logo.png via bin2header; provide it.
mkdir -p "${ghostling_src}/assets"
cp "${logo_src}" "${ghostling_src}/assets/toast-logo.png"

# Upstream bin2header.cmake is quadratic on the hex stream (tens of minutes
# for the logo/font); swap in our linear implementation.
cp "${repo_root}/scripts/fastbin2header.cmake" "${ghostling_src}/bin2header.cmake"

echo "building Ghostling libghostty terminal payload"
cmake -S "${ghostling_src}" -B "${ghostling_build}" -G Ninja -DCMAKE_BUILD_TYPE=Release -DZIG_EXECUTABLE="${zig_bin}"
cmake --build "${ghostling_build}"

if [[ ! -x "${ghostling_payload}" ]]; then
	echo "Ghostling build did not produce ${ghostling_payload}" >&2
	exit 1
fi

# Make the bundle portable: ghostling's only rpath points into the build tree.
# Add @executable_path so libghostty-vt.dylib is found next to the binary when
# the launcher unpacks it into a temp directory on any machine.
if [[ "$(uname -s)" == "Darwin" ]]; then
	echo "adding @executable_path rpath for libghostty-vt"
	install_name_tool -add_rpath @executable_path "${ghostling_payload}"
fi

cleanup_payloads() {
	rm -f "${payload_dir}/ghostling" "${payload_dir}/toast" "${payload_dir}/libghostty-vt.dylib"
}
trap cleanup_payloads EXIT

cp "${ghostling_payload}" "${payload_dir}/ghostling"
cp "${toast_payload}" "${payload_dir}/toast"
cp -L "${ghostty_dylib}" "${payload_dir}/libghostty-vt.dylib"

echo "building single-file bundle"
version="${TOAST_VERSION:-}"
if [[ -z "${version}" ]]; then
	version="$(sed -n 's/^var version = "\(.*\)"/\1/p' "${repo_root}/cmd/toast/main.go" | head -n 1)"
fi
if [[ -z "${version}" ]]; then
	version="dev"
fi
# Info.plist's CFBundleShortVersionString must be numeric-only (no "v" prefix).
plist_version="${version#v}"
echo "bundle version: ${version}"
(
	cd "${repo_root}"
	go build -ldflags "-X main.version=${version}" -o "${bundle_path}" ./cmd/toastapp
)

echo "wrote ${bundle_path}"

# ── macOS .app bundle ─────────────────────────────────────────────────────
# Wrap the launcher in a proper application bundle so the toast logo shows as
# the dock icon and the app has a name/identity. The launcher is a copy: it
# self-extracts its embedded payloads at runtime.
if [[ "$(uname -s)" == "Darwin" ]]; then
	app_dir="${dist_dir}/Toast.app"
	app_macos="${app_dir}/Contents/MacOS"
	app_res="${app_dir}/Contents/Resources"

	echo "assembling ${app_dir}"
	mkdir -p "${app_macos}" "${app_res}"

	# Generate AppIcon.icns from the toast logo (iconset → iconutil).
	iconset="${build_dir}/AppIcon.iconset"
	mkdir -p "${iconset}"
	for size in 16 32 128 256 512; do
		sips -z "${size}" "${size}" "${logo_src}" --out "${iconset}/icon_${size}x${size}.png" >/dev/null
		double=$((size * 2))
		sips -z "${double}" "${double}" "${logo_src}" --out "${iconset}/icon_${size}x${size}@2x.png" >/dev/null
	done
	iconutil -c icns "${iconset}" -o "${app_res}/AppIcon.icns"

	cat > "${app_dir}/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleExecutable</key>
	<string>toast-libghostty</string>
	<key>CFBundleIconFile</key>
	<string>AppIcon</string>
	<key>CFBundleIdentifier</key>
	<string>dev.toast.editor</string>
	<key>CFBundleInfoDictionaryVersion</key>
	<string>6.0</string>
	<key>CFBundleName</key>
	<string>Toast</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>CFBundleShortVersionString</key>
	<string>${plist_version}</string>
	<key>CFBundleVersion</key>
	<string>1</string>
	<key>LSMinimumSystemVersion</key>
	<string>13.0</string>
	<key>NSHighResolutionCapable</key>
	<true/>
</dict>
</plist>
EOF

	cp "${bundle_path}" "${app_macos}/toast-libghostty"
	chmod +x "${app_macos}/toast-libghostty"
	echo "wrote ${app_dir}"
fi
