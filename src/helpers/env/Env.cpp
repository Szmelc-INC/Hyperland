#include "Env.hpp"

#include <cstdlib>
#include <string_view>
#include <utility>
#include <vector>

extern char** environ;

bool          Env::envEnabled(const std::string& env) {
    auto ret = getenv(env.c_str());
    if (!ret)
        return false;

    const std::string_view sv = ret;

    return !sv.empty() && sv != "0";
}

void Env::mirrorHyperlandEnv() {
    std::vector<std::pair<std::string, std::string>> twins;
    for (char** e = environ; e && *e; ++e) {
        const std::string_view kv = *e;
        const auto             eq = kv.find('=');
        if (!kv.starts_with("HYPERLAND_") || eq == std::string_view::npos)
            continue;
        twins.emplace_back("HYPRLAND_" + std::string{kv.substr(10, eq - 10)}, std::string{kv.substr(eq + 1)});
    }
    for (const auto& [k, v] : twins) {
        setenv(k.c_str(), v.c_str(), 0);
    }
}

bool Env::isTrace() {
    static bool TRACE = envEnabled("HYPRLAND_TRACE");
    return TRACE;
}
