#include "platform.h"

#include <windows.h>

#include <filesystem>
#include <fstream>
#include <string>
#include <vector>

namespace xp2p::ui {

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

static std::string ToUtf8(const std::wstring& ws) {
    if (ws.empty()) {
        return "";
    }
    int needed = WideCharToMultiByte(CP_UTF8, 0, ws.data(), static_cast<int>(ws.size()), nullptr, 0, nullptr, nullptr);
    if (needed <= 0) {
        return "";
    }
    std::string out(static_cast<size_t>(needed), '\0');
    WideCharToMultiByte(CP_UTF8, 0, ws.data(), static_cast<int>(ws.size()), out.data(), needed, nullptr, nullptr);
    return out;
}

std::string GetEnv(const std::string& key) {
    std::wstring wkey = ToWide(key);
    DWORD needed = GetEnvironmentVariableW(wkey.c_str(), nullptr, 0);
    if (needed == 0) {
        return "";
    }
    std::wstring buf(static_cast<size_t>(needed), L'\0');
    DWORD got = GetEnvironmentVariableW(wkey.c_str(), buf.data(), needed);
    if (got == 0) {
        return "";
    }
    if (!buf.empty() && buf.back() == L'\0') {
        buf.pop_back();
    }
    return ToUtf8(buf);
}

std::string GetUserNameForAudit() {
    wchar_t buf[256];
    DWORD sz = static_cast<DWORD>(std::size(buf));
    if (!GetUserNameW(buf, &sz)) {
        return "unknown";
    }
    std::wstring ws(buf);
    return ToUtf8(ws);
}

std::string GetCommandLineForAudit() {
    const wchar_t* cmd = GetCommandLineW();
    if (!cmd) {
        return "";
    }
    return ToUtf8(cmd);
}

bool FileExists(const std::string& path) {
    std::error_code ec;
    return std::filesystem::exists(std::filesystem::path(ToWide(path)), ec);
}

std::vector<unsigned char> ReadFileBytesOrEmpty(const std::string& path) {
    std::ifstream f(std::filesystem::path(ToWide(path)), std::ios::binary);
    if (!f) {
        return {};
    }
    std::vector<unsigned char> data((std::istreambuf_iterator<char>(f)), std::istreambuf_iterator<char>());
    return data;
}

bool EnsureDirForFile(const std::string& path) {
    std::error_code ec;
    std::filesystem::path p(ToWide(path));
    auto dir = p.parent_path();
    if (dir.empty()) {
        return true;
    }
    std::filesystem::create_directories(dir, ec);
    return !ec;
}

bool WriteFileAtomic(const std::string& path, const std::vector<unsigned char>& data) {
    if (!EnsureDirForFile(path)) {
        return false;
    }
    std::filesystem::path p(ToWide(path));
    std::filesystem::path dir = p.parent_path();
    const std::wstring tmpName = L".tmp-ui-xp2p";
    std::filesystem::path tmp = dir / tmpName;

    {
        std::ofstream out(tmp, std::ios::binary | std::ios::trunc);
        if (!out) {
            return false;
        }
        out.write(reinterpret_cast<const char*>(data.data()), static_cast<std::streamsize>(data.size()));
        out.flush();
    }

    const std::wstring wtmp = tmp.wstring();
    const std::wstring wdst = p.wstring();
    if (!MoveFileExW(wtmp.c_str(), wdst.c_str(), MOVEFILE_REPLACE_EXISTING | MOVEFILE_WRITE_THROUGH)) {
        DeleteFileW(wtmp.c_str());
        return false;
    }
    return true;
}

bool AppendFileTextUtf8(const std::string& path, const std::string& content) {
    if (!EnsureDirForFile(path)) {
        return false;
    }
    std::ofstream out(std::filesystem::path(ToWide(path)), std::ios::binary | std::ios::app);
    if (!out) {
        return false;
    }
    out.write(content.data(), static_cast<std::streamsize>(content.size()));
    return true;
}

}

