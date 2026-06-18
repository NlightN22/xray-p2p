#include "mode_manager.h"

#include "platform.h"
#include "toml_edit.h"

#include <chrono>
#include <iomanip>
#include <random>
#include <sstream>
#include <stdexcept>
#include <algorithm>

namespace xp2p::ui {

static std::string UtcNowIso8601() {
    using namespace std::chrono;
    auto now = system_clock::now();
    std::time_t tt = system_clock::to_time_t(now);
    std::tm tm{};
#if defined(_WIN32)
    gmtime_s(&tm, &tt);
#else
    gmtime_r(&tt, &tm);
#endif
    std::ostringstream ss;
    ss << std::put_time(&tm, "%Y-%m-%dT%H:%M:%SZ");
    return ss.str();
}

static std::string NewGuidLikeId() {
    std::random_device rd;
    std::mt19937_64 gen(rd());
    std::uniform_int_distribution<uint64_t> dist;
    uint64_t a = dist(gen);
    uint64_t b = dist(gen);
    std::ostringstream ss;
    ss << std::hex << std::nouppercase << std::setfill('0');
    ss << std::setw(8) << static_cast<unsigned>((a >> 32) & 0xffffffffULL);
    ss << "-";
    ss << std::setw(4) << static_cast<unsigned>((a >> 16) & 0xffffULL);
    ss << "-";
    ss << std::setw(4) << static_cast<unsigned>((a)&0xffffULL);
    ss << "-";
    ss << std::setw(4) << static_cast<unsigned>((b >> 48) & 0xffffULL);
    ss << "-";
    ss << std::setw(12) << static_cast<unsigned long long>(b & 0x0000ffffffffffffULL);
    return ss.str();
}

static std::string NormalizeRole(const std::string& role) {
    std::string out = role;
    out.erase(out.begin(), std::find_if(out.begin(), out.end(), [](unsigned char c) { return !std::isspace(c); }));
    out.erase(std::find_if(out.rbegin(), out.rend(), [](unsigned char c) { return !std::isspace(c); }).base(), out.end());
    std::transform(out.begin(), out.end(), out.begin(), [](unsigned char c) { return static_cast<char>(std::tolower(c)); });
    return out;
}

OperationResult OperationResult::Ok(const std::string& message) {
    OperationResult r{};
    r.success = true;
    r.message = message;
    return r;
}

OperationResult OperationResult::Fail(const std::string& message) {
    OperationResult r{};
    r.success = false;
    r.message = message;
    return r;
}

ModeManager::ModeManager(std::function<void(const std::string&)> log) : log_(std::move(log)) {}

static std::string ReadTextOrEmpty(const std::string& path) {
    auto bytes = ReadFileBytesOrEmpty(path);
    if (bytes.empty()) {
        return "";
    }
    return std::string(reinterpret_cast<const char*>(bytes.data()), bytes.size());
}

static std::string ResolveSourceConfig(const std::string& configPath, const std::string& pendingPath) {
    if (FileExists(configPath)) {
        return configPath;
    }
    if (FileExists(pendingPath)) {
        return pendingPath;
    }
    return configPath;
}

static bool MatchesRole(const std::string& existingRole, const std::string& desiredRole) {
    const std::string existing = NormalizeRole(existingRole);
    const std::string desired = NormalizeRole(desiredRole);
    if (desired.empty()) {
        return false;
    }
    if (existing.empty()) {
        return true;
    }
    if (existing == desired) {
        return true;
    }
    return existing == "any";
}

static bool RequiresAnyRole(const std::string& existingRole, const std::string& desiredRole) {
    const std::string existing = NormalizeRole(existingRole);
    const std::string desired = NormalizeRole(desiredRole);
    if (existing.empty() || desired.empty()) {
        return false;
    }
    if (existing == "any" || desired == "any") {
        return false;
    }
    return existing != desired;
}

static std::string ExtractApplyRequestRole(const std::string& json) {
    const std::string key = "\"Role\"";
    size_t pos = json.find(key);
    if (pos == std::string::npos) {
        pos = json.find("\"role\"");
        if (pos == std::string::npos) {
            return "";
        }
    }
    pos = json.find(':', pos);
    if (pos == std::string::npos) {
        return "";
    }
    pos++;
    while (pos < json.size() && std::isspace(static_cast<unsigned char>(json[pos]))) {
        pos++;
    }
    if (pos >= json.size() || json[pos] != '"') {
        return "";
    }
    pos++;
    std::string out;
    while (pos < json.size() && json[pos] != '"') {
        out.push_back(json[pos]);
        pos++;
    }
    return NormalizeRole(out);
}

void ModeManager::WriteApplyRequest(const std::string& role) {
    std::string desiredRole = NormalizeRole(role);
    const std::string path = GetApplyRequestPath();
    if (!path.empty() && FileExists(path)) {
        const std::string existingText = ReadTextOrEmpty(path);
        const std::string existingRole = ExtractApplyRequestRole(existingText);
        if (!existingText.empty() && MatchesRole(existingRole, desiredRole)) {
            if (log_) {
                log_("mode manager: apply request already matches role=" + desiredRole + " path=" + path);
            }
            return;
        }
        if (!existingText.empty() && RequiresAnyRole(existingRole, desiredRole)) {
            desiredRole = "any";
        }
    }

    std::string json;
    json += "{\n";
    json += "  \"Id\": \"" + NewGuidLikeId() + "\",\n";
    json += "  \"Timestamp\": \"" + UtcNowIso8601() + "\",\n";
    json += "  \"Role\": \"" + desiredRole + "\"\n";
    json += "}\n";
    WriteFileWithAudit(path, json, true);
    if (log_) {
        log_("mode manager: apply request written role=" + desiredRole + " path=" + path);
    }
}

bool ModeManager::WriteFileWithAudit(const std::string& path, const std::string& content, bool ignoreAuditErrors) {
    const std::string normalized = NormalizeLineEndings(content);
    const std::vector<unsigned char> data(normalized.begin(), normalized.end());
    const auto old = ReadFileBytesOrEmpty(path);
    if (old == data) {
        if (log_) {
            log_("mode manager: skip write (no changes) path=" + path);
        }
        return false;
    }
    if (!WriteFileAtomic(path, data)) {
        throw std::runtime_error("write failed");
    }

    const std::string auditPath = GetAuditLogPath();
    if (auditPath.empty()) {
        return true;
    }
    try {
        std::ostringstream ss;
        ss << UtcNowIso8601();
        ss << " user=" << GetUserNameForAudit();
        ss << " file=" << path;
        ss << " old_size=" << old.size();
        ss << " new_size=" << data.size();
        const std::string cmd = GetCommandLineForAudit();
        if (!cmd.empty()) {
            ss << " cmd=" << cmd;
        }
        ss << "\n";
        AppendFileTextUtf8(auditPath, ss.str());
    } catch (...) {
        if (!ignoreAuditErrors) {
            throw;
        }
        if (log_) {
            log_("mode manager: audit write skipped path=" + auditPath);
        }
    }
    return true;
}

OperationResult ModeManager::ApplyClientMode(ClientMode mode, const std::optional<std::string>& fullTunnelTagOverride) {
    const bool desiredTunEnabled = mode != ClientMode::Proxy;
    const std::string desiredTunMode = (mode == ClientMode::TunFull) ? "full" : "split";
    const std::string configPath = GetConfigPath("xp2p-client.toml");
    const std::string legacyPendingPath = GetPendingConfigPath("xp2p-client.toml");
    const std::string sourcePath = ResolveSourceConfig(configPath, legacyPendingPath);

    if (log_) {
        log_("mode manager: client mode request " + FormatClientMode(mode));
        log_("mode manager: client config source=" + sourcePath + " desired=" + configPath);
    }

    std::string content = ReadTextOrEmpty(sourcePath);
    content = UpdateTomlValue(content, "client", "tun_enabled", desiredTunEnabled ? "true" : "false");
    if (desiredTunEnabled) {
        if (mode == ClientMode::TunFull) {
            std::string existingTag = ReadTomlValue(content, "client", "full_tunnel_tag").value_or("");
            existingTag = TrimTomlQuotes(existingTag);
            std::string resolvedTag = fullTunnelTagOverride.has_value() ? *fullTunnelTagOverride : existingTag;
            if (resolvedTag.empty()) {
                std::vector<std::string> tags = ReadEndpointTags(content);
                if (log_) {
                    log_("mode manager: client endpoint tags count=" + std::to_string(tags.size()));
                }
                if (tags.empty()) {
                    if (log_) {
                        log_("mode manager: client full mode rejected (no endpoints found)");
                    }
                    return OperationResult::Fail("Full mode requires endpoint tag; no endpoints found.");
                }
                if (tags.size() > 1) {
                    if (log_) {
                        log_("mode manager: client full mode rejected (multiple endpoints found)");
                    }
                    return OperationResult::Fail("Full mode requires endpoint tag; multiple endpoints found.");
                }
                resolvedTag = tags[0];
                if (log_) {
                    log_("mode manager: client full mode auto tag=" + resolvedTag);
                }
            }
            if (!resolvedTag.empty()) {
                content = UpdateTomlValue(content, "client", "full_tunnel_tag", "\"" + resolvedTag + "\"");
            }
        }
        content = UpdateTomlValue(content, "client", "tun_mode", "\"" + desiredTunMode + "\"");
    }

    try {
        const bool changed = WriteFileWithAudit(configPath, content, false);
        if (log_) {
            log_("mode manager: client desired config written " + configPath);
        }
        if (changed) {
            WriteApplyRequest("client");
        }
        return OperationResult::Ok("Client mode requested: " + FormatClientMode(mode) + ".");
    } catch (const std::exception& ex) {
        if (log_) {
            log_(std::string("mode manager: client mode update failed ") + ex.what());
        }
        return OperationResult::Fail(std::string("Client mode update failed: ") + ex.what());
    }
}

OperationResult ModeManager::ApplyServerMode(ServerMode mode) {
    const bool desiredTunEnabled = mode == ServerMode::Tun;
    const std::string configPath = GetConfigPath("xp2p-server.toml");
    const std::string legacyPendingPath = GetPendingConfigPath("xp2p-server.toml");
    const std::string sourcePath = ResolveSourceConfig(configPath, legacyPendingPath);

    if (log_) {
        log_("mode manager: server mode request " + FormatServerMode(mode));
        log_("mode manager: server config source=" + sourcePath + " desired=" + configPath);
    }

    std::string content = ReadTextOrEmpty(sourcePath);
    content = UpdateTomlValue(content, "server", "tun_enabled", desiredTunEnabled ? "true" : "false");

    try {
        const bool changed = WriteFileWithAudit(configPath, content, false);
        if (log_) {
            log_("mode manager: server desired config written " + configPath);
        }
        if (changed) {
            WriteApplyRequest("server");
        }
        return OperationResult::Ok("Server mode requested: " + FormatServerMode(mode) + ".");
    } catch (const std::exception& ex) {
        if (log_) {
            log_(std::string("mode manager: server mode update failed ") + ex.what());
        }
        return OperationResult::Fail(std::string("Server mode update failed: ") + ex.what());
    }
}

std::string ModeManager::GetClientStatePath() const {
    return GetConfigPath("xp2p-client.state.json");
}

std::string ModeManager::GetServerStatePath() const {
    return GetConfigPath("xp2p-server.state.json");
}

ClientFullTunnelTagState ModeManager::GetClientFullTunnelTagState() const {
    ClientFullTunnelTagState st{};
    const std::string configPath = GetConfigPath("xp2p-client.toml");
    const std::string legacyPendingPath = GetPendingConfigPath("xp2p-client.toml");
    const std::string sourcePath = ResolveSourceConfig(configPath, legacyPendingPath);
    const std::string content = ReadTextOrEmpty(sourcePath);
    st.existingTag = TrimTomlQuotes(ReadTomlValue(content, "client", "full_tunnel_tag").value_or(""));
    st.candidateTags = ReadEndpointTags(content);
    return st;
}

std::string ModeManager::GetConfigRoot() const {
    const std::string overrideValue = GetEnv("XP2P_CONFIG_ROOT");
    if (!overrideValue.empty()) {
        return overrideValue;
    }
#if defined(_WIN32)
    const std::string programData = GetEnv("ProgramData");
    if (!programData.empty()) {
        return programData + "\\xp2p";
    }
    return "C:\\ProgramData\\xp2p";
#else
    return "/tmp/xp2p";
#endif
}

std::string ModeManager::GetLogRoot() const {
    const std::string overrideValue = GetEnv("XP2P_LOG_ROOT");
    if (!overrideValue.empty()) {
        return overrideValue;
    }
#if defined(_WIN32)
    return GetConfigRoot() + "\\logs";
#else
    return GetConfigRoot() + "/logs";
#endif
}

std::string ModeManager::GetAuditLogPath() const {
#if defined(_WIN32)
    return GetLogRoot() + "\\audit.log";
#else
    return GetLogRoot() + "/audit.log";
#endif
}

std::string ModeManager::GetConfigPath(const std::string& fileName) const {
    const std::string root = GetConfigRoot();
#if defined(_WIN32)
    return root + "\\" + fileName;
#else
    return root + "/" + fileName;
#endif
}

std::string ModeManager::GetPendingConfigPath(const std::string& fileName) const {
    const std::string root = GetConfigRoot();
#if defined(_WIN32)
    return root + "\\.apply\\pending\\" + fileName;
#else
    return root + "/.apply/pending/" + fileName;
#endif
}

std::string ModeManager::GetStateRoot() const {
    const std::string root = GetConfigRoot();
#if defined(_WIN32)
    return root + "\\.state";
#else
    return root + "/.state";
#endif
}

std::string ModeManager::GetApplyRequestPath() const {
    const std::string root = GetStateRoot();
#if defined(_WIN32)
    return root + "\\apply.request";
#else
    return root + "/apply.request";
#endif
}

}
