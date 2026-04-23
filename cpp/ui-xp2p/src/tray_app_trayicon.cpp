#include "tray_app.h"

#include "tray_app_internal.h"

#include <shellapi.h>

namespace xp2p::ui {

void TrayApp::EnsureTrayIcon() {
    if (trayAdded_) {
        return;
    }
    NOTIFYICONDATAW nid{};
    nid.cbSize = sizeof(nid);
    nid.hWnd = hwnd_;
    nid.uID = 1;
    nid.uFlags = NIF_MESSAGE | NIF_ICON | NIF_TIP;
    nid.uCallbackMessage = internal::WM_TRAYICON;
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

}

