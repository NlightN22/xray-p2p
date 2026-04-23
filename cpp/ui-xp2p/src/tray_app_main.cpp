#include "tray_app.h"

#include "logging.h"
#include "mode_manager.h"
#include "png_icon.h"
#include "tray_app_internal.h"

#include <windows.h>

namespace xp2p::ui {

TrayApp::TrayApp(HINSTANCE instance) : instance_(instance) {}

int TrayApp::Run() {
    const wchar_t* className = L"xp2p_ui_tray_window";
    LogInfo("ui-xp2p starting.");

    WNDCLASSEXW wc{};
    wc.cbSize = sizeof(wc);
    wc.lpfnWndProc = TrayApp::WndProc;
    wc.hInstance = instance_;
    wc.lpszClassName = className;
    wc.hIcon = LoadIconW(instance_, MAKEINTRESOURCEW(internal::IDI_XP2P_APPICON));
    wc.hCursor = LoadCursorW(nullptr, IDC_ARROW);

    if (!RegisterClassExW(&wc)) {
        return 2;
    }

    hwnd_ = CreateWindowExW(
        0,
        className,
        L"ui-xp2p",
        WS_OVERLAPPED,
        CW_USEDEFAULT,
        CW_USEDEFAULT,
        CW_USEDEFAULT,
        CW_USEDEFAULT,
        nullptr,
        nullptr,
        instance_,
        this);
    if (!hwnd_) {
        return 3;
    }

    const int iconSize = GetSystemMetrics(SM_CXSMICON);
    iconDisabled_ = CreateIconFromPngResource(instance_, internal::IDR_PNG_DISABLED, iconSize);
    iconEnabled_ = CreateIconFromPngResource(instance_, internal::IDR_PNG_ENABLED, iconSize);
    iconBusy_ = CreateIconFromPngResource(instance_, internal::IDR_PNG_ENABLING, iconSize);
    iconCurrent_ = iconDisabled_ ? iconDisabled_ : LoadIconW(instance_, MAKEINTRESOURCEW(internal::IDI_XP2P_APPICON));

    EnsureTrayIcon();
    RefreshStatus();
    SetTimer(hwnd_, internal::TIMER_STATUS, internal::GetStatusPollMs(), nullptr);

    MSG msg{};
    while (GetMessageW(&msg, nullptr, 0, 0) > 0) {
        TranslateMessage(&msg);
        DispatchMessageW(&msg);
    }

    LogInfo("ui-xp2p exiting.");
    RemoveTrayIcon();
    if (iconDisabled_) {
        DestroyIcon(iconDisabled_);
        iconDisabled_ = nullptr;
    }
    if (iconEnabled_) {
        DestroyIcon(iconEnabled_);
        iconEnabled_ = nullptr;
    }
    if (iconBusy_) {
        DestroyIcon(iconBusy_);
        iconBusy_ = nullptr;
    }
    iconCurrent_ = nullptr;
    return 0;
}

LRESULT CALLBACK TrayApp::WndProc(HWND hwnd, UINT msg, WPARAM wparam, LPARAM lparam) {
    auto* self = reinterpret_cast<TrayApp*>(GetWindowLongPtrW(hwnd, GWLP_USERDATA));
    if (msg == WM_NCCREATE) {
        auto* cs = reinterpret_cast<CREATESTRUCTW*>(lparam);
        self = reinterpret_cast<TrayApp*>(cs->lpCreateParams);
        SetWindowLongPtrW(hwnd, GWLP_USERDATA, reinterpret_cast<LONG_PTR>(self));
    }
    if (self) {
        return self->HandleMessage(hwnd, msg, wparam, lparam);
    }
    return DefWindowProcW(hwnd, msg, wparam, lparam);
}

LRESULT TrayApp::HandleMessage(HWND hwnd, UINT msg, WPARAM wparam, LPARAM lparam) {
    switch (msg) {
        case internal::WM_TRAYICON: {
            if (lparam == WM_RBUTTONUP || lparam == WM_CONTEXTMENU || lparam == WM_LBUTTONUP) {
                ShowContextMenu();
            }
            return 0;
        }
        case WM_TIMER: {
            if (wparam == internal::TIMER_STATUS) {
                RefreshStatus();
            }
            return 0;
        }
        case internal::WM_ACTION_DONE:
        case internal::WM_MODE_DONE: {
            busy_ = false;
            RefreshStatus();
            return 0;
        }
        case internal::WM_STOPALL_DONE: {
            DestroyWindow(hwnd_);
            PostQuitMessage(0);
            return 0;
        }
        case WM_COMMAND: {
            const int id = LOWORD(wparam);
            switch (id) {
                case internal::IDM_CLIENT_START:
                case internal::IDM_CLIENT_STOP:
                case internal::IDM_CLIENT_RESTART:
                    StartServiceAction(internal::kServiceClient, "client", id);
                    return 0;
                case internal::IDM_SERVER_START:
                case internal::IDM_SERVER_STOP:
                case internal::IDM_SERVER_RESTART:
                    StartServiceAction(internal::kServiceServer, "server", id);
                    return 0;
                case internal::IDM_CLIENT_STATUS:
                    ShowServiceStatusDialog(internal::kServiceClient, L"Client service");
                    return 0;
                case internal::IDM_SERVER_STATUS:
                    ShowServiceStatusDialog(internal::kServiceServer, L"Server service");
                    return 0;
                case internal::IDM_OPEN_LOGS:
                    OpenLogsFolder();
                    return 0;
                case internal::IDM_EXIT:
                    RequestShutdown();
                    return 0;
                case internal::IDM_CLIENT_MODE_PROXY:
                case internal::IDM_CLIENT_MODE_SPLIT:
                case internal::IDM_CLIENT_MODE_FULL:
                    RequestClientMode(id, "");
                    return 0;
                case internal::IDM_SERVER_MODE_PROXY:
                case internal::IDM_SERVER_MODE_TUN:
                    RequestServerMode(id);
                    return 0;
                default:
                    if (id >= internal::IDM_CLIENT_MODE_FULL_TAG_BASE && id < internal::IDM_CLIENT_MODE_FULL_TAG_BASE + 200) {
                        const int idx = id - internal::IDM_CLIENT_MODE_FULL_TAG_BASE;
                        ModeManager mm;
                        auto tags = mm.GetClientFullTunnelTagState().candidateTags;
                        if (idx >= 0 && idx < static_cast<int>(tags.size())) {
                            RequestClientMode(internal::IDM_CLIENT_MODE_FULL, tags[static_cast<size_t>(idx)]);
                        }
                        return 0;
                    }
                    return 0;
            }
        }
        case WM_DESTROY:
            PostQuitMessage(0);
            return 0;
        default:
            return DefWindowProcW(hwnd, msg, wparam, lparam);
    }
}

}

