#pragma once

#include <string>

namespace xp2p::ui {

struct ServiceStatus {
    std::string label;
    bool ok;
    unsigned long error;
};

ServiceStatus QueryServiceStatus(const wchar_t* serviceName);
ServiceStatus StartServiceAndWait(const wchar_t* serviceName, unsigned long timeoutMs);
ServiceStatus StopServiceAndWait(const wchar_t* serviceName, unsigned long timeoutMs);
ServiceStatus RestartServiceAndWait(const wchar_t* serviceName, unsigned long timeoutMs);

}

