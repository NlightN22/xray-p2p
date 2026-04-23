#pragma once

#include <optional>
#include <string>
#include <vector>

namespace xp2p::ui {

std::string NormalizeLineEndings(const std::string& text);
std::string UpdateTomlValue(const std::string& content, const std::string& section, const std::string& key, const std::string& value);
std::optional<std::string> ReadTomlValue(const std::string& content, const std::string& section, const std::string& key);
std::vector<std::string> ReadEndpointTags(const std::string& content);
std::string TrimTomlQuotes(const std::string& value);

}

