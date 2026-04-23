#include "tray_app.h"

#include "logging.h"
#include "mode_logic.h"
#include "mode_manager.h"
#include "path_utils.h"
#include "runtime_state.h"
#include "runtime_view.h"
#include "service_manager.h"
#include "tray_app_internal.h"
#include "ui_logic.h"

#include <shellapi.h>

namespace xp2p::ui {

void TrayApp::RefreshStatus() {
    ServiceStatus client = QueryServiceStatus(internal::kServiceClient);
    ServiceStatus server = QueryServiceStatus(internal::kServiceServer);

    clientStatus_ = client.label;
    serverStatus_ = server.label;

    ModeManager mm;
    auto clientState = TryLoadClientStateFile(mm.GetClientStatePath());
    auto serverState = TryLoadServerStateFile(mm.GetServerStatePath());

    const ClientRuntimeView clientRuntime = BuildClientRuntimeView(clientStatus_, clientState);

    if (clientState.has_value()) {
        const bool routesFull = clientState->runtime.has_value() && clientState->runtime->routes.has_value() && clientState->runtime->routes->fullApplied;
        const bool routesRedirect =
            clientState->runtime.has_value() && clientState->runtime->routes.has_value() && clientState->runtime->routes->redirectApplied;
        const ClientMode mode = ResolveClientMode(clientState->tunEnabled, clientState->mode, routesFull, routesRedirect);
        currentClientMode_ = mode;
        clientModeLabel_ = FormatClientMode(mode);
        if (pendingClientMode_.has_value() && mode == *pendingClientMode_) {
            clientModePending_ = false;
            pendingClientMode_.reset();
        }
    } else {
        currentClientMode_.reset();
        clientModeLabel_.clear();
    }

    if (serverState.has_value()) {
        const ServerMode mode = ResolveServerMode(serverState->tunEnabled, serverState->mode);
        currentServerMode_ = mode;
        serverModeLabel_ = FormatServerMode(mode);
        if (pendingServerMode_.has_value() && mode == *pendingServerMode_) {
            serverModePending_ = false;
            pendingServerMode_.reset();
        }
    } else {
        currentServerMode_.reset();
        serverModeLabel_.clear();
    }

    UpdateTooltip();
    UpdateTrayIconState();
    LogStatusIfChanged();

    const UINT basePoll = internal::GetStatusPollMs();
    const bool fastPoll = busy_ || clientModePending_ || serverModePending_ || IsServicePending(clientStatus_) || IsServicePending(serverStatus_) ||
        (clientRuntime.status == ClientRuntimeStatus::Pending);
    EnsureStatusTimer(fastPoll ? 500u : basePoll);
}

void TrayApp::UpdateTooltip() {
    if (!trayAdded_) {
        return;
    }
    ModeManager mm;
    const auto clientState = TryLoadClientStateFile(mm.GetClientStatePath());
    const ClientRuntimeView runtime = BuildClientRuntimeView(clientStatus_, clientState);

    std::string text;
    if (runtime.summary.empty()) {
        text = "Client: " + clientStatus_ + "\r\nServer: " + serverStatus_;
    } else {
        text = "Client: " + clientStatus_ + " | " + runtime.summary + "\r\nServer: " + serverStatus_;
    }
    std::wstring tip = internal::ToWide(text);
    if (tip.size() > 63) {
        tip.resize(63);
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
    enum class IconState { Disabled, Enabled, Busy };
    IconState state = IconState::Disabled;
    if (busy_) {
        desired = iconBusy_ ? iconBusy_ : iconDisabled_;
        state = IconState::Busy;
    } else {
        ModeManager mm;
        auto clientState = TryLoadClientStateFile(mm.GetClientStatePath());
        ClientRuntimeView view = BuildClientRuntimeView(clientStatus_, clientState);
        if (view.status == ClientRuntimeStatus::Pending) {
            desired = iconBusy_ ? iconBusy_ : iconDisabled_;
            state = IconState::Busy;
        } else if (view.status == ClientRuntimeStatus::Ready) {
            desired = iconEnabled_ ? iconEnabled_ : iconDisabled_;
            state = IconState::Enabled;
        } else if (IsServiceRunning(clientStatus_) || IsServiceRunning(serverStatus_)) {
            desired = iconEnabled_ ? iconEnabled_ : iconDisabled_;
            state = IconState::Enabled;
        }
    }
    if (!desired) {
        desired = LoadIconW(instance_, MAKEINTRESOURCEW(internal::IDI_XP2P_APPICON));
    }
    if (desired == iconCurrent_) {
        return;
    }
    iconCurrent_ = desired;
    UpdateTrayIcon();
    switch (state) {
        case IconState::Busy:
            LogInfo("tray icon: busy");
            break;
        case IconState::Enabled:
            LogInfo("tray icon: enabled");
            break;
        default:
            LogInfo("tray icon: disabled");
            break;
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

void TrayApp::OpenLogsFolder() {
    std::wstring logPath = GetUiLogPath();
    std::wstring args = L"/select,\"" + logPath + L"\"";
    ShellExecuteW(nullptr, L"open", L"explorer.exe", args.c_str(), nullptr, SW_SHOWNORMAL);
}

}
