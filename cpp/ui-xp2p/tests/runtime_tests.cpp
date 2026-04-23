#include "../src/mode_logic.h"
#include "../src/runtime_state.h"
#include "../src/runtime_view.h"
#include "../src/simple_json.h"

#include "test_harness.h"

#include <filesystem>
#include <fstream>
#include <string>

namespace {

using xp2p::ui::tests::Expect;
using xp2p::ui::tests::ExpectContains;

std::filesystem::path WriteTemp(const std::string& name, const std::string& content) {
    auto dir = std::filesystem::temp_directory_path() / "xp2p-ui-runtime-tests";
    std::filesystem::create_directories(dir);
    auto path = dir / name;
    std::ofstream f(path, std::ios::out | std::ios::binary | std::ios::trunc);
    f << content;
    return path;
}

void TestSimpleJsonBasics() {
    const std::string json = R"({"a":true,"b":123,"c":"hello","obj":{"x":1}})";
    Expect(xp2p::ui::ExtractBool(json, "a").value_or(false) == true, "json bool");
    Expect(xp2p::ui::ExtractInt(json, "b").value_or(0) == 123, "json int");
    Expect(xp2p::ui::ExtractString(json, "c").value_or("") == "hello", "json string");
    auto obj = xp2p::ui::ExtractObject(json, "obj");
    Expect(obj.has_value(), "json object extract");
    if (obj) {
        Expect(xp2p::ui::ExtractInt(*obj, "x").value_or(0) == 1, "json nested int");
    }
}

void TestRuntimeViewProxyReady() {
    const std::string json = R"({
  "tun_enabled": false,
  "mode": "proxy",
  "runtime": {
    "socks_ready": true,
    "timestamp": "2026-01-01T00:00:00Z"
  }
})";
    auto path = WriteTemp("client-proxy.json", json);
    auto state = xp2p::ui::TryLoadClientStateFile(path.string());
    auto view = xp2p::ui::BuildClientRuntimeView("Running", state);
    Expect(view.status == xp2p::ui::ClientRuntimeStatus::Ready, "proxy ready status");
    ExpectContains(view.summary, "Proxy: Ready", "proxy ready summary");
}

void TestRuntimeViewProxyReadyWithoutSocksKey() {
    const std::string json = R"({
  "tun_enabled": false,
  "mode": "proxy",
  "runtime": {
    "timestamp": "2026-01-01T00:00:00Z"
  }
})";
    auto path = WriteTemp("client-proxy-nosocks.json", json);
    auto state = xp2p::ui::TryLoadClientStateFile(path.string());
    auto view = xp2p::ui::BuildClientRuntimeView("Running", state);
    Expect(view.status == xp2p::ui::ClientRuntimeStatus::Ready, "proxy ready status without socks_ready");
    ExpectContains(view.summary, "Proxy: Ready", "proxy ready summary without socks_ready");
}

void TestRuntimeViewTunReadySplit() {
    const std::string json = R"({
  "tun_enabled": true,
  "mode": "split",
  "runtime": {
    "timestamp": "2026-01-01T00:00:00Z",
    "tun": { "name": "xp2p", "ipv4": "10.0.0.2", "prefix": 24, "oper_status": "Up", "dad_state": "Preferred", "ready": true },
    "routes": { "redirect_applied": true, "redirect_count": 3, "full_applied": false, "full_bypass_count": 0 }
  }
})";
    auto path = WriteTemp("client-tun.json", json);
    auto state = xp2p::ui::TryLoadClientStateFile(path.string());
    auto view = xp2p::ui::BuildClientRuntimeView("Running", state);
    Expect(view.status == xp2p::ui::ClientRuntimeStatus::Ready, "tun ready status");
    ExpectContains(view.summary, "Tun: Ready", "tun ready summary");
    ExpectContains(view.summary, "Routes: Split", "routes split summary");
    ExpectContains(view.detail, "split(3)", "routes count detail");
}

void TestModeLogicResolveClient() {
    auto mode1 = xp2p::ui::ResolveClientMode(false, "full", true, false);
    Expect(mode1 == xp2p::ui::ClientMode::Proxy, "resolve client proxy when tun disabled");
    auto mode2 = xp2p::ui::ResolveClientMode(true, "", true, false);
    Expect(mode2 == xp2p::ui::ClientMode::TunFull, "resolve client full by routes");
}

} // namespace

void RunRuntimeTests() {
    TestSimpleJsonBasics();
    TestRuntimeViewProxyReady();
    TestRuntimeViewProxyReadyWithoutSocksKey();
    TestRuntimeViewTunReadySplit();
    TestModeLogicResolveClient();
}
