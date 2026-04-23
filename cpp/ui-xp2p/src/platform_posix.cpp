#include "platform.h"

#include <cerrno>
#include <cstdlib>
#include <filesystem>
#include <fstream>
#include <sstream>

namespace xp2p::ui {

std::string GetEnv(const std::string& key) {
    const char* v = std::getenv(key.c_str());
    return v ? std::string(v) : std::string();
}

std::string GetUserNameForAudit() {
    const std::string u = GetEnv("USER");
    return u.empty() ? "unknown" : u;
}

std::string GetCommandLineForAudit() {
    return "";
}

bool FileExists(const std::string& path) {
    return std::filesystem::exists(path);
}

std::vector<unsigned char> ReadFileBytesOrEmpty(const std::string& path) {
    std::ifstream f(path, std::ios::binary);
    if (!f) {
        return {};
    }
    std::vector<unsigned char> data((std::istreambuf_iterator<char>(f)), std::istreambuf_iterator<char>());
    return data;
}

bool EnsureDirForFile(const std::string& path) {
    std::error_code ec;
    std::filesystem::path p(path);
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
    std::filesystem::path p(path);
    std::filesystem::path dir = p.parent_path();
    std::string tmpName = ".tmp-ui-xp2p";
    std::filesystem::path tmp = dir / tmpName;
    {
        std::ofstream out(tmp, std::ios::binary | std::ios::trunc);
        if (!out) {
            return false;
        }
        out.write(reinterpret_cast<const char*>(data.data()), static_cast<std::streamsize>(data.size()));
        out.flush();
    }
    std::error_code ec;
    std::filesystem::rename(tmp, p, ec);
    if (ec) {
        std::filesystem::remove(tmp, ec);
        return false;
    }
    return true;
}

bool AppendFileTextUtf8(const std::string& path, const std::string& content) {
    if (!EnsureDirForFile(path)) {
        return false;
    }
    std::ofstream out(path, std::ios::binary | std::ios::app);
    if (!out) {
        return false;
    }
    out.write(content.data(), static_cast<std::streamsize>(content.size()));
    return true;
}

}

