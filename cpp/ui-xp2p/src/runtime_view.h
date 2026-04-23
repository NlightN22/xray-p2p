#pragma once

#include "runtime_state.h"

#include <string>

namespace xp2p::ui {

enum class ClientRuntimeStatus {
    Ready,
    Pending,
    Failed
};

struct ClientRuntimeView {
    ClientRuntimeStatus status = ClientRuntimeStatus::Failed;
    std::string summary;
    std::string detail;
    std::string lastError;
    bool isFresh = false;
};

ClientRuntimeView BuildClientRuntimeView(const std::string& serviceStatus, const std::optional<ClientStateFile>& state);

}

