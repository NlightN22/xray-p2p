#include "tray_app.h"

#include "mode_logic.h"
#include "mode_manager.h"
#include "tray_app_internal.h"
#include "ui_logic.h"

namespace xp2p::ui {

void TrayApp::ShowContextMenu() {
    RefreshStatus();

    HMENU root = CreatePopupMenu();
    HMENU client = CreatePopupMenu();
    HMENU server = CreatePopupMenu();

    AppendMenuW(client, MF_STRING, internal::IDM_CLIENT_START, L"Start");
    AppendMenuW(client, MF_STRING, internal::IDM_CLIENT_STOP, L"Stop");
    AppendMenuW(client, MF_STRING, internal::IDM_CLIENT_RESTART, L"Restart");
    AppendMenuW(client, MF_SEPARATOR, 0, nullptr);
    AppendMenuW(client, MF_STRING, internal::IDM_CLIENT_STATUS, L"Status");
    AppendMenuW(client, MF_SEPARATOR, 0, nullptr);

    HMENU clientMode = CreatePopupMenu();
    AppendMenuW(clientMode, MF_STRING, internal::IDM_CLIENT_MODE_PROXY, L"Proxy");
    AppendMenuW(clientMode, MF_STRING, internal::IDM_CLIENT_MODE_SPLIT, L"Tun Split");

    ModeManager mm;
    auto tagState = mm.GetClientFullTunnelTagState();
    const bool canInlineFull = !tagState.existingTag.empty() || tagState.candidateTags.size() <= 1;
    HMENU clientFullTags = nullptr;
    if (canInlineFull) {
        AppendMenuW(clientMode, MF_STRING, internal::IDM_CLIENT_MODE_FULL, L"Tun Full");
    } else {
        clientFullTags = CreatePopupMenu();
        for (size_t i = 0; i < tagState.candidateTags.size(); i++) {
            const std::wstring label = internal::ToWide(tagState.candidateTags[i]);
            AppendMenuW(
                clientFullTags,
                MF_STRING,
                internal::IDM_CLIENT_MODE_FULL_TAG_BASE + static_cast<UINT>(i),
                label.c_str());
        }
        AppendMenuW(clientMode, MF_POPUP, reinterpret_cast<UINT_PTR>(clientFullTags), L"Tun Full");
    }

    std::wstring clientModeLabel = L"Mode: " + internal::ToWide(clientModeLabel_.empty() ? "Unknown" : clientModeLabel_);
    if (clientModePending_) {
        clientModeLabel = L"Mode: " + internal::ToWide(FormatPending(clientModeLabel_.empty() ? "Unknown" : clientModeLabel_));
    }
    AppendMenuW(client, MF_POPUP, reinterpret_cast<UINT_PTR>(clientMode), clientModeLabel.c_str());

    AppendMenuW(server, MF_STRING, internal::IDM_SERVER_START, L"Start");
    AppendMenuW(server, MF_STRING, internal::IDM_SERVER_STOP, L"Stop");
    AppendMenuW(server, MF_STRING, internal::IDM_SERVER_RESTART, L"Restart");
    AppendMenuW(server, MF_SEPARATOR, 0, nullptr);
    AppendMenuW(server, MF_STRING, internal::IDM_SERVER_STATUS, L"Status");
    AppendMenuW(server, MF_SEPARATOR, 0, nullptr);

    HMENU serverMode = CreatePopupMenu();
    AppendMenuW(serverMode, MF_STRING, internal::IDM_SERVER_MODE_PROXY, L"Proxy");
    AppendMenuW(serverMode, MF_STRING, internal::IDM_SERVER_MODE_TUN, L"Tun");
    AppendMenuW(serverMode, MF_SEPARATOR, 0, nullptr);
    AppendMenuW(serverMode, MF_STRING | MF_DISABLED | MF_GRAYED, 0, L"Split/Full modes are not supported on server.");
    std::wstring serverModeLabel = L"Mode: " + internal::ToWide(serverModeLabel_.empty() ? "Unknown" : serverModeLabel_);
    if (serverModePending_) {
        serverModeLabel = L"Mode: " + internal::ToWide(FormatPending(serverModeLabel_.empty() ? "Unknown" : serverModeLabel_));
    }
    AppendMenuW(server, MF_POPUP, reinterpret_cast<UINT_PTR>(serverMode), serverModeLabel.c_str());

    std::wstring clientLabel = L"Client service: " + internal::ToWide(clientStatus_);
    std::wstring serverLabel = L"Server service: " + internal::ToWide(serverStatus_);

    AppendMenuW(root, MF_POPUP, reinterpret_cast<UINT_PTR>(client), clientLabel.c_str());
    AppendMenuW(root, MF_POPUP, reinterpret_cast<UINT_PTR>(server), serverLabel.c_str());
    AppendMenuW(root, MF_SEPARATOR, 0, nullptr);
    AppendMenuW(root, MF_STRING, internal::IDM_OPEN_LOGS, L"Open logs folder");
    AppendMenuW(root, MF_SEPARATOR, 0, nullptr);
    AppendMenuW(root, MF_STRING, internal::IDM_EXIT, L"Exit");

    const ServiceButtons clientButtons = GetServiceButtons(clientStatus_, busy_);
    internal::SetMenuItemEnabled(client, internal::IDM_CLIENT_START, clientButtons.startEnabled);
    internal::SetMenuItemEnabled(client, internal::IDM_CLIENT_STOP, clientButtons.stopEnabled);
    internal::SetMenuItemEnabled(client, internal::IDM_CLIENT_RESTART, clientButtons.restartEnabled);

    const ServiceButtons serverButtons = GetServiceButtons(serverStatus_, busy_);
    internal::SetMenuItemEnabled(server, internal::IDM_SERVER_START, serverButtons.startEnabled);
    internal::SetMenuItemEnabled(server, internal::IDM_SERVER_STOP, serverButtons.stopEnabled);
    internal::SetMenuItemEnabled(server, internal::IDM_SERVER_RESTART, serverButtons.restartEnabled);

    const bool disableClientMode = clientModePending_ || busy_;
    internal::SetMenuItemEnabled(clientMode, internal::IDM_CLIENT_MODE_PROXY, !disableClientMode && clientModeLabel_ != "Proxy");
    internal::SetMenuItemEnabled(clientMode, internal::IDM_CLIENT_MODE_SPLIT, !disableClientMode && clientModeLabel_ != "Tun Split");
    if (canInlineFull) {
        internal::SetMenuItemEnabled(clientMode, internal::IDM_CLIENT_MODE_FULL, !disableClientMode && clientModeLabel_ != "Tun Full");
    } else if (clientFullTags) {
        for (size_t i = 0; i < tagState.candidateTags.size(); i++) {
            internal::SetMenuItemEnabled(
                clientFullTags,
                internal::IDM_CLIENT_MODE_FULL_TAG_BASE + static_cast<UINT>(i),
                !disableClientMode);
        }
    }

    const bool disableServerMode = serverModePending_ || busy_;
    internal::SetMenuItemEnabled(serverMode, internal::IDM_SERVER_MODE_PROXY, !disableServerMode && serverModeLabel_ != "Proxy");
    internal::SetMenuItemEnabled(serverMode, internal::IDM_SERVER_MODE_TUN, !disableServerMode && serverModeLabel_ != "Tun");

    POINT pt{};
    GetCursorPos(&pt);
    SetForegroundWindow(hwnd_);
    TrackPopupMenu(root, TPM_RIGHTBUTTON, pt.x, pt.y, 0, hwnd_, nullptr);

    DestroyMenu(root);
}

}

