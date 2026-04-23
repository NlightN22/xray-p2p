#include "runtime_state.h"

#include "simple_json.h"

#include <fstream>
#include <sstream>

namespace xp2p::ui {

static std::optional<std::string> ReadFileText(const std::string& path) {
    std::ifstream f(path, std::ios::in | std::ios::binary);
    if (!f) {
        return std::nullopt;
    }
    std::ostringstream ss;
    ss << f.rdbuf();
    std::string text = ss.str();
    if (text.empty()) {
        return std::nullopt;
    }
    return text;
}

std::optional<ClientStateFile> TryLoadClientStateFile(const std::string& path) {
    if (path.empty()) {
        return std::nullopt;
    }
    auto textOpt = ReadFileText(path);
    if (!textOpt) {
        return std::nullopt;
    }
    const std::string& json = *textOpt;

    ClientStateFile out{};
    out.tunEnabled = ExtractBool(json, "tun_enabled").value_or(false);
    out.mode = ExtractString(json, "mode").value_or("");

    auto runtimeObj = ExtractObject(json, "runtime");
    if (!runtimeObj) {
        return out;
    }
    ClientRuntimeState runtime{};
    runtime.socksReady = ExtractBool(*runtimeObj, "socks_ready").value_or(false);
    runtime.lastError = ExtractString(*runtimeObj, "last_error").value_or("");
    runtime.hasTimestamp = HasKey(*runtimeObj, "timestamp");

    if (auto tunObj = ExtractObject(*runtimeObj, "tun")) {
        RuntimeTunState tun{};
        tun.name = ExtractString(*tunObj, "name").value_or("");
        tun.ipv4 = ExtractString(*tunObj, "ipv4").value_or("");
        tun.prefix = ExtractInt(*tunObj, "prefix").value_or(0);
        tun.operStatus = ExtractString(*tunObj, "oper_status").value_or("");
        tun.dadState = ExtractString(*tunObj, "dad_state").value_or("");
        tun.ready = ExtractBool(*tunObj, "ready").value_or(false);
        runtime.tun = tun;
    }

    if (auto routesObj = ExtractObject(*runtimeObj, "routes")) {
        RuntimeRoutesState routes{};
        routes.redirectApplied = ExtractBool(*routesObj, "redirect_applied").value_or(false);
        routes.redirectCount = ExtractInt(*routesObj, "redirect_count").value_or(0);
        routes.fullApplied = ExtractBool(*routesObj, "full_applied").value_or(false);
        routes.fullBypassCount = ExtractInt(*routesObj, "full_bypass_count").value_or(0);
        runtime.routes = routes;
    }

    out.runtime = runtime;
    return out;
}

std::optional<ServerStateFile> TryLoadServerStateFile(const std::string& path) {
    if (path.empty()) {
        return std::nullopt;
    }
    auto textOpt = ReadFileText(path);
    if (!textOpt) {
        return std::nullopt;
    }
    const std::string& json = *textOpt;

    ServerStateFile out{};
    out.tunEnabled = ExtractBool(json, "tun_enabled").value_or(false);
    out.mode = ExtractString(json, "mode").value_or("");
    out.hasTimestamp = HasKey(json, "timestamp");
    return out;
}

}

