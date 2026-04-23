#include "tray_app.h"

#include "logging.h"
#include "path_utils.h"
#include "png_icon.h"
#include "service_manager.h"
#include "ui_logic.h"

#include <shellapi.h>

namespace xp2p::ui {

namespace {

constexpr UINT WM_TRAYICON = WM_APP + 1;
constexpr UINT_PTR TIMER_STATUS = 1;
constexpr UINT WM_ACTION_DONE = WM_APP + 3;

constexpr int IDM_CLIENT_START = 1001;
constexpr int IDM_CLIENT_STOP = 1002;
constexpr int IDM_CLIENT_RESTART = 1003;
constexpr int IDM_CLIENT_STATUS = 1004;

constexpr int IDM_SERVER_START = 1011;
constexpr int IDM_SERVER_STOP = 1012;
constexpr int IDM_SERVER_RESTART = 1013;
constexpr int IDM_SERVER_STATUS = 1014;

constexpr int IDM_OPEN_LOGS = 1101;
constexpr int IDM_EXIT = 1102;

const wchar_t* kServiceClient = L"xp2p-client";
const wchar_t* kServiceServer = L"xp2p-server";

constexpr int IDI_XP2P_APPICON = 101;
constexpr int IDR_PNG_DISABLED = 201;
constexpr int IDR_PNG_ENABLED = 202;
constexpr int IDR_PNG_ENABLING = 203;

static void SetMenuItemEnabled(HMENU menu, UINT id, bool enabled) {
    EnableMenuItem(menu, id, MF_BYCOMMAND | (enabled ? MF_ENABLED : (MF_DISABLED | MF_GRAYED)));
}

static std::wstring ToWide(const std::string& s) {
    if (s.empty()) {
        return L"";
    }
    int needed = MultiByteToWideChar(CP_UTF8, 0, s.data(), static_cast<int>(s.size()), nullptr, 0);
    if (needed <= 0) {
        return L"";
    }
    std::wstring out(static_cast<size_t>(needed), L'\0');
    MultiByteToWideChar(CP_UTF8, 0, s.data(), static_cast<int>(s.size()), out.data(), needed);
    return out;
}

struct ServiceActionContext {
    HWND hwnd = nullptr;
    const wchar_t* serviceName = nullptr;
    const char* serviceKey = nullptr;
    int actionId = 0;
};

static DWORD WINAPI ServiceActionThreadProc(LPVOID param) {
    auto* ctx = static_cast<ServiceActionContext*>(param);
    if (!ctx) {
        return 0;
    }
    ServiceStatus result{};
    if (ctx->actionId == IDM_CLIENT_START || ctx->actionId == IDM_SERVER_START) {
        result = StartServiceAndWait(ctx->serviceName, 60000);
    } else if (ctx->actionId == IDM_CLIENT_STOP || ctx->actionId == IDM_SERVER_STOP) {
        result = StopServiceAndWait(ctx->serviceName, 60000);
    } else {
        result = RestartServiceAndWait(ctx->serviceName, 90000);
    }

    if (result.ok) {
        LogInfo(std::string("service action: ") + ctx->serviceKey + " ok status=" + result.label);
    } else {
        LogWarn(std::string("service action: ") + ctx->serviceKey + " failed code=" + std::to_string(result.error));
    }

    if (ctx->hwnd) {
        PostMessageW(ctx->hwnd, WM_ACTION_DONE, 0, 0);
    }
    delete ctx;
    return 0;
}

} // namespace

TrayApp::TrayApp(HINSTANCE instance) : instance_(instance) {}

int TrayApp::Run() {
    const wchar_t* className = L"xp2p_ui_tray_window";

    WNDCLASSEXW wc{};
    wc.cbSize = sizeof(wc);
    wc.lpfnWndProc = TrayApp::WndProc;
    wc.hInstance = instance_;
    wc.lpszClassName = className;
    wc.hIcon = LoadIconW(instance_, MAKEINTRESOURCEW(IDI_XP2P_APPICON));
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
    iconDisabled_ = CreateIconFromPngResource(instance_, IDR_PNG_DISABLED, iconSize);
    iconEnabled_ = CreateIconFromPngResource(instance_, IDR_PNG_ENABLED, iconSize);
    iconBusy_ = CreateIconFromPngResource(instance_, IDR_PNG_ENABLING, iconSize);
    iconCurrent_ = iconDisabled_ ? iconDisabled_ : LoadIconW(instance_, MAKEINTRESOURCEW(IDI_XP2P_APPICON));
    EnsureTrayIcon();
    RefreshStatus();
    SetTimer(hwnd_, TIMER_STATUS, 2000, nullptr);

    MSG msg{};
    while (GetMessageW(&msg, nullptr, 0, 0) > 0) {
        TranslateMessage(&msg);
        DispatchMessageW(&msg);
    }
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
        case WM_TRAYICON: {
            if (lparam == WM_RBUTTONUP || lparam == WM_CONTEXTMENU || lparam == WM_LBUTTONUP) {
                ShowContextMenu();
            }
            return 0;
        }
        case WM_TIMER: {
            if (wparam == TIMER_STATUS) {
                RefreshStatus();
            }
            return 0;
        }
        case WM_ACTION_DONE: {
            busy_ = false;
            RefreshStatus();
            return 0;
        }
        case WM_COMMAND: {
            const int id = LOWORD(wparam);
            switch (id) {
                case IDM_CLIENT_START:
                case IDM_CLIENT_STOP:
                case IDM_CLIENT_RESTART:
                    StartServiceAction(kServiceClient, "client", id);
                    return 0;
                case IDM_SERVER_START:
                case IDM_SERVER_STOP:
                case IDM_SERVER_RESTART:
                    StartServiceAction(kServiceServer, "server", id);
                    return 0;
                case IDM_CLIENT_STATUS:
                    ShowServiceStatusDialog(kServiceClient, L"Client service");
                    return 0;
                case IDM_SERVER_STATUS:
                    ShowServiceStatusDialog(kServiceServer, L"Server service");
                    return 0;
                case IDM_OPEN_LOGS:
                    OpenLogsFolder();
                    return 0;
                case IDM_EXIT:
                    DestroyWindow(hwnd_);
                    PostQuitMessage(0);
                    return 0;
                default:
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

void TrayApp::EnsureTrayIcon() {
    if (trayAdded_) {
        return;
    }
    NOTIFYICONDATAW nid{};
    nid.cbSize = sizeof(nid);
    nid.hWnd = hwnd_;
    nid.uID = 1;
    nid.uFlags = NIF_MESSAGE | NIF_ICON | NIF_TIP;
    nid.uCallbackMessage = WM_TRAYICON;
    nid.hIcon = iconCurrent_;
    wcscpy_s(nid.szTip, L"xp2p");
    trayAdded_ = Shell_NotifyIconW(NIM_ADD, &nid) != 0;
}

void TrayApp::RemoveTrayIcon() {
    if (!trayAdded_) {
        return;
    }
    NOTIFYICONDATAW nid{};
    nid.cbSize = sizeof(nid);
    nid.hWnd = hwnd_;
    nid.uID = 1;
    Shell_NotifyIconW(NIM_DELETE, &nid);
    trayAdded_ = false;
}

void TrayApp::ShowContextMenu() {
    RefreshStatus();

    HMENU root = CreatePopupMenu();
    HMENU client = CreatePopupMenu();
    HMENU server = CreatePopupMenu();

    AppendMenuW(client, MF_STRING, IDM_CLIENT_START, L"Start");
    AppendMenuW(client, MF_STRING, IDM_CLIENT_STOP, L"Stop");
    AppendMenuW(client, MF_STRING, IDM_CLIENT_RESTART, L"Restart");
    AppendMenuW(client, MF_SEPARATOR, 0, nullptr);
    AppendMenuW(client, MF_STRING, IDM_CLIENT_STATUS, L"Status");

    AppendMenuW(server, MF_STRING, IDM_SERVER_START, L"Start");
    AppendMenuW(server, MF_STRING, IDM_SERVER_STOP, L"Stop");
    AppendMenuW(server, MF_STRING, IDM_SERVER_RESTART, L"Restart");
    AppendMenuW(server, MF_SEPARATOR, 0, nullptr);
    AppendMenuW(server, MF_STRING, IDM_SERVER_STATUS, L"Status");

    std::wstring clientLabel = L"Client service: " + ToWide(clientStatus_);
    std::wstring serverLabel = L"Server service: " + ToWide(serverStatus_);

    AppendMenuW(root, MF_POPUP, reinterpret_cast<UINT_PTR>(client), clientLabel.c_str());
    AppendMenuW(root, MF_POPUP, reinterpret_cast<UINT_PTR>(server), serverLabel.c_str());
    AppendMenuW(root, MF_SEPARATOR, 0, nullptr);
    AppendMenuW(root, MF_STRING, IDM_OPEN_LOGS, L"Open logs folder");
    AppendMenuW(root, MF_SEPARATOR, 0, nullptr);
    AppendMenuW(root, MF_STRING, IDM_EXIT, L"Exit");

    const ServiceButtons clientButtons = GetServiceButtons(clientStatus_, busy_);
    SetMenuItemEnabled(client, IDM_CLIENT_START, clientButtons.startEnabled);
    SetMenuItemEnabled(client, IDM_CLIENT_STOP, clientButtons.stopEnabled);
    SetMenuItemEnabled(client, IDM_CLIENT_RESTART, clientButtons.restartEnabled);

    const ServiceButtons serverButtons = GetServiceButtons(serverStatus_, busy_);
    SetMenuItemEnabled(server, IDM_SERVER_START, serverButtons.startEnabled);
    SetMenuItemEnabled(server, IDM_SERVER_STOP, serverButtons.stopEnabled);
    SetMenuItemEnabled(server, IDM_SERVER_RESTART, serverButtons.restartEnabled);

    POINT pt{};
    GetCursorPos(&pt);
    SetForegroundWindow(hwnd_);
    TrackPopupMenu(root, TPM_RIGHTBUTTON, pt.x, pt.y, 0, hwnd_, nullptr);

    DestroyMenu(root);
    DestroyMenu(client);
    DestroyMenu(server);
}

void TrayApp::RefreshStatus() {
    ServiceStatus client = QueryServiceStatus(kServiceClient);
    ServiceStatus server = QueryServiceStatus(kServiceServer);

    clientStatus_ = client.label;
    serverStatus_ = server.label;
    UpdateTooltip();
    UpdateTrayIconState();
    LogStatusIfChanged();
}

void TrayApp::UpdateTooltip() {
    if (!trayAdded_) {
        return;
    }
    std::wstring tip = ToWide(BuildTrayTooltip(clientStatus_, serverStatus_, busy_));
    if (tip.size() >= 127) {
        tip.resize(126);
    }

    NOTIFYICONDATAW nid{};
    nid.cbSize = sizeof(nid);
    nid.hWnd = hwnd_;
    nid.uID = 1;
    nid.uFlags = NIF_TIP;
    wcscpy_s(nid.szTip, tip.c_str());
    Shell_NotifyIconW(NIM_MODIFY, &nid);
}

void TrayApp::UpdateTrayIcon() {
    if (!trayAdded_) {
        return;
    }
    NOTIFYICONDATAW nid{};
    nid.cbSize = sizeof(nid);
    nid.hWnd = hwnd_;
    nid.uID = 1;
    nid.uFlags = NIF_ICON;
    nid.hIcon = iconCurrent_;
    Shell_NotifyIconW(NIM_MODIFY, &nid);
}

void TrayApp::UpdateTrayIconState() {
    HICON desired = iconDisabled_;
    if (busy_) {
        desired = iconBusy_ ? iconBusy_ : iconDisabled_;
    } else if (IsServiceRunning(clientStatus_) || IsServiceRunning(serverStatus_)) {
        desired = iconEnabled_ ? iconEnabled_ : iconDisabled_;
    }
    if (!desired) {
        desired = LoadIconW(instance_, MAKEINTRESOURCEW(IDI_XP2P_APPICON));
    }
    if (desired == iconCurrent_) {
        return;
    }
    iconCurrent_ = desired;
    UpdateTrayIcon();
    if (busy_) {
        LogInfo("tray icon: busy");
    } else if (IsServiceRunning(clientStatus_) || IsServiceRunning(serverStatus_)) {
        LogInfo("tray icon: enabled");
    } else {
        LogInfo("tray icon: disabled");
    }
}

void TrayApp::LogStatusIfChanged() {
    std::string key = clientStatus_ + "|" + serverStatus_ + "|" + (busy_ ? "1" : "0");
    if (key == lastLoggedKey_) {
        return;
    }
    lastLoggedKey_ = key;
    LogInfo(BuildTrayStatusLogLine(clientStatus_, serverStatus_, busy_));
}

void TrayApp::StartServiceAction(const wchar_t* serviceName, const char* serviceKey, int actionId) {
    if (busy_) {
        return;
    }
    busy_ = true;
    LogInfo(std::string("tray action: ") + serviceKey);
    UpdateTooltip();
    UpdateTrayIconState();
    LogStatusIfChanged();

    auto* ctx = new ServiceActionContext();
    ctx->hwnd = hwnd_;
    ctx->serviceName = serviceName;
    ctx->serviceKey = serviceKey;
    ctx->actionId = actionId;
    HANDLE thread = CreateThread(nullptr, 0, ServiceActionThreadProc, ctx, 0, nullptr);
    if (!thread) {
        delete ctx;
        busy_ = false;
        RefreshStatus();
        LogError(std::string("service action: ") + serviceKey + " failed to start worker");
        return;
    }
    CloseHandle(thread);
}

void TrayApp::ShowServiceStatusDialog(const wchar_t* serviceName, const wchar_t* title) {
    ServiceStatus s = QueryServiceStatus(serviceName);
    std::wstring body = L"Status: " + ToWide(s.label);
    MessageBoxW(hwnd_, body.c_str(), title, MB_OK | MB_ICONINFORMATION);
}

void TrayApp::OpenLogsFolder() {
    std::wstring dir = GetXp2pLogsDir();
    EnsureDirectoryTree(dir);
    ShellExecuteW(nullptr, L"open", dir.c_str(), nullptr, nullptr, SW_SHOWNORMAL);
}

}
