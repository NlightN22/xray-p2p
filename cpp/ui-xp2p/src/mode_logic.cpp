#include "mode_logic.h"

#include <algorithm>

namespace xp2p::ui {

static std::string Normalize(const std::string& v) {
    std::string out = v;
    out.erase(out.begin(), std::find_if(out.begin(), out.end(), [](unsigned char c) { return !std::isspace(c); }));
    out.erase(std::find_if(out.rbegin(), out.rend(), [](unsigned char c) { return !std::isspace(c); }).base(), out.end());
    std::transform(out.begin(), out.end(), out.begin(), [](unsigned char c) { return static_cast<char>(std::tolower(c)); });
    return out;
}

std::string FormatClientMode(ClientMode mode) {
    switch (mode) {
        case ClientMode::Proxy:
            return "Proxy";
        case ClientMode::TunSplit:
            return "Tun Split";
        case ClientMode::TunFull:
            return "Tun Full";
        default:
            return "Unknown";
    }
}

std::string FormatServerMode(ServerMode mode) {
    switch (mode) {
        case ServerMode::Proxy:
            return "Proxy";
        case ServerMode::Tun:
            return "Tun";
        default:
            return "Unknown";
    }
}

std::string FormatPending(const std::string& label) {
    return label + " (Pending)";
}

ClientMode ResolveClientMode(bool tunEnabled, const std::string& modeValue, bool routesFullApplied, bool routesRedirectApplied) {
    if (!tunEnabled) {
        return ClientMode::Proxy;
    }
    const std::string mode = Normalize(modeValue);
    if (mode == "proxy") {
        return ClientMode::Proxy;
    }
    if (mode == "full" || mode == "tun-full") {
        return ClientMode::TunFull;
    }
    if (mode == "split" || mode == "tun-split") {
        return ClientMode::TunSplit;
    }
    if (routesFullApplied) {
        return ClientMode::TunFull;
    }
    if (routesRedirectApplied) {
        return ClientMode::TunSplit;
    }
    return ClientMode::TunSplit;
}

ServerMode ResolveServerMode(bool tunEnabled, const std::string& modeValue) {
    const std::string mode = Normalize(modeValue);
    if (mode == "proxy") {
        return ServerMode::Proxy;
    }
    if (mode == "tun") {
        return ServerMode::Tun;
    }
    return tunEnabled ? ServerMode::Tun : ServerMode::Proxy;
}

}

