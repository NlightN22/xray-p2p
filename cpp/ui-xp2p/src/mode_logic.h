#pragma once

#include <string>

namespace xp2p::ui {

enum class ClientMode {
    Proxy,
    TunSplit,
    TunFull
};

enum class ServerMode {
    Proxy,
    Tun
};

std::string FormatClientMode(ClientMode mode);
std::string FormatServerMode(ServerMode mode);
std::string FormatPending(const std::string& label);

ClientMode ResolveClientMode(bool tunEnabled, const std::string& modeValue, bool routesFullApplied, bool routesRedirectApplied);
ServerMode ResolveServerMode(bool tunEnabled, const std::string& modeValue);

}

