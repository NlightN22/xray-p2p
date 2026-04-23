#pragma once

#include <optional>
#include <string>

#include <windows.h>

namespace xp2p::ui {

enum class ClientMode;
enum class ServerMode;

class TrayApp final {
public:
    explicit TrayApp(HINSTANCE instance);
    int Run();

private:
    static LRESULT CALLBACK WndProc(HWND hwnd, UINT msg, WPARAM wparam, LPARAM lparam);
    LRESULT HandleMessage(HWND hwnd, UINT msg, WPARAM wparam, LPARAM lparam);

    void EnsureTrayIcon();
    void RemoveTrayIcon();
    void ShowContextMenu();
    void RefreshStatus();
    void EnsureStatusTimer(UINT intervalMs);
    void UpdateTooltip();
    void UpdateTrayIcon();
    void UpdateTrayIconState();
    void LogStatusIfChanged();
    void StartServiceAction(const wchar_t* serviceName, const char* serviceKey, int actionId);
    void RequestClientMode(int actionId, const std::string& tagOverride);
    void RequestServerMode(int actionId);
    void RequestShutdown();
    void StopAllServicesAndExit();
    void OpenLogsFolder();

    HINSTANCE instance_ = nullptr;
    HWND hwnd_ = nullptr;
    HICON iconCurrent_ = nullptr;
    HICON iconDisabled_ = nullptr;
    HICON iconEnabled_ = nullptr;
    HICON iconBusy_ = nullptr;
    bool trayAdded_ = false;
    bool busy_ = false;
    UINT statusPollMs_ = 0;

    std::string clientStatus_;
    std::string serverStatus_;
    std::string lastLoggedKey_;

    std::string clientModeLabel_;
    std::string serverModeLabel_;
    bool clientModePending_ = false;
    bool serverModePending_ = false;
    std::optional<ClientMode> currentClientMode_;
    std::optional<ServerMode> currentServerMode_;
    std::optional<ClientMode> pendingClientMode_;
    std::optional<ServerMode> pendingServerMode_;
};

}
