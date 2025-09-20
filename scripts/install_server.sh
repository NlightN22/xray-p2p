#!/bin/sh
# Install XRAY server
curl -s https://gist.githubusercontent.com/NlightN22/d410a3f9dd674308999f13f3aeb558ff/raw/da2634081050deefd504504d5ecb86406381e366/install_xray_openwrt.sh | sh
# Ensure configuration directory exists
XRAY_CONFIG_DIR="/etc/xray"
if [ ! -d "$XRAY_CONFIG_DIR" ]; then
    echo "Creating XRAY configuration directory at $XRAY_CONFIG_DIR"
    mkdir -p "$XRAY_CONFIG_DIR"
fi
# Download configuration templates from GitHub repository
CONFIG_BASE_URL="https://raw.githubusercontent.com/NlightN22/xray-p2p/main/config_templates/server"
CONFIG_FILES="inbounds.json logs.json outbounds.json"
for file in $CONFIG_FILES; do
    echo "Downloading $file to $XRAY_CONFIG_DIR"
    if ! curl -fsSL "$CONFIG_BASE_URL/$file" -o "$XRAY_CONFIG_DIR/$file"; then
        echo "Failed to download $file" >&2
        exit 1
    fi
    chmod 644 "$XRAY_CONFIG_DIR/$file"
done
# Additional XRAY configuration steps can be added below