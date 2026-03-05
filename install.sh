#!/bin/sh
# SecTUI installer - https://github.com/orlandobianco/SecTUI
# Usage: curl -fsSL https://orlandobianco.github.io/SecTUI/install.sh | sh
#
# Environment variables:
#   SECTUI_VERSION   - version to install (default: latest)
#   SECTUI_INSTALL   - install directory (default: /usr/local/bin)

set -e

REPO="orlandobianco/SecTUI"
INSTALL_DIR="${SECTUI_INSTALL:-/usr/local/bin}"
BINARY_NAME="sectui"

# --- Helpers ----------------------------------------------------------------

info()  { printf '\033[1;36m%s\033[0m %s\n' "[info]"  "$*"; }
ok()    { printf '\033[1;32m%s\033[0m %s\n' "[ok]"    "$*"; }
warn()  { printf '\033[1;33m%s\033[0m %s\n' "[warn]"  "$*" >&2; }
error() { printf '\033[1;31m%s\033[0m %s\n' "[error]" "$*" >&2; exit 1; }

need_cmd() {
  if ! command -v "$1" > /dev/null 2>&1; then
    error "required command not found: $1"
  fi
}

# --- Detect platform --------------------------------------------------------

detect_os() {
  case "$(uname -s)" in
    Linux*)  echo "linux" ;;
    Darwin*) echo "darwin" ;;
    *)       error "unsupported OS: $(uname -s)" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64)   echo "amd64" ;;
    aarch64|arm64)   echo "arm64" ;;
    *)               error "unsupported architecture: $(uname -m)" ;;
  esac
}

# --- Resolve version --------------------------------------------------------

resolve_version() {
  if [ -n "${SECTUI_VERSION:-}" ]; then
    echo "$SECTUI_VERSION"
    return
  fi

  need_cmd curl

  # Note: /releases/latest ignores prereleases, so we query /releases and
  # pick the first (most recent) tag instead.
  version=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases" \
    | grep '"tag_name"' \
    | head -1 \
    | sed 's/.*"tag_name": *"//;s/".*//')

  if [ -z "$version" ]; then
    error "could not determine latest version. Set SECTUI_VERSION manually."
  fi

  echo "$version"
}

# --- Download & install -----------------------------------------------------

install() {
  need_cmd curl
  need_cmd chmod

  OS="$(detect_os)"
  ARCH="$(detect_arch)"
  VERSION="$(resolve_version)"
  TAG="${VERSION#v}"  # strip leading v if present for URL construction

  ASSET="${BINARY_NAME}-${OS}-${ARCH}"
  URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET}"
  CHECKSUM_URL="${URL}.sha256"

  info "Installing SecTUI ${VERSION} (${OS}/${ARCH})"

  TMP_DIR=$(mktemp -d)
  trap 'rm -rf "$TMP_DIR"' EXIT

  info "Downloading ${URL}"
  curl -fsSL -o "${TMP_DIR}/${ASSET}" "$URL" || error "download failed. Check that version ${VERSION} exists."

  # Verify checksum if sha256sum is available.
  if command -v sha256sum > /dev/null 2>&1; then
    info "Verifying checksum..."
    curl -fsSL -o "${TMP_DIR}/${ASSET}.sha256" "$CHECKSUM_URL" 2>/dev/null
    if [ -f "${TMP_DIR}/${ASSET}.sha256" ]; then
      cd "$TMP_DIR"
      if sha256sum -c "${ASSET}.sha256" > /dev/null 2>&1; then
        ok "Checksum verified"
      else
        error "checksum verification failed!"
      fi
      cd - > /dev/null
    else
      warn "Could not download checksum file, skipping verification"
    fi
  elif command -v shasum > /dev/null 2>&1; then
    info "Verifying checksum..."
    curl -fsSL -o "${TMP_DIR}/${ASSET}.sha256" "$CHECKSUM_URL" 2>/dev/null
    if [ -f "${TMP_DIR}/${ASSET}.sha256" ]; then
      EXPECTED=$(awk '{print $1}' "${TMP_DIR}/${ASSET}.sha256")
      ACTUAL=$(shasum -a 256 "${TMP_DIR}/${ASSET}" | awk '{print $1}')
      if [ "$EXPECTED" = "$ACTUAL" ]; then
        ok "Checksum verified"
      else
        error "checksum verification failed!"
      fi
    else
      warn "Could not download checksum file, skipping verification"
    fi
  else
    warn "sha256sum/shasum not found, skipping checksum verification"
  fi

  chmod +x "${TMP_DIR}/${ASSET}"

  # Install to INSTALL_DIR (may need sudo).
  if [ -w "$INSTALL_DIR" ]; then
    mv "${TMP_DIR}/${ASSET}" "${INSTALL_DIR}/${BINARY_NAME}"
  else
    info "Elevated permissions required to install to ${INSTALL_DIR}"
    sudo mv "${TMP_DIR}/${ASSET}" "${INSTALL_DIR}/${BINARY_NAME}"
  fi

  ok "SecTUI ${VERSION} installed to ${INSTALL_DIR}/${BINARY_NAME}"
  echo ""
  info "Run 'sectui' to launch the TUI dashboard"
  info "Run 'sectui scan' for a quick security scan"
  info "Run 'sectui --help' for all commands"
}

# --- Main -------------------------------------------------------------------

install
