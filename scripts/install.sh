#!/usr/bin/env sh
# Install pgwd from GitHub releases.
# Usage: curl -sSL https://raw.githubusercontent.com/hrodrig/pgwd/main/scripts/install.sh | sh
# Or: BINDIR=~/bin sh install.sh

set -e

REPO="hrodrig/pgwd"
BINDIR="${BINDIR:-/usr/local/bin}"
VERSION="${VERSION:-latest}"

# Detect OS and arch
detect_platform() {
  _os=""
  _arch=""
  _ext="tar.gz"

  case "$(uname -s)" in
    Linux)   _os="linux" ;;
    Darwin)  _os="darwin" ;;
    FreeBSD) _os="freebsd" ;;
    OpenBSD) _os="openbsd" ;;
    NetBSD)  _os="netbsd" ;;
    *)
      echo "Unsupported OS: $(uname -s)" >&2
      exit 1
      ;;
  esac

  case "$(uname -m)" in
    x86_64|amd64) _arch="amd64" ;;
    aarch64|arm64) _arch="arm64" ;;
    armv7l|armv6l) _arch="arm" ;;
    *)
      echo "Unsupported arch: $(uname -m)" >&2
      exit 1
      ;;
  esac

  if [ "$_os" = "windows" ]; then
    _ext="zip"
  fi

  echo "${_os} ${_arch} ${_ext}"
}

# Fetch latest release tag from GitHub API
get_latest_tag() {
  _api="https://api.github.com/repos/${REPO}/releases/latest"
  if command -v curl >/dev/null 2>&1; then
    curl -sSL "${_api}" | grep '"tag_name"' | head -1 | sed -E 's/.*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/'
  elif command -v wget >/dev/null 2>&1; then
    wget -qO- "${_api}" | grep '"tag_name"' | head -1 | sed -E 's/.*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/'
  else
    echo "curl or wget required" >&2
    exit 1
  fi
}

# Download and install
main() {
  _platform=$(detect_platform)
  _os=$(echo "${_platform}" | awk '{print $1}')
  _arch=$(echo "${_platform}" | awk '{print $2}')
  _ext=$(echo "${_platform}" | awk '{print $3}')

  if [ "$VERSION" = "latest" ]; then
    _tag=$(get_latest_tag)
  else
    _tag="$VERSION"
  fi

  _name="pgwd_${_tag}_${_os}_${_arch}"
  _url="https://github.com/${REPO}/releases/download/${_tag}/${_name}.${_ext}"

  echo "Installing pgwd ${_tag} (${_os}/${_arch}) to ${BINDIR}"

  _tmpdir=$(mktemp -d)
  trap 'rm -rf "${_tmpdir}"' EXIT

  if command -v curl >/dev/null 2>&1; then
    curl -sSL -o "${_tmpdir}/archive.${_ext}" "${_url}"
  else
    wget -q -O "${_tmpdir}/archive.${_ext}" "${_url}"
  fi

  if [ "$_ext" = "zip" ]; then
    if command -v unzip >/dev/null 2>&1; then
      unzip -q -o "${_tmpdir}/archive.${_ext}" -d "${_tmpdir}"
    else
      echo "unzip required for Windows" >&2
      exit 1
    fi
  else
    tar -xzf "${_tmpdir}/archive.${_ext}" -C "${_tmpdir}"
  fi

  _binary="${_tmpdir}/pgwd"
  if [ ! -f "${_binary}" ]; then
    _binary="${_tmpdir}/${_name}/pgwd"
  fi
  if [ ! -f "${_binary}" ]; then
    echo "Binary not found in archive" >&2
    exit 1
  fi

  mkdir -p "${BINDIR}"
  if [ -w "${BINDIR}" ]; then
    cp "${_binary}" "${BINDIR}/pgwd"
    chmod +x "${BINDIR}/pgwd"
  else
    echo "Need sudo to write to ${BINDIR}"
    sudo cp "${_binary}" "${BINDIR}/pgwd"
    sudo chmod +x "${BINDIR}/pgwd"
  fi

  echo "Installed: ${BINDIR}/pgwd"
  "${BINDIR}/pgwd" -version 2>/dev/null || true
}

main
