#!/bin/bash
set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

echo_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

echo_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

INSTALL_DIR="/usr/local/bin"
TARGET_NAME="mihosh"

detect_system() {
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
    echo_info "Detected System: $OS_TYPE $ARCH_TYPE"
}

do_install() {
    detect_system

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
            return 0
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

    # Find the binary
    EXTRACTED_BINARY=$(find "$TMP_DIR" -type f -name "${TARGET_NAME}" | head -n 1)

    if [ -z "$EXTRACTED_BINARY" ]; then
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
}

do_uninstall() {
    echo_info "Starting uninstallation of mihosh..."

    TARGET_PATH="${INSTALL_DIR}/${TARGET_NAME}"
    if [ ! -f "$TARGET_PATH" ] && command -v "$TARGET_NAME" >/dev/null 2>&1; then
        TARGET_PATH=$(command -v "$TARGET_NAME")
    fi

    if [ -f "$TARGET_PATH" ]; then
        echo_info "Removing binary: $TARGET_PATH"
        if [ -w "$(dirname "$TARGET_PATH")" ]; then
            rm -f "$TARGET_PATH"
        else
            echo_info "Sudo permission required to remove $TARGET_PATH"
            sudo rm -f "$TARGET_PATH"
        fi
        echo_info "Binary file removed successfully."
    else
        echo_warn "No $TARGET_NAME binary found in system path."
    fi

    CONFIG_DIR="${HOME}/.mihosh"
    if [ -d "$CONFIG_DIR" ]; then
        CONFIRM_RM_CONFIG=""
        IF_PROMPTED=0

        if [ -t 0 ]; then
            IF_PROMPTED=1
            echo_warn "The configuration directory ($CONFIG_DIR) contains your settings and profiles."
            read -rp "Do you want to delete config directory ($CONFIG_DIR)? [y/N]: " CONFIRM_RM_CONFIG
        elif (exec 3</dev/tty) 2>/dev/null; then
            IF_PROMPTED=1
            echo_warn "The configuration directory ($CONFIG_DIR) contains your settings and profiles."
            read -rp "Do you want to delete config directory ($CONFIG_DIR)? [y/N]: " CONFIRM_RM_CONFIG </dev/tty
        fi

        if [ "$IF_PROMPTED" -eq 1 ]; then
            case "$CONFIRM_RM_CONFIG" in
                [yY][eE][sS]|[yY])
                    rm -rf "$CONFIG_DIR"
                    echo_info "Config directory deleted: $CONFIG_DIR"
                    ;;
                *)
                    echo_info "Config directory preserved: $CONFIG_DIR"
                    ;;
            esac
        else
            echo_info "Non-interactive mode, preserving config directory: $CONFIG_DIR"
        fi
    fi

    echo_info "mihosh uninstallation completed."
}

show_help() {
    echo "Mihosh Installer / Uninstaller"
    echo ""
    echo "Usage: ./install.sh [OPTION]"
    echo ""
    echo "Options:"
    echo "  -i, --install      Install or upgrade mihosh (default)"
    echo "  -u, --uninstall    Uninstall mihosh"
    echo "  -h, --help         Show this help message"
}

show_menu() {
    while true; do
        echo -e "${GREEN}"
        cat << 'EOF'
  __  __ _ _                _     
 |  \/  (_) |              | |    
 | \  / |_| |__   ___  ___ | |__  
 | |\/| | |  _ \ / _ \/ __||  _ \ 
 | |  | | | | | | (_) \__ \| | | |
 |_|  |_|_|_| |_|\___/|___/|_| |_|
EOF
        echo -e "${NC}"
        echo "================================="
        echo "1) Install / Upgrade mihosh"
        echo "2) Uninstall mihosh"
        echo "q) Quit"
        echo "---------------------------------"
        read -rp "Please select an option [1-2, q]: " CHOICE
        case "$CHOICE" in
            1)
                do_install
                break
                ;;
            2)
                do_uninstall
                break
                ;;
            q|Q)
                echo_info "Exiting."
                exit 0
                ;;
            *)
                echo_warn "Invalid option: '$CHOICE'. Returning to menu..."
                echo ""
                ;;
        esac
    done
}

# Main entry point
case "$1" in
    -i|--install)
        do_install
        ;;
    -u|--uninstall)
        do_uninstall
        ;;
    -h|--help)
        show_help
        ;;
    "")
        if [ -t 0 ]; then
            show_menu
        else
            do_install
        fi
        ;;
    *)
        echo_error "Unknown option: $1"
        show_help
        exit 1
        ;;
esac
