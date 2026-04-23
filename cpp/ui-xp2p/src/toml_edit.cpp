#include "toml_edit.h"

#include <algorithm>
#include <regex>
#include <sstream>

namespace xp2p::ui {

std::string NormalizeLineEndings(const std::string& text) {
    if (text.empty()) {
        return "\n";
    }
    std::string out;
    out.reserve(text.size() + 2);
    for (size_t i = 0; i < text.size(); i++) {
        char c = text[i];
        if (c == '\r') {
            if (i + 1 < text.size() && text[i + 1] == '\n') {
                i++;
            }
            out.push_back('\n');
            continue;
        }
        out.push_back(c);
    }
    if (out.empty() || out.back() != '\n') {
        out.push_back('\n');
    }
    return out;
}

static std::vector<std::string> SplitLines(const std::string& content) {
    std::vector<std::string> lines;
    std::istringstream ss(NormalizeLineEndings(content));
    std::string line;
    while (std::getline(ss, line, '\n')) {
        lines.push_back(line);
    }
    return lines;
}

static std::string JoinLines(const std::vector<std::string>& lines) {
    if (lines.empty()) {
        return "\n";
    }
    std::string out;
    for (const auto& line : lines) {
        out.append(line);
        out.push_back('\n');
    }
    return out;
}

static std::string Trim(const std::string& s) {
    size_t b = 0;
    while (b < s.size() && std::isspace(static_cast<unsigned char>(s[b]))) {
        b++;
    }
    size_t e = s.size();
    while (e > b && std::isspace(static_cast<unsigned char>(s[e - 1]))) {
        e--;
    }
    return s.substr(b, e - b);
}

static std::string NormalizeSectionName(const std::string& s) {
    std::string out = s;
    out = Trim(out);
    std::transform(out.begin(), out.end(), out.begin(), [](unsigned char c) { return static_cast<char>(std::tolower(c)); });
    return out;
}

static bool IsSectionHeader(const std::string& line, std::string* outName) {
    std::smatch m;
    if (!std::regex_match(line, m, std::regex(R"(^\s*\[([^\]]+)\]\s*$)", std::regex::icase))) {
        return false;
    }
    if (outName) {
        *outName = NormalizeSectionName(m[1].str());
    }
    return true;
}

std::optional<std::string> ReadTomlValue(const std::string& content, const std::string& section, const std::string& key) {
    const std::string sec = NormalizeSectionName(section);
    const std::regex keyRe("^\\s*" + key + "\\s*=\\s*(.+?)\\s*$", std::regex::icase);
    bool in = false;
    for (const auto& line : SplitLines(content)) {
        std::string header;
        if (IsSectionHeader(line, &header)) {
            in = (header == sec);
            continue;
        }
        if (!in) {
            continue;
        }
        std::smatch m;
        if (std::regex_match(line, m, keyRe)) {
            return Trim(m[1].str());
        }
    }
    return std::nullopt;
}

std::string UpdateTomlValue(const std::string& content, const std::string& section, const std::string& key, const std::string& value) {
    std::vector<std::string> lines = SplitLines(content);
    const std::string sec = NormalizeSectionName(section);
    const std::regex keyRe("^\\s*" + key + "\\s*=.*$", std::regex::icase);

    bool in = false;
    bool sectionFound = false;
    size_t insertAt = lines.size();
    for (size_t i = 0; i < lines.size(); i++) {
        std::string header;
        if (IsSectionHeader(lines[i], &header)) {
            if (in) {
                insertAt = i;
                in = false;
            }
            if (header == sec) {
                in = true;
                sectionFound = true;
                insertAt = i + 1;
            }
            continue;
        }
        if (in) {
            insertAt = i + 1;
            if (std::regex_match(lines[i], keyRe)) {
                lines[i] = key + " = " + value;
                return JoinLines(lines);
            }
        }
    }

    if (!sectionFound) {
        if (!lines.empty() && !lines.back().empty()) {
            lines.push_back("");
        }
        lines.push_back("[" + section + "]");
        lines.push_back(key + " = " + value);
        return JoinLines(lines);
    }

    if (insertAt > lines.size()) {
        insertAt = lines.size();
    }
    lines.insert(lines.begin() + static_cast<long long>(insertAt), key + " = " + value);
    return JoinLines(lines);
}

std::string TrimTomlQuotes(const std::string& value) {
    if (value.size() < 2) {
        return value;
    }
    if ((value.front() == '"' && value.back() == '"') || (value.front() == '\'' && value.back() == '\'')) {
        return value.substr(1, value.size() - 2);
    }
    return value;
}

static std::string ExtractEndpointsInlineBlock(const std::vector<std::string>& lines) {
    bool capture = false;
    int bracketDepth = 0;
    std::string out;
    for (const auto& line : lines) {
        const std::string trimmed = Trim(line);
        if (!capture && std::regex_search(trimmed, std::regex(R"(^endpoints\s*=)", std::regex::icase))) {
            capture = true;
        }
        if (!capture) {
            continue;
        }
        out.append(line);
        out.push_back('\n');
        for (char c : line) {
            if (c == '[') {
                bracketDepth++;
            } else if (c == ']') {
                bracketDepth--;
                if (bracketDepth <= 0) {
                    return out;
                }
            }
        }
    }
    return "";
}

std::vector<std::string> ReadEndpointTags(const std::string& content) {
    std::vector<std::string> tags;
    const std::vector<std::string> lines = SplitLines(content);

    const std::string inlineBlock = ExtractEndpointsInlineBlock(lines);
    if (!inlineBlock.empty()) {
        const std::regex tagRe(R"(\btag\s*=\s*(['"])([^'"]+)\1)", std::regex::icase);
        for (auto it = std::sregex_iterator(inlineBlock.begin(), inlineBlock.end(), tagRe); it != std::sregex_iterator(); ++it) {
            const std::string tag = (*it)[2].str();
            if (!tag.empty()) {
                tags.push_back(tag);
            }
        }
    }

    bool inEndpoints = false;
    for (const auto& line : lines) {
        std::string header;
        if (std::regex_match(line, std::regex(R"(^\s*\[\[\s*([^\]]+)\s*\]\]\s*$)", std::regex::icase))) {
            std::smatch m;
            std::regex_match(line, m, std::regex(R"(^\s*\[\[\s*([^\]]+)\s*\]\]\s*$)", std::regex::icase));
            header = NormalizeSectionName(m[1].str());
            if (header == "client.endpoints") {
                inEndpoints = true;
            } else if (inEndpoints) {
                inEndpoints = false;
            }
            continue;
        }
        if (!inEndpoints) {
            continue;
        }
        std::smatch m;
        if (std::regex_match(line, m, std::regex(R"(^\s*tag\s*=\s*(['"])([^'"]+)\1\s*$)", std::regex::icase))) {
            const std::string tag = m[2].str();
            if (!tag.empty()) {
                tags.push_back(tag);
            }
        }
    }

    std::sort(tags.begin(), tags.end());
    tags.erase(std::unique(tags.begin(), tags.end()), tags.end());
    return tags;
}

}

