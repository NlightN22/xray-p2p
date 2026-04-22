#pragma once

#include <string>

namespace xp2p::ui {

struct ServiceButtons {
    bool startEnabled = true;
    bool stopEnabled = true;
    bool restartEnabled = true;
};

bool IsServiceRunning(const std::string& status);
bool IsServiceStopped(const std::string& status);
bool IsServicePending(const std::string& status);

ServiceButtons GetServiceButtons(const std::string& status, bool busy);

std::string BuildTrayTooltip(const std::string& clientStatus, const std::string& serverStatus, bool busy);
std::string BuildTrayStatusLogLine(const std::string& clientStatus, const std::string& serverStatus, bool busy);

}

