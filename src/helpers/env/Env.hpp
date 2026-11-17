#pragma once

#include <string>

namespace Env {
    bool envEnabled(const std::string& env);
    bool isTrace();

    // copy every HYPERLAND_* variable to its HYPRLAND_* twin, unless that one is already set
    void mirrorHyperlandEnv();
}
