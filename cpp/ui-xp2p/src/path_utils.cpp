#include "path_utils.h"

#include <shlobj.h>
#include <windows.h>

namespace xp2p::ui {

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
    return JoinPath(JoinPath(GetProgramDataDir(), L"xp2p"), L"logs");
}

std::wstring GetUiLogPath() {
    return JoinPath(GetXp2pLogsDir(), L"ui-xp2p.log");
}

}

