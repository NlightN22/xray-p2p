#include "path_utils.h"

#include <shlobj.h>
#include <windows.h>

namespace xp2p::ui {

static std::wstring GetEnvWide(const wchar_t* key) {
    DWORD needed = GetEnvironmentVariableW(key, nullptr, 0);
    if (needed == 0) {
        return L"";
    }
    std::wstring out(static_cast<size_t>(needed), L'\0');
    DWORD got = GetEnvironmentVariableW(key, out.data(), needed);
    if (got == 0) {
        return L"";
    }
    if (!out.empty() && out.back() == L'\0') {
        out.pop_back();
    }
    return out;
}

static std::wstring JoinPath(const std::wstring& left, const std::wstring& right) {
    if (left.empty()) {
        return right;
    }
    if (right.empty()) {
        return left;
    }
    if (left.back() == L'\\' || left.back() == L'/') {
        return left + right;
    }
    return left + L'\\' + right;
}

bool EnsureDirectoryTree(const std::wstring& path) {
    if (path.empty()) {
        return false;
    }
    DWORD attrs = GetFileAttributesW(path.c_str());
    if (attrs != INVALID_FILE_ATTRIBUTES && (attrs & FILE_ATTRIBUTE_DIRECTORY)) {
        return true;
    }

    std::wstring current;
    current.reserve(path.size());
    for (size_t i = 0; i < path.size(); i++) {
        wchar_t c = path[i];
        current.push_back(c);
        if (c == L'\\' || c == L'/') {
            if (current.size() <= 3) {
                continue;
            }
            CreateDirectoryW(current.c_str(), nullptr);
        }
    }
    CreateDirectoryW(path.c_str(), nullptr);
    attrs = GetFileAttributesW(path.c_str());
    return attrs != INVALID_FILE_ATTRIBUTES && (attrs & FILE_ATTRIBUTE_DIRECTORY);
}

std::wstring GetProgramDataDir() {
    PWSTR raw = nullptr;
    if (SUCCEEDED(SHGetKnownFolderPath(FOLDERID_ProgramData, 0, nullptr, &raw)) && raw) {
        std::wstring dir(raw);
        CoTaskMemFree(raw);
        return dir;
    }
    return L"C:\\ProgramData";
}

std::wstring GetXp2pLogsDir() {
    std::wstring overrideLogRoot = GetEnvWide(L"XP2P_LOG_ROOT");
    if (!overrideLogRoot.empty()) {
        return overrideLogRoot;
    }
    std::wstring configRoot = GetEnvWide(L"XP2P_CONFIG_ROOT");
    if (configRoot.empty()) {
        configRoot = JoinPath(GetProgramDataDir(), L"xp2p");
    }
    return JoinPath(configRoot, L"logs");
}

std::wstring GetUiLogPath() {
    return JoinPath(GetXp2pLogsDir(), L"ui-xp2p.log");
}

}
