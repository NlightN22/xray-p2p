#pragma once

#include <string>

#include <windows.h>

namespace xp2p::ui {

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
    void UpdateTooltip();
    void LogStatusIfChanged();
    void StartServiceAction(const wchar_t* serviceName, const char* serviceKey, int actionId);
    void ShowServiceStatusDialog(const wchar_t* serviceName, const wchar_t* title);
    void OpenLogsFolder();

    HINSTANCE instance_ = nullptr;
    HWND hwnd_ = nullptr;
    HICON icon_ = nullptr;
    bool trayAdded_ = false;
    bool busy_ = false;

    std::string clientStatus_;
    std::string serverStatus_;
    std::string lastLoggedKey_;
};

}

