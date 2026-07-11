#!/usr/bin/env bash
# install_godot_xterm.sh — Downloads the pinned godot-xterm release and
# extracts its addons/godot_xterm/ folder into place.
#
# godot-xterm ships prebuilt native libraries in its release zip, so no
# `scons` build step is required. Nothing here is committed to the repo
# (see .gitignore + VENDOR.md) — this script is the documented install path.
#
# Usage: godot/addons/install_godot_xterm.sh
set -euo pipefail

REPO="lihop/godot-xterm"
TAG="v4.0.3"
PINNED_COMMIT="e65b9d1d2a5982c721aeb7ddff8e5b9876e53ec6"
ASSET="godot-xterm-${TAG}.zip"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${TAG}/${ASSET}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ADDONS_DIR="${SCRIPT_DIR}"
TARGET_DIR="${ADDONS_DIR}/godot_xterm"

if [[ -d "${TARGET_DIR}" ]]; then
	echo "godot-xterm already installed at ${TARGET_DIR} — remove it first to reinstall." >&2
	exit 0
fi

WORKDIR="$(mktemp -d)"
trap 'rm -rf "${WORKDIR}"' EXIT

echo "Downloading ${ASSET} (pinned tag ${TAG}, commit ${PINNED_COMMIT})..."
if command -v curl >/dev/null 2>&1; then
	curl -fL --output "${WORKDIR}/${ASSET}" "${DOWNLOAD_URL}"
elif command -v wget >/dev/null 2>&1; then
	wget -O "${WORKDIR}/${ASSET}" "${DOWNLOAD_URL}"
else
	echo "Neither curl nor wget is available — install one and retry." >&2
	exit 1
fi

echo "Extracting addons/godot_xterm/ ..."
if ! command -v unzip >/dev/null 2>&1; then
	echo "unzip is required to extract the release archive — install it and retry." >&2
	exit 1
fi
unzip -q "${WORKDIR}/${ASSET}" "addons/godot_xterm/*" -d "${WORKDIR}/extracted"

if [[ ! -d "${WORKDIR}/extracted/addons/godot_xterm" ]]; then
	echo "Extraction did not produce addons/godot_xterm/ — the release layout may have changed." >&2
	exit 1
fi

mv "${WORKDIR}/extracted/addons/godot_xterm" "${TARGET_DIR}"

echo "Installed godot-xterm ${TAG} to ${TARGET_DIR}"
echo "Open the project in the Godot editor to let it register the Terminal/PTY GDExtension classes."
