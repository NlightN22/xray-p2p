#pragma once

#include <string>
#include <vector>

namespace xp2p::ui {

std::string GetEnv(const std::string& key);
std::string GetUserNameForAudit();
std::string GetCommandLineForAudit();

bool FileExists(const std::string& path);
std::vector<unsigned char> ReadFileBytesOrEmpty(const std::string& path);
bool EnsureDirForFile(const std::string& path);
bool WriteFileAtomic(const std::string& path, const std::vector<unsigned char>& data);
bool AppendFileTextUtf8(const std::string& path, const std::string& content);

}

