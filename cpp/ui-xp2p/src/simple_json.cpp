#include "simple_json.h"

#include <cctype>

namespace xp2p::ui {

static size_t SkipWs(const std::string& s, size_t i) {
    while (i < s.size() && std::isspace(static_cast<unsigned char>(s[i]))) {
        i++;
    }
    return i;
}

static std::optional<size_t> FindKeyPos(const std::string& json, const std::string& key) {
    if (key.empty()) {
        return std::nullopt;
    }
    const std::string needle = "\"" + key + "\"";
    size_t pos = json.find(needle);
    if (pos == std::string::npos) {
        return std::nullopt;
    }
    return pos + needle.size();
}

static std::optional<size_t> FindValueStart(const std::string& json, const std::string& key) {
    auto afterKey = FindKeyPos(json, key);
    if (!afterKey) {
        return std::nullopt;
    }
    size_t i = SkipWs(json, *afterKey);
    if (i >= json.size() || json[i] != ':') {
        return std::nullopt;
    }
    i = SkipWs(json, i + 1);
    if (i >= json.size()) {
        return std::nullopt;
    }
    return i;
}

bool HasKey(const std::string& json, const std::string& key) {
    return FindKeyPos(json, key).has_value();
}

std::optional<std::string> ExtractObject(const std::string& json, const std::string& key) {
    auto startOpt = FindValueStart(json, key);
    if (!startOpt) {
        return std::nullopt;
    }
    size_t i = *startOpt;
    if (json[i] != '{') {
        return std::nullopt;
    }
    int depth = 0;
    size_t begin = i;
    for (; i < json.size(); i++) {
        char c = json[i];
        if (c == '"') {
            i++;
            while (i < json.size()) {
                if (json[i] == '\\') {
                    i += 2;
                    continue;
                }
                if (json[i] == '"') {
                    break;
                }
                i++;
            }
            continue;
        }
        if (c == '{') {
            depth++;
            continue;
        }
        if (c == '}') {
            depth--;
            if (depth == 0) {
                return json.substr(begin, i - begin + 1);
            }
        }
    }
    return std::nullopt;
}

std::optional<std::string> ExtractString(const std::string& json, const std::string& key) {
    auto startOpt = FindValueStart(json, key);
    if (!startOpt) {
        return std::nullopt;
    }
    size_t i = *startOpt;
    if (json[i] != '"') {
        return std::nullopt;
    }
    i++;
    std::string out;
    for (; i < json.size(); i++) {
        char c = json[i];
        if (c == '\\') {
            if (i + 1 < json.size()) {
                out.push_back(json[i + 1]);
                i++;
                continue;
            }
            break;
        }
        if (c == '"') {
            return out;
        }
        out.push_back(c);
    }
    return std::nullopt;
}

std::optional<bool> ExtractBool(const std::string& json, const std::string& key) {
    auto startOpt = FindValueStart(json, key);
    if (!startOpt) {
        return std::nullopt;
    }
    size_t i = *startOpt;
    if (json.compare(i, 4, "true") == 0) {
        return true;
    }
    if (json.compare(i, 5, "false") == 0) {
        return false;
    }
    return std::nullopt;
}

std::optional<int> ExtractInt(const std::string& json, const std::string& key) {
    auto startOpt = FindValueStart(json, key);
    if (!startOpt) {
        return std::nullopt;
    }
    size_t i = *startOpt;
    bool neg = false;
    if (json[i] == '-') {
        neg = true;
        i++;
    }
    long long val = 0;
    bool any = false;
    for (; i < json.size(); i++) {
        if (!std::isdigit(static_cast<unsigned char>(json[i]))) {
            break;
        }
        any = true;
        val = val * 10 + (json[i] - '0');
        if (val > 0x7fffffffLL) {
            break;
        }
    }
    if (!any) {
        return std::nullopt;
    }
    int out = static_cast<int>(val);
    return neg ? -out : out;
}

}

