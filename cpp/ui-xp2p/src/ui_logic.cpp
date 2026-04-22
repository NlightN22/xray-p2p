#include "ui_logic.h"

#include <cctype>
#include <string_view>

namespace xp2p::ui {

static bool EqualsIgnoreCase(const std::string& left, std::string_view right) {
    if (left.size() != right.size()) {
        return false;
    }
    for (size_t i = 0; i < right.size(); i++) {
        const char a = left[i];
        const char b = right[i];
        if (a == b) {
            continue;
        }
        const char al = static_cast<char>(std::tolower(static_cast<unsigned char>(a)));
        const char bl = static_cast<char>(std::tolower(static_cast<unsigned char>(b)));
        if (al != bl) {
            return false;
        }
    }
    return true;
}

bool IsServiceRunning(const std::string& status) {
    return EqualsIgnoreCase(status, "Running");
}

bool IsServiceStopped(const std::string& status) {
    return EqualsIgnoreCase(status, "Stopped");
}

bool IsServicePending(const std::string& status) {
    return EqualsIgnoreCase(status, "StartPending") || EqualsIgnoreCase(status, "StopPending") ||
        EqualsIgnoreCase(status, "PausePending") || EqualsIgnoreCase(status, "ContinuePending");
}

ServiceButtons GetServiceButtons(const std::string& status, bool busy) {
    ServiceButtons b{};
    if (busy || IsServicePending(status)) {
        b.startEnabled = false;
        b.stopEnabled = false;
        b.restartEnabled = false;
        return b;
    }
    if (IsServiceRunning(status)) {
        b.startEnabled = false;
        b.stopEnabled = true;
        b.restartEnabled = true;
        return b;
    }
    if (IsServiceStopped(status)) {
        b.startEnabled = true;
        b.stopEnabled = false;
        b.restartEnabled = true;
        return b;
    }
    b.startEnabled = true;
    b.stopEnabled = true;
    b.restartEnabled = true;
    return b;
}

std::string BuildTrayTooltip(const std::string& clientStatus, const std::string& serverStatus, bool busy) {
    std::string tip = "xp2p | client: " + clientStatus + " | server: " + serverStatus;
    if (busy) {
        tip += " | busy";
    }
    return tip;
}

std::string BuildTrayStatusLogLine(const std::string& clientStatus, const std::string& serverStatus, bool busy) {
    return "tray status: client=" + clientStatus + " server=" + serverStatus + " busy=" + (busy ? "1" : "0");
}

}
