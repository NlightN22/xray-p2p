#include "logging.h"

#include "path_utils.h"

#include <windows.h>

#include <cstdio>
#include <string>

namespace xp2p::ui {

static std::string FormatUtcTimestamp() {
    SYSTEMTIME st{};
    GetSystemTime(&st);
    char buf[64];
    std::snprintf(
        buf,
        sizeof(buf),
        "%04u-%02u-%02uT%02u:%02u:%02u.%03uZ",
        st.wYear,
        st.wMonth,
        st.wDay,
        st.wHour,
        st.wMinute,
        st.wSecond,
        st.wMilliseconds);
    return std::string(buf);
}

static void AppendLine(const std::string& line) {
    try {
        const std::wstring logPath = GetUiLogPath();
        const std::wstring logDir = GetXp2pLogsDir();
        EnsureDirectoryTree(logDir);

        HANDLE file = CreateFileW(
            logPath.c_str(),
            FILE_APPEND_DATA,
            FILE_SHARE_READ | FILE_SHARE_WRITE | FILE_SHARE_DELETE,
            nullptr,
            OPEN_ALWAYS,
            FILE_ATTRIBUTE_NORMAL,
            nullptr);
        if (file == INVALID_HANDLE_VALUE) {
            return;
        }

        std::string withNewline = line;
        withNewline.append("\r\n");
        DWORD written = 0;
        WriteFile(file, withNewline.data(), static_cast<DWORD>(withNewline.size()), &written, nullptr);
        CloseHandle(file);
    } catch (...) {
        return;
    }
}

static void LogWithLevel(const char* level, const std::string& message) {
    std::string line;
    line.reserve(message.size() + 64);
    line.append(FormatUtcTimestamp());
    line.push_back(' ');
    line.append(level);
    line.push_back(' ');
    line.append(message);
    AppendLine(line);
}

void LogInfo(const std::string& message) {
    LogWithLevel("INFO", message);
}

void LogWarn(const std::string& message) {
    LogWithLevel("WARN", message);
}

void LogError(const std::string& message) {
    LogWithLevel("ERROR", message);
}

}

