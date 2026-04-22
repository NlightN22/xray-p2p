#pragma once

#include <string>

namespace xp2p::ui {

std::wstring GetProgramDataDir();
std::wstring GetXp2pLogsDir();
std::wstring GetUiLogPath();
bool EnsureDirectoryTree(const std::wstring& path);

}

