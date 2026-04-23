#include "tray_app.h"

#include "logging.h"
#include "mode_manager.h"
#include "service_manager.h"
#include "tray_app_internal.h"
#include "ui_logic.h"

namespace xp2p::ui {

namespace {

struct ServiceActionContext {
    HWND hwnd = nullptr;
    const wchar_t* serviceName = nullptr;
    const char* serviceKey = nullptr;
    int actionId = 0;
};

DWORD WINAPI ServiceActionThreadProc(LPVOID param) {
    auto* ctx = static_cast<ServiceActionContext*>(param);
    if (!ctx) {
        return 0;
    }
    ServiceStatus result{};
    if (ctx->actionId == internal::IDM_CLIENT_START || ctx->actionId == internal::IDM_SERVER_START) {
        result = StartServiceAndWait(ctx->serviceName, 60000);
    } else if (ctx->actionId == internal::IDM_CLIENT_STOP || ctx->actionId == internal::IDM_SERVER_STOP) {
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
        PostMessageW(ctx->hwnd, internal::WM_ACTION_DONE, 0, 0);
    }
    delete ctx;
    return 0;
}

struct ModeActionContext {
    HWND hwnd = nullptr;
    bool isClient = false;
    int modeActionId = 0;
    std::string tagOverride;
};

DWORD WINAPI ModeActionThreadProc(LPVOID param) {
    auto* ctx = static_cast<ModeActionContext*>(param);
    if (!ctx) {
        return 0;
    }
    ModeManager manager([](const std::string& m) { LogInfo(m); });
    OperationResult result{};
    if (ctx->isClient) {
        if (ctx->modeActionId == internal::IDM_CLIENT_MODE_PROXY) {
            result = manager.ApplyClientMode(ClientMode::Proxy);
        } else if (ctx->modeActionId == internal::IDM_CLIENT_MODE_SPLIT) {
            result = manager.ApplyClientMode(ClientMode::TunSplit);
        } else {
            if (!ctx->tagOverride.empty()) {
                result = manager.ApplyClientMode(ClientMode::TunFull, ctx->tagOverride);
            } else {
                result = manager.ApplyClientMode(ClientMode::TunFull);
            }
        }
    } else {
        if (ctx->modeActionId == internal::IDM_SERVER_MODE_TUN) {
            result = manager.ApplyServerMode(ServerMode::Tun);
        } else {
            result = manager.ApplyServerMode(ServerMode::Proxy);
        }
    }

    if (result.success) {
        LogInfo(result.message);
    } else {
        LogWarn(result.message);
    }

    if (ctx->hwnd) {
        PostMessageW(ctx->hwnd, internal::WM_MODE_DONE, ctx->isClient ? 1 : 0, ctx->modeActionId);
    }
    delete ctx;
    return 0;
}

DWORD WINAPI StopAllThreadProc(LPVOID param) {
    HWND hwnd = static_cast<HWND>(param);
    StopServiceAndWait(internal::kServiceClient, 60000);
    StopServiceAndWait(internal::kServiceServer, 60000);
    if (hwnd) {
        PostMessageW(hwnd, internal::WM_STOPALL_DONE, 0, 0);
    }
    return 0;
}

} // namespace

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

void TrayApp::RequestClientMode(int actionId, const std::string& tagOverride) {
    if (busy_ || clientModePending_) {
        return;
    }
    busy_ = true;
    pendingClientModeLabel_ = (actionId == internal::IDM_CLIENT_MODE_PROXY)
        ? "Proxy"
        : (actionId == internal::IDM_CLIENT_MODE_SPLIT) ? "Tun Split" : "Tun Full";
    clientModePending_ = true;
    UpdateTrayIconState();
    UpdateTooltip();
    LogStatusIfChanged();

    auto* ctx = new ModeActionContext();
    ctx->hwnd = hwnd_;
    ctx->isClient = true;
    ctx->modeActionId = actionId;
    ctx->tagOverride = tagOverride;
    HANDLE thread = CreateThread(nullptr, 0, ModeActionThreadProc, ctx, 0, nullptr);
    if (!thread) {
        delete ctx;
        busy_ = false;
        clientModePending_ = false;
        RefreshStatus();
        LogError("client mode change failed to start worker");
        return;
    }
    CloseHandle(thread);
}

void TrayApp::RequestServerMode(int actionId) {
    if (busy_ || serverModePending_) {
        return;
    }
    busy_ = true;
    pendingServerModeLabel_ = (actionId == internal::IDM_SERVER_MODE_TUN) ? "Tun" : "Proxy";
    serverModePending_ = true;
    UpdateTrayIconState();
    UpdateTooltip();
    LogStatusIfChanged();

    auto* ctx = new ModeActionContext();
    ctx->hwnd = hwnd_;
    ctx->isClient = false;
    ctx->modeActionId = actionId;
    HANDLE thread = CreateThread(nullptr, 0, ModeActionThreadProc, ctx, 0, nullptr);
    if (!thread) {
        delete ctx;
        busy_ = false;
        serverModePending_ = false;
        RefreshStatus();
        LogError("server mode change failed to start worker");
        return;
    }
    CloseHandle(thread);
}

void TrayApp::RequestShutdown() {
    const bool shouldPrompt = IsServiceRunning(clientStatus_) || IsServiceRunning(serverStatus_) || IsServicePending(clientStatus_) || IsServicePending(serverStatus_);
    if (!shouldPrompt) {
        DestroyWindow(hwnd_);
        PostQuitMessage(0);
        return;
    }
    const int result = MessageBoxW(hwnd_, L"Stop all services?", L"xp2p", MB_YESNO | MB_ICONQUESTION);
    if (result == IDYES) {
        StopAllServicesAndExit();
        return;
    }
    DestroyWindow(hwnd_);
    PostQuitMessage(0);
}

void TrayApp::StopAllServicesAndExit() {
    if (busy_) {
        return;
    }
    busy_ = true;
    UpdateTrayIconState();
    UpdateTooltip();
    HANDLE thread = CreateThread(nullptr, 0, StopAllThreadProc, hwnd_, 0, nullptr);
    if (thread) {
        CloseHandle(thread);
    }
}

}
