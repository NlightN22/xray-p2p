#pragma once

#include <windows.h>

#include <cstdlib>
#include <string>

namespace xp2p::ui::internal {

constexpr UINT WM_TRAYICON = WM_APP + 1;
constexpr UINT_PTR TIMER_STATUS = 1;
constexpr UINT WM_ACTION_DONE = WM_APP + 3;
constexpr UINT WM_MODE_DONE = WM_APP + 4;
constexpr UINT WM_STOPALL_DONE = WM_APP + 5;

constexpr int IDM_CLIENT_START = 1001;
constexpr int IDM_CLIENT_STOP = 1002;
constexpr int IDM_CLIENT_RESTART = 1003;

constexpr int IDM_SERVER_START = 1011;
constexpr int IDM_SERVER_STOP = 1012;
constexpr int IDM_SERVER_RESTART = 1013;

constexpr int IDM_OPEN_LOGS = 1101;
constexpr int IDM_EXIT = 1102;

constexpr int IDM_CLIENT_MODE_PROXY = 2001;
constexpr int IDM_CLIENT_MODE_SPLIT = 2002;
constexpr int IDM_CLIENT_MODE_FULL = 2003;
constexpr int IDM_CLIENT_MODE_FULL_TAG_BASE = 2100;

constexpr int IDM_SERVER_MODE_PROXY = 2201;
constexpr int IDM_SERVER_MODE_TUN = 2202;

inline const wchar_t* kServiceClient = L"xp2p-client";
inline const wchar_t* kServiceServer = L"xp2p-server";

constexpr int IDI_XP2P_APPICON = 101;
constexpr int IDR_PNG_DISABLED = 201;
constexpr int IDR_PNG_ENABLED = 202;
constexpr int IDR_PNG_ENABLING = 203;

inline void SetMenuItemEnabled(HMENU menu, UINT id, bool enabled) {
    EnableMenuItem(menu, id, MF_BYCOMMAND | (enabled ? MF_ENABLED : (MF_DISABLED | MF_GRAYED)));
}

inline std::wstring ToWide(const std::string& s) {
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

inline UINT GetStatusPollMs() {
    char buf[64];
    DWORD got = GetEnvironmentVariableA("XP2P_UI_STATUS_POLL_SECONDS", buf, static_cast<DWORD>(sizeof(buf)));
    if (got > 0 && got < sizeof(buf)) {
        buf[got] = '\0';
        int seconds = std::atoi(buf);
        if (seconds > 0 && seconds <= 60) {
            return static_cast<UINT>(seconds) * 1000u;
        }
    }
    return 5000;
}

}
