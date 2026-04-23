#pragma once

#include <optional>
#include <string>

namespace xp2p::ui {

struct RuntimeTunState {
    std::string name;
    std::string ipv4;
    int prefix = 0;
    std::string operStatus;
    std::string dadState;
    bool ready = false;
};

struct RuntimeRoutesState {
    bool redirectApplied = false;
    int redirectCount = 0;
    bool fullApplied = false;
    int fullBypassCount = 0;
};

struct ClientRuntimeState {
    bool socksReady = false;
    bool hasSocksReady = false;
    std::string lastError;
    bool hasTimestamp = false;
    std::optional<RuntimeTunState> tun;
    std::optional<RuntimeRoutesState> routes;
};

struct ClientStateFile {
    bool tunEnabled = false;
    std::string mode;
    std::optional<ClientRuntimeState> runtime;
};

struct ServerStateFile {
    bool tunEnabled = false;
    std::string mode;
    bool hasTimestamp = false;
};

std::optional<ClientStateFile> TryLoadClientStateFile(const std::string& path);
std::optional<ServerStateFile> TryLoadServerStateFile(const std::string& path);

}
