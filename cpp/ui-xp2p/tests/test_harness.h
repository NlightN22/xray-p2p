#pragma once

#include <iostream>
#include <string>

namespace xp2p::ui::tests {

inline int g_failures = 0;

inline void Expect(bool cond, const char* message) {
    if (cond) {
        return;
    }
    g_failures++;
    std::cerr << "FAIL: " << message << "\n";
}

inline void ExpectEq(const std::string& got, const std::string& want, const char* message) {
    if (got == want) {
        return;
    }
    g_failures++;
    std::cerr << "FAIL: " << message << "\n";
    std::cerr << "  got : " << got << "\n";
    std::cerr << "  want: " << want << "\n";
}

inline void ExpectContains(const std::string& haystack, const std::string& needle, const char* message) {
    if (haystack.find(needle) != std::string::npos) {
        return;
    }
    g_failures++;
    std::cerr << "FAIL: " << message << "\n";
    std::cerr << "  missing: " << needle << "\n";
}

}

