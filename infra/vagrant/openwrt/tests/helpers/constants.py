from __future__ import annotations

SETUP_URL = "https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/xsetup.sh"
SERVER_SCRIPT_URL = "https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/server.sh"
CLIENT_SCRIPT_URL = "https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/client.sh"
SERVER_USER_URL = "https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/server_user.sh"
SERVER_REVERSE_URL = "https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/server_reverse.sh"
SERVER_CERT_APPLY_URL = "https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/lib/server_install_cert_apply.sh"
SERVER_CERT_SELFSIGNED_URL = "https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/lib/server_install_cert_selfsigned.sh"
SERVER_CERT_PATHS_URL = "https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/lib/server_cert_paths.sh"

SERVER_CONFIG_DIR = "/etc/xray-p2p"
SERVER_CLIENTS_PATH = f"{SERVER_CONFIG_DIR}/config/clients.json"
SERVER_INBOUNDS_PATH = f"{SERVER_CONFIG_DIR}/inbounds.json"
SERVER_ROUTING_PATH = f"{SERVER_CONFIG_DIR}/routing.json"
SERVER_TUNNELS_PATH = f"{SERVER_CONFIG_DIR}/config/tunnels.json"
SERVER_SERVICE_PATH = "/etc/init.d/xray-p2p"
