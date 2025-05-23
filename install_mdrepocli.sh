#!/bin/bash

# Script to install md-repo-cli

# --- Configuration ---
DEFAULT_MDREPOCLI_VER=$(curl -L -s https://raw.githubusercontent.com/MD-Repo/md-repo-cli/refs/heads/main/VERSION.txt)
INSTALL_DIR="/usr/local/bin" # Where to install the md-repo-cli binary
BINARY_NAME="mdrepo"          # The name of the binary after install
USE_SUDO="true"              # Use sudo for installation (can be overridden)

# --- Functions ---

# Function to download and extract
download_and_extract() {
    local url="$1"
    echo "Downloading and extracting from: $url"
    curl -L -s "$url" | tar zxvf -
}

# Function to install the binary
install_binary() {
    if [ ! -d "$INSTALL_DIR" ]; then
        echo "Creating installation directory: $INSTALL_DIR"
        if [[ "$USE_SUDO" == "true" ]]; then
            sudo mkdir -p "$INSTALL_DIR"
        else
            mkdir -p "$INSTALL_DIR"
        fi
    fi

    echo "Installing $BINARY_NAME to $INSTALL_DIR"
    if [[ "$USE_SUDO" == "true" ]]; then
        sudo mv "$BINARY_NAME" "$INSTALL_DIR/"
        sudo chmod +x "$INSTALL_DIR/$BINARY_NAME" # Make executable
    else
        mv "$BINARY_NAME" "$INSTALL_DIR/"
        chmod +x "$INSTALL_DIR/$BINARY_NAME"
    fi
    echo "$BINARY_NAME installed successfully to $INSTALL_DIR"
}

# Function to display usage
usage() {
    echo "Usage: $0 [options]"
    echo "Options:"
    echo "  --version <version>  Specify the md-repo-cli version to install (e.g., v1.2.3)"
    echo "  --no-sudo          Install without using sudo"
    echo "  --install          Install the binary"
    echo "  --help             Display this help message"
    exit 1
}

# --- Argument Parsing ---
MDREPOCLI_VER="$DEFAULT_MDREPOCLI_VER" # Set the default version
INSTALL="false" # Default to not installing
while [[ $# -gt 0 ]]; do
    case "$1" in
        --version)
            if [[ -z "$2" ]]; then
            echo "Error: --version requires a value"
            usage
            fi
            MDREPOCLI_VER="$2"
            shift # past argument
            ;;
        --no-sudo)
            USE_SUDO="false"
            ;;
        --install)
            INSTALL="true"
            ;;
        --help)
            usage
            ;;
        *)
            echo "Unknown parameter: $1"
            usage
            ;;
    esac
    shift # past option
done

# --- OS and Architecture Detection ---
OS=$(uname -s)
ARCH=$(uname -m)

case "$OS-$ARCH" in
    Darwin-x86_64)
        URL="https://github.com/MD-Repo/md-repo-cli/releases/download/${MDREPOCLI_VER}/gocmd-${MDREPOCLI_VER}-darwin-amd64.tar.gz"
        ;;
    Darwin-arm64)
        URL="https://github.com/MD-Repo/md-repo-cli/releases/download/${MDREPOCLI_VER}/gocmd-${MDREPOCLI_VER}-darwin-arm64.tar.gz"
        ;;
    Darwin-aarch64)
        URL="https://github.com/MD-Repo/md-repo-cli/releases/download/${MDREPOCLI_VER}/gocmd-${MDREPOCLI_VER}-darwin-arm64.tar.gz"
        ;;
    Linux-x86_64)
        URL="https://github.com/MD-Repo/md-repo-cli/releases/download/${MDREPOCLI_VER}/gocmd-${MDREPOCLI_VER}-linux-amd64.tar.gz"
        ;;
    Linux-arm64)
        URL="https://github.com/MD-Repo/md-repo-cli/releases/download/${MDREPOCLI_VER}/gocmd-${MDREPOCLI_VER}-linux-arm64.tar.gz"
        ;;
    Linux-aarch64)
        URL="https://github.com/MD-Repo/md-repo-cli/releases/download/${MDREPOCLI_VER}/gocmd-${MDREPOCLI_VER}-linux-arm64.tar.gz"
        ;;
    *)
        echo "Unsupported OS/Architecture: $OS-$ARCH"
        exit 1
        ;;
esac

# --- Download and Install ---
download_and_extract "$URL"

if [[ "$INSTALL" == "true" ]]; then
    # After extraction, move the binary to the install location
    install_binary
else
    echo "$BINARY_NAME downloaded successfully"
fi
