#include "runtime_view.h"

#include "ui_logic.h"

namespace xp2p::ui {

static std::string BuildTunDetail(const std::optional<RuntimeTunState>& tunOpt, const std::optional<RuntimeRoutesState>& routesOpt, const std::string& routeLabel) {
    const RuntimeTunState tun = tunOpt.value_or(RuntimeTunState{});
    const RuntimeRoutesState routes = routesOpt.value_or(RuntimeRoutesState{});

    const std::string name = tun.name.empty() ? "-" : tun.name;
    const std::string ip = tun.ipv4.empty() ? "-" : tun.ipv4;
    const std::string prefix = tun.prefix > 0 ? ("/" + std::to_string(tun.prefix)) : "";
    const std::string oper = tun.operStatus.empty() ? "-" : tun.operStatus;
    const std::string dad = tun.dadState.empty() ? "-" : tun.dadState;

    std::string routesText = "none";
    if (routeLabel == "Full") {
        routesText = "full(" + std::to_string(routes.fullBypassCount) + ")";
    } else if (routeLabel == "Split") {
        routesText = "split(" + std::to_string(routes.redirectCount) + ")";
    }

    return "Tun: " + name + " " + ip + prefix + " " + oper + "/" + dad + " | Routes: " + routesText;
}

ClientRuntimeView BuildClientRuntimeView(const std::string& serviceStatus, const std::optional<ClientStateFile>& stateOpt) {
    if (!stateOpt.has_value() || !stateOpt->runtime.has_value() || !stateOpt->runtime->hasTimestamp) {
        ClientRuntimeView v{};
        v.status = ClientRuntimeStatus::Failed;
        v.summary = "Tun: Unknown";
        v.detail = "Tun: Unknown";
        v.isFresh = false;
        return v;
    }

    const ClientStateFile& state = *stateOpt;
    const ClientRuntimeState& runtime = *state.runtime;

    if (IsServiceStopped(serviceStatus)) {
        return ClientRuntimeView{ClientRuntimeStatus::Failed, "Tun: Stopped", "Tun: Stopped", runtime.lastError, true};
    }
    if (IsServicePending(serviceStatus)) {
        return ClientRuntimeView{ClientRuntimeStatus::Pending, "Tun: Pending", "Tun: Pending", runtime.lastError, true};
    }
    if (!IsServiceRunning(serviceStatus)) {
        const std::string label = "Tun: " + serviceStatus;
        return ClientRuntimeView{ClientRuntimeStatus::Failed, label, label, runtime.lastError, true};
    }

    if (!state.tunEnabled) {
        if (runtime.socksReady) {
            return ClientRuntimeView{ClientRuntimeStatus::Ready, "Proxy: Ready (SOCKS)", "Proxy: Ready (SOCKS)", "", true};
        }
        return ClientRuntimeView{ClientRuntimeStatus::Pending, "Proxy: Pending (SOCKS)", "Proxy: Pending (SOCKS)", runtime.lastError, true};
    }

    const bool tunReady = runtime.tun.has_value() && runtime.tun->ready;
    const bool fullApplied = runtime.routes.has_value() && runtime.routes->fullApplied;
    const bool redirectApplied = runtime.routes.has_value() && runtime.routes->redirectApplied;
    const std::string routeLabel = fullApplied ? "Full" : redirectApplied ? "Split" : "Tun";
    const bool routeReady = fullApplied || redirectApplied;
    const std::string summary = (tunReady && routeReady) ? ("Tun: Ready | Routes: " + routeLabel) : ("Tun: Pending | Routes: " + routeLabel);
    const std::string detail = BuildTunDetail(runtime.tun, runtime.routes, routeLabel);
    const ClientRuntimeStatus status = (tunReady && routeReady) ? ClientRuntimeStatus::Ready : ClientRuntimeStatus::Pending;

    ClientRuntimeView v{};
    v.status = status;
    v.summary = summary;
    v.detail = detail;
    v.lastError = runtime.lastError;
    v.isFresh = true;
    return v;
}

}

