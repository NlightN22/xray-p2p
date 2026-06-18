#pragma once

#include "mode_logic.h"

#include <functional>
#include <optional>
#include <string>
#include <vector>

namespace xp2p::ui {

struct OperationResult {
    bool success = false;
    std::string message;

    static OperationResult Ok(const std::string& message);
    static OperationResult Fail(const std::string& message);
};

struct ClientFullTunnelTagState {
    std::string existingTag;
    std::vector<std::string> candidateTags;
};

class ModeManager final {
public:
    explicit ModeManager(std::function<void(const std::string&)> log = nullptr);

    OperationResult ApplyClientMode(ClientMode mode, const std::optional<std::string>& fullTunnelTagOverride = std::nullopt);
    OperationResult ApplyServerMode(ServerMode mode);

    std::string GetClientStatePath() const;
    std::string GetServerStatePath() const;
    ClientFullTunnelTagState GetClientFullTunnelTagState() const;

private:
    std::string GetConfigRoot() const;
    std::string GetLogRoot() const;
    std::string GetAuditLogPath() const;
    std::string GetConfigPath(const std::string& fileName) const;
    std::string GetPendingConfigPath(const std::string& fileName) const;
    std::string GetStateRoot() const;
    std::string GetApplyRequestPath() const;

    void WriteApplyRequest(const std::string& role);
    bool WriteFileWithAudit(const std::string& path, const std::string& content, bool ignoreAuditErrors);

    std::function<void(const std::string&)> log_;
};

}

