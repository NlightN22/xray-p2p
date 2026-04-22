#include "tray_app.h"

#include "logging.h"
#include "path_utils.h"
#include "service_manager.h"

#include <shellapi.h>

namespace xp2p::ui {

namespace {

constexpr UINT WM_TRAYICON = WM_APP + 1;
constexpr UINT_PTR TIMER_STATUS = 1;

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

} // namespace

TrayApp::TrayApp(HINSTANCE instance) : instance_(instance) {}

int TrayApp::Run() {
    const wchar_t* className = L"xp2p_ui_tray_window";

    WNDCLASSEXW wc{};
    wc.cbSize = sizeof(wc);
    wc.lpfnWndProc = TrayApp::WndProc;
    wc.hInstance = instance_;
    wc.lpszClassName = className;
    wc.hIcon = LoadIconW(instance_, MAKEINTRESOURCEW(101));
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

    icon_ = LoadIconW(instance_, MAKEINTRESOURCEW(101));
    EnsureTrayIcon();
    RefreshStatus();
    SetTimer(hwnd_, TIMER_STATUS, 2000, nullptr);

    MSG msg{};
    while (GetMessageW(&msg, nullptr, 0, 0) > 0) {
        TranslateMessage(&msg);
        DispatchMessageW(&msg);
    }
    RemoveTrayIcon();
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
    nid.hIcon = icon_;
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

    const bool clientRunning = clientStatus_ == "Running";
    const bool clientStopped = clientStatus_ == "Stopped";
    SetMenuItemEnabled(client, IDM_CLIENT_START, !busy_ && !clientRunning);
    SetMenuItemEnabled(client, IDM_CLIENT_STOP, !busy_ && !clientStopped);
    SetMenuItemEnabled(client, IDM_CLIENT_RESTART, !busy_);

    const bool serverRunning = serverStatus_ == "Running";
    const bool serverStopped = serverStatus_ == "Stopped";
    SetMenuItemEnabled(server, IDM_SERVER_START, !busy_ && !serverRunning);
    SetMenuItemEnabled(server, IDM_SERVER_STOP, !busy_ && !serverStopped);
    SetMenuItemEnabled(server, IDM_SERVER_RESTART, !busy_);

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
    LogStatusIfChanged();
}

void TrayApp::UpdateTooltip() {
    if (!trayAdded_) {
        return;
    }
    std::wstring tip = L"xp2p";
    tip += L" | client: " + ToWide(clientStatus_);
    tip += L" | server: " + ToWide(serverStatus_);
    if (busy_) {
        tip += L" | busy";
    }
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

void TrayApp::LogStatusIfChanged() {
    std::string key = clientStatus_ + "|" + serverStatus_ + "|" + (busy_ ? "1" : "0");
    if (key == lastLoggedKey_) {
        return;
    }
    lastLoggedKey_ = key;
    LogInfo("tray status: client=" + clientStatus_ + " server=" + serverStatus_ + " busy=" + (busy_ ? "1" : "0"));
}

void TrayApp::StartServiceAction(const wchar_t* serviceName, const char* serviceKey, int actionId) {
    if (busy_) {
        return;
    }
    busy_ = true;
    LogInfo(std::string("tray action: ") + serviceKey);
    UpdateTooltip();
    LogStatusIfChanged();

    ServiceStatus result{};
    if (actionId == IDM_CLIENT_START || actionId == IDM_SERVER_START) {
        result = StartServiceAndWait(serviceName, 20000);
    } else if (actionId == IDM_CLIENT_STOP || actionId == IDM_SERVER_STOP) {
        result = StopServiceAndWait(serviceName, 20000);
    } else {
        result = RestartServiceAndWait(serviceName, 30000);
    }

    if (result.ok) {
        LogInfo(std::string("service action: ") + serviceKey + " ok status=" + result.label);
    } else {
        LogWarn(std::string("service action: ") + serviceKey + " failed code=" + std::to_string(result.error));
    }

    busy_ = false;
    RefreshStatus();
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
