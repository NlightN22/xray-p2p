# Install XRAY server
curl -s https://gist.githubusercontent.com/NlightN22/d410a3f9dd674308999f13f3aeb558ff/raw/da2634081050deefd504504d5ecb86406381e366/install_xray_openwrt.sh | sh

# Check if XRAY is not installed and exit with error
# Configure XRAY
# download simple config files for trojan server
# define at config file:
# - server public ip for connecting
# - serverName for tls certificates
# - trojan port

# Create certificates at /etc/xray/

XRAY_CONF_SRV=xray-server.trojan.conf.json
XRAY_CONF_CL=xray-client.trojan.conf.json

cp $XRAY_CONF /etc/xray/
uci set xray.enabled.enabled='1'
uci set xray.config.conffiles="/etc/xray/$XRAY_CONF"
uci commit xray
service xray restart