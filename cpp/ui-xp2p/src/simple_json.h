#pragma once

#include <optional>
#include <string>

namespace xp2p::ui {

std::optional<std::string> ExtractObject(const std::string& json, const std::string& key);
std::optional<std::string> ExtractString(const std::string& json, const std::string& key);
std::optional<bool> ExtractBool(const std::string& json, const std::string& key);
std::optional<int> ExtractInt(const std::string& json, const std::string& key);
bool HasKey(const std::string& json, const std::string& key);

}

