#include "../src/mode_manager.h"

#include "test_harness.h"

#include <cstdlib>
#include <filesystem>
#include <fstream>
#include <iostream>
#include <string>

namespace {

using xp2p::ui::tests::Expect;
using xp2p::ui::tests::ExpectContains;

std::string ReadText(const std::filesystem::path& path) {
    std::ifstream f(path, std::ios::in | std::ios::binary);
    std::string s((std::istreambuf_iterator<char>(f)), std::istreambuf_iterator<char>());
    return s;
}

void WriteText(const std::filesystem::path& path, const std::string& text) {
    std::filesystem::create_directories(path.parent_path());
    std::ofstream f(path, std::ios::out | std::ios::binary | std::ios::trunc);
    f << text;
}

struct TempConfigRoot {
    std::string prevRoot;
    std::string prevLogRoot;
    std::filesystem::path root;
    std::filesystem::path logs;

    TempConfigRoot() {
        const char* r = std::getenv("XP2P_CONFIG_ROOT");
        const char* l = std::getenv("XP2P_LOG_ROOT");
        prevRoot = r ? r : "";
        prevLogRoot = l ? l : "";

        root = std::filesystem::temp_directory_path() / ("xp2p-ui-tests-" + std::to_string(std::rand()));
        logs = root / "logs";
        std::filesystem::create_directories(root);
        std::filesystem::create_directories(logs);

#if defined(_WIN32)
        const std::string rootStr = root.string();
        const std::string logsStr = logs.string();
        _putenv_s("XP2P_CONFIG_ROOT", rootStr.c_str());
        _putenv_s("XP2P_LOG_ROOT", logsStr.c_str());
#else
        setenv("XP2P_CONFIG_ROOT", root.c_str(), 1);
        setenv("XP2P_LOG_ROOT", logs.c_str(), 1);
#endif
    }

    ~TempConfigRoot() {
#if defined(_WIN32)
        _putenv_s("XP2P_CONFIG_ROOT", prevRoot.empty() ? "" : prevRoot.c_str());
        _putenv_s("XP2P_LOG_ROOT", prevLogRoot.empty() ? "" : prevLogRoot.c_str());
#else
        if (prevRoot.empty()) {
            unsetenv("XP2P_CONFIG_ROOT");
        } else {
            setenv("XP2P_CONFIG_ROOT", prevRoot.c_str(), 1);
        }
        if (prevLogRoot.empty()) {
            unsetenv("XP2P_LOG_ROOT");
        } else {
            setenv("XP2P_LOG_ROOT", prevLogRoot.c_str(), 1);
        }
#endif
        std::error_code ec;
        std::filesystem::remove_all(root, ec);
    }
};

void TestApplyClientModeFullSingleEndpointTag() {
    TempConfigRoot env;
    xp2p::ui::ModeManager manager;

    auto configPath = env.root / "xp2p-client.toml";
    WriteText(configPath, "[[client.endpoints]]\n"
                         "tag = \"proxy-alpha\"\n"
                         "hostname = \"edge.example\"\n");

    auto result = manager.ApplyClientMode(xp2p::ui::ClientMode::TunFull);
    Expect(result.success, "ApplyClientMode full single endpoint success");
    Expect(std::filesystem::exists(configPath), "client config exists");
    const std::string content = ReadText(configPath);
    ExpectContains(content, "[client]", "client section present");
    ExpectContains(content, "tun_enabled = true", "tun enabled");
    ExpectContains(content, "tun_mode = \"full\"", "tun mode full");
    ExpectContains(content, "full_tunnel_tag = \"proxy-alpha\"", "full tunnel tag set");
}

void TestApplyClientModeFullInlineEndpointTag() {
    TempConfigRoot env;
    xp2p::ui::ModeManager manager;

    auto configPath = env.root / "xp2p-client.toml";
    WriteText(configPath, "[client]\n"
                         "endpoints = [{ tag = \"proxy-inline\", hostname = \"edge.example\" }]\n");

    auto result = manager.ApplyClientMode(xp2p::ui::ClientMode::TunFull);
    Expect(result.success, "ApplyClientMode full inline endpoint success");
    const std::string content = ReadText(configPath);
    ExpectContains(content, "full_tunnel_tag = \"proxy-inline\"", "inline tag set");
}

void TestApplyClientModeFullFailsMultipleEndpoints() {
    TempConfigRoot env;
    xp2p::ui::ModeManager manager;

    auto configPath = env.root / "xp2p-client.toml";
    const std::string original =
        "[[client.endpoints]]\n"
        "tag = \"proxy-alpha\"\n"
        "hostname = \"edge.example\"\n"
        "\n"
        "[[client.endpoints]]\n"
        "tag = \"proxy-beta\"\n"
        "hostname = \"edge2.example\"\n";
    WriteText(configPath, original);

    auto result = manager.ApplyClientMode(xp2p::ui::ClientMode::TunFull);
    Expect(!result.success, "ApplyClientMode full multiple endpoints fails");
    Expect(ReadText(configPath) == original, "client config unchanged on failure");
}

void TestApplyClientModeFullOverrideTagWritesDesired() {
    TempConfigRoot env;
    xp2p::ui::ModeManager manager;

    auto configPath = env.root / "xp2p-client.toml";
    WriteText(configPath, "[[client.endpoints]]\n"
                         "tag = \"proxy-alpha\"\n"
                         "hostname = \"edge.example\"\n"
                         "\n"
                         "[[client.endpoints]]\n"
                         "tag = \"proxy-beta\"\n"
                         "hostname = \"edge2.example\"\n");

    auto result = manager.ApplyClientMode(xp2p::ui::ClientMode::TunFull, std::string("proxy-beta"));
    Expect(result.success, "ApplyClientMode full override success");
    const std::string content = ReadText(configPath);
    ExpectContains(content, "full_tunnel_tag = \"proxy-beta\"", "override tag set");
}

void TestApplyClientModeSplitWritesDesiredAndRequest() {
    TempConfigRoot env;
    xp2p::ui::ModeManager manager;

    auto result = manager.ApplyClientMode(xp2p::ui::ClientMode::TunSplit);
    Expect(result.success, "ApplyClientMode split success");

    auto configPath = env.root / "xp2p-client.toml";
    Expect(std::filesystem::exists(configPath), "split writes desired config");
    const std::string content = ReadText(configPath);
    ExpectContains(content, "[client]", "split contains client section");
    ExpectContains(content, "tun_enabled = true", "split tun enabled");
    ExpectContains(content, "tun_mode = \"split\"", "split tun mode");

    auto requestPath = env.root / ".state" / "apply.request";
    Expect(std::filesystem::exists(requestPath), "apply.request exists");
}

void TestApplyClientModeProxyWritesDesired() {
    TempConfigRoot env;
    xp2p::ui::ModeManager manager;

    auto result = manager.ApplyClientMode(xp2p::ui::ClientMode::Proxy);
    Expect(result.success, "ApplyClientMode proxy success");

    auto configPath = env.root / "xp2p-client.toml";
    Expect(std::filesystem::exists(configPath), "proxy writes desired config");
    const std::string content = ReadText(configPath);
    ExpectContains(content, "[client]", "proxy contains client section");
    ExpectContains(content, "tun_enabled = false", "proxy tun disabled");
}

void TestApplyClientModeNoOpDoesNotWriteRequest() {
    TempConfigRoot env;
    xp2p::ui::ModeManager manager;

    auto configPath = env.root / "xp2p-client.toml";
    WriteText(configPath, "[client]\n"
                         "tun_enabled = true\n"
                         "tun_mode = \"split\"\n");

    auto result = manager.ApplyClientMode(xp2p::ui::ClientMode::TunSplit);
    Expect(result.success, "ApplyClientMode split no-op success");

    auto requestPath = env.root / ".state" / "apply.request";
    Expect(!std::filesystem::exists(requestPath), "client no-op does not write apply.request");
}

void TestApplyServerModeTunWritesDesiredAndRequest() {
    TempConfigRoot env;
    xp2p::ui::ModeManager manager;

    auto result = manager.ApplyServerMode(xp2p::ui::ServerMode::Tun);
    Expect(result.success, "ApplyServerMode tun success");

    auto configPath = env.root / "xp2p-server.toml";
    Expect(std::filesystem::exists(configPath), "server writes desired config");
    const std::string content = ReadText(configPath);
    ExpectContains(content, "[server]", "server contains server section");
    ExpectContains(content, "tun_enabled = true", "server tun enabled");

    auto requestPath = env.root / ".state" / "apply.request";
    Expect(std::filesystem::exists(requestPath), "server apply.request exists");
}

void TestApplyServerModeNoOpDoesNotWriteRequest() {
    TempConfigRoot env;
    xp2p::ui::ModeManager manager;

    auto configPath = env.root / "xp2p-server.toml";
    WriteText(configPath, "[server]\n"
                         "tun_enabled = true\n");

    auto result = manager.ApplyServerMode(xp2p::ui::ServerMode::Tun);
    Expect(result.success, "ApplyServerMode tun no-op success");

    auto requestPath = env.root / ".state" / "apply.request";
    Expect(!std::filesystem::exists(requestPath), "server no-op does not write apply.request");
}

} // namespace

void RunModeManagerTests() {
    TestApplyClientModeFullSingleEndpointTag();
    TestApplyClientModeFullInlineEndpointTag();
    TestApplyClientModeFullFailsMultipleEndpoints();
    TestApplyClientModeFullOverrideTagWritesDesired();
    TestApplyClientModeSplitWritesDesiredAndRequest();
    TestApplyClientModeProxyWritesDesired();
    TestApplyClientModeNoOpDoesNotWriteRequest();
    TestApplyServerModeTunWritesDesiredAndRequest();
    TestApplyServerModeNoOpDoesNotWriteRequest();
}
