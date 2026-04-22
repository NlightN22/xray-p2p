#include "../src/ui_logic.h"

#include <cstdlib>
#include <iostream>
#include <string>

namespace {

int failures = 0;

void Expect(bool cond, const char* message) {
    if (cond) {
        return;
    }
    failures++;
    std::cerr << "FAIL: " << message << "\n";
}

void ExpectEq(const std::string& got, const std::string& want, const char* message) {
    if (got == want) {
        return;
    }
    failures++;
    std::cerr << "FAIL: " << message << "\n";
    std::cerr << "  got : " << got << "\n";
    std::cerr << "  want: " << want << "\n";
}

void TestServiceStatusPredicates() {
    Expect(xp2p::ui::IsServiceRunning("Running"), "IsServiceRunning(Running)");
    Expect(xp2p::ui::IsServiceRunning("running"), "IsServiceRunning(running)");
    Expect(!xp2p::ui::IsServiceRunning("Stopped"), "IsServiceRunning(Stopped)=false");

    Expect(xp2p::ui::IsServiceStopped("Stopped"), "IsServiceStopped(Stopped)");
    Expect(xp2p::ui::IsServiceStopped("stopped"), "IsServiceStopped(stopped)");
    Expect(!xp2p::ui::IsServiceStopped("Running"), "IsServiceStopped(Running)=false");

    Expect(xp2p::ui::IsServicePending("StartPending"), "IsServicePending(StartPending)");
    Expect(xp2p::ui::IsServicePending("StopPending"), "IsServicePending(StopPending)");
    Expect(xp2p::ui::IsServicePending("PausePending"), "IsServicePending(PausePending)");
    Expect(xp2p::ui::IsServicePending("ContinuePending"), "IsServicePending(ContinuePending)");
    Expect(!xp2p::ui::IsServicePending("Running"), "IsServicePending(Running)=false");
}

void TestButtons() {
    {
        auto b = xp2p::ui::GetServiceButtons("Running", false);
        Expect(!b.startEnabled, "buttons running: start disabled");
        Expect(b.stopEnabled, "buttons running: stop enabled");
        Expect(b.restartEnabled, "buttons running: restart enabled");
    }
    {
        auto b = xp2p::ui::GetServiceButtons("Stopped", false);
        Expect(b.startEnabled, "buttons stopped: start enabled");
        Expect(!b.stopEnabled, "buttons stopped: stop disabled");
        Expect(b.restartEnabled, "buttons stopped: restart enabled");
    }
    {
        auto b = xp2p::ui::GetServiceButtons("StartPending", false);
        Expect(!b.startEnabled && !b.stopEnabled && !b.restartEnabled, "buttons pending: all disabled");
    }
    {
        auto b = xp2p::ui::GetServiceButtons("Unknown", false);
        Expect(b.startEnabled && b.stopEnabled && b.restartEnabled, "buttons unknown: all enabled");
    }
    {
        auto b = xp2p::ui::GetServiceButtons("Running", true);
        Expect(!b.startEnabled && !b.stopEnabled && !b.restartEnabled, "buttons busy: all disabled");
    }
}

void TestTooltipAndLogLine() {
    const std::string tip = xp2p::ui::BuildTrayTooltip("Running", "Stopped", true);
    Expect(tip.find("xp2p") != std::string::npos, "tooltip contains xp2p");
    Expect(tip.find("client: Running") != std::string::npos, "tooltip contains client status");
    Expect(tip.find("server: Stopped") != std::string::npos, "tooltip contains server status");
    Expect(tip.find("busy") != std::string::npos, "tooltip contains busy");

    const std::string tip2 = xp2p::ui::BuildTrayTooltip("Running", "Stopped", false);
    Expect(tip2.find("busy") == std::string::npos, "tooltip without busy");

    const std::string log = xp2p::ui::BuildTrayStatusLogLine("Running", "Stopped", false);
    ExpectEq(log, "tray status: client=Running server=Stopped busy=0", "log line format");
}

} // namespace

int main() {
    TestServiceStatusPredicates();
    TestButtons();
    TestTooltipAndLogLine();

    if (failures > 0) {
        std::cerr << failures << " test(s) failed.\n";
        return 1;
    }
    std::cout << "OK\n";
    return 0;
}
