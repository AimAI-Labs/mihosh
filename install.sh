#!/bin/bash
set -e

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

echo_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 1. Detect OS and Architecture
OS="$(uname -s)"
ARCH="$(uname -m)"

case "$OS" in
    Linux)
        OS_TYPE="linux"
        ;;
    Darwin)
        OS_TYPE="darwin"
        ;;
    *)
        echo_error "Unsupported operating system: $OS"
        exit 1
        ;;
esac

case "$ARCH" in
    x86_64)
        ARCH_TYPE="amd64"
        ;;
    aarch64|arm64)
        ARCH_TYPE="arm64"
        ;;
    *)
        echo_error "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

BINARY_NAME="mihosh-${OS_TYPE}-${ARCH_TYPE}"
INSTALL_DIR="/usr/local/bin"
TARGET_NAME="mihosh"

echo_info "Detected System: $OS_TYPE $ARCH_TYPE"

# 2. Get Latest Version
echo_info "Checking latest version..."
LATEST_RELEASE_URL="https://api.github.com/repos/AimAI-Labs/mihosh/releases/latest"
VERSION=$(curl -s $LATEST_RELEASE_URL | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$VERSION" ]; then
    echo_error "Failed to fetch latest version info."
    exit 1
fi

echo_info "Latest version: $VERSION"

# Check if already installed and up-to-date
if command -v "$TARGET_NAME" >/dev/null 2>&1; then
    CURRENT_VERSION=$("$TARGET_NAME" version 2>/dev/null | awk '{print $2}')
    if [ "${CURRENT_VERSION#v}" = "${VERSION#v}" ]; then
        echo_info "$TARGET_NAME is already up to date (version $VERSION). Skip installation."
        exit 0
    else
        echo_info "Found existing version: ${CURRENT_VERSION:-unknown}. Upgrading to $VERSION..."
    fi
fi

# 3. Construct Download URL
DOWNLOAD_URL="https://github.com/AimAI-Labs/mihosh/releases/download/${VERSION}/${BINARY_NAME}.tar.gz"

# 4. Download and Install
TMP_DIR=$(mktemp -d)
ARCHIVE_FILE="${TMP_DIR}/${BINARY_NAME}.tar.gz"
LOCAL_ARCHIVE="${BINARY_NAME}.tar.gz"

if [ -f "$LOCAL_ARCHIVE" ]; then
    echo_info "Found local archive $LOCAL_ARCHIVE. Skipping download."
    cp "$LOCAL_ARCHIVE" "$ARCHIVE_FILE"
else
    echo_info "Downloading ${BINARY_NAME} from ${DOWNLOAD_URL}..."
    if curl -L -o "$ARCHIVE_FILE" --fail "$DOWNLOAD_URL"; then
        echo_info "Download successful."
    else
        echo_error "Download failed. Please check your internet connection or if the asset exists for your architecture."
        rm -rf "$TMP_DIR"
        exit 1
    fi
fi

echo_info "Extracting archive..."
tar -xzf "$ARCHIVE_FILE" -C "$TMP_DIR"

# Find the binary (assuming it's named 'mihosh' or similar)
EXTRACTED_BINARY=$(find "$TMP_DIR" -type f -name "${TARGET_NAME}" | head -n 1)

if [ -z "$EXTRACTED_BINARY" ]; then
    # Fallback types
    EXTRACTED_BINARY=$(find "$TMP_DIR" -type f -perm -u+x ! -name "*.tar.gz" | head -n 1)
fi

if [ -z "$EXTRACTED_BINARY" ]; then
    echo_error "Could not find binary in the downloaded archive."
    rm -rf "$TMP_DIR"
    exit 1
fi

chmod +x "$EXTRACTED_BINARY"

echo_info "Installing to ${INSTALL_DIR}/${TARGET_NAME}..."
if [ -w "$INSTALL_DIR" ]; then
    mv -f "$EXTRACTED_BINARY" "${INSTALL_DIR}/${TARGET_NAME}"
else
    echo_info "Sudo permission required to install to ${INSTALL_DIR}"
    sudo mv -f "$EXTRACTED_BINARY" "${INSTALL_DIR}/${TARGET_NAME}"
fi

rm -rf "$TMP_DIR"

echo_info "Installation completed successfully!"
echo_info "Run 'mihosh' to start."
