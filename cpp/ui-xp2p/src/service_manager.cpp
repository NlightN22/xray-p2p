#include "service_manager.h"

#include <windows.h>

#include <string>

namespace xp2p::ui {

static std::string StateToLabel(DWORD state) {
    switch (state) {
        case SERVICE_STOPPED:
            return "Stopped";
        case SERVICE_START_PENDING:
            return "StartPending";
        case SERVICE_STOP_PENDING:
            return "StopPending";
        case SERVICE_RUNNING:
            return "Running";
        case SERVICE_CONTINUE_PENDING:
            return "ContinuePending";
        case SERVICE_PAUSE_PENDING:
            return "PausePending";
        case SERVICE_PAUSED:
            return "Paused";
        default:
            return "Unknown";
    }
}

static ServiceStatus MakeError(const char* prefix, DWORD err) {
    ServiceStatus status{};
    status.ok = false;
    status.error = err;
    status.state = 0;
    status.label = std::string(prefix) + "(" + std::to_string(err) + ")";
    return status;
}

static ServiceStatus QueryStatusByHandle(SC_HANDLE svc) {
    SERVICE_STATUS_PROCESS ssp{};
    DWORD bytesNeeded = 0;
    if (!QueryServiceStatusEx(svc, SC_STATUS_PROCESS_INFO, reinterpret_cast<LPBYTE>(&ssp), sizeof(ssp), &bytesNeeded)) {
        return MakeError("QueryFailed", GetLastError());
    }
    ServiceStatus status{};
    status.ok = true;
    status.error = 0;
    status.state = ssp.dwCurrentState;
    status.label = StateToLabel(ssp.dwCurrentState);
    return status;
}

ServiceStatus QueryServiceStatus(const wchar_t* serviceName) {
    SC_HANDLE scm = OpenSCManagerW(nullptr, nullptr, SC_MANAGER_CONNECT);
    if (!scm) {
        return MakeError("ScmOpenFailed", GetLastError());
    }
    SC_HANDLE svc = OpenServiceW(scm, serviceName, SERVICE_QUERY_STATUS);
    if (!svc) {
        DWORD err = GetLastError();
        CloseServiceHandle(scm);
        if (err == ERROR_SERVICE_DOES_NOT_EXIST) {
            ServiceStatus status{};
            status.ok = false;
            status.error = err;
            status.state = 0;
            status.label = "NotInstalled";
            return status;
        }
        return MakeError("ServiceOpenFailed", err);
    }

    ServiceStatus status = QueryStatusByHandle(svc);
    CloseServiceHandle(svc);
    CloseServiceHandle(scm);
    return status;
}

static ServiceStatus WaitForState(SC_HANDLE svc, DWORD desiredState, DWORD timeoutMs) {
    const DWORD pollMs = 250;
    DWORD waited = 0;
    while (waited <= timeoutMs) {
        ServiceStatus cur = QueryStatusByHandle(svc);
        if (!cur.ok) {
            return cur;
        }
        if (cur.state == desiredState) {
            return cur;
        }
        Sleep(pollMs);
        waited += pollMs;
    }
    return MakeError("Timeout", ERROR_TIMEOUT);
}

static ServiceStatus StartStopCore(
    const wchar_t* serviceName,
    bool start,
    DWORD timeoutMs) {
    SC_HANDLE scm = OpenSCManagerW(nullptr, nullptr, SC_MANAGER_CONNECT);
    if (!scm) {
        return MakeError("ScmOpenFailed", GetLastError());
    }
    DWORD access = SERVICE_QUERY_STATUS | (start ? SERVICE_START : SERVICE_STOP);
    SC_HANDLE svc = OpenServiceW(scm, serviceName, access);
    if (!svc) {
        DWORD err = GetLastError();
        CloseServiceHandle(scm);
        return MakeError("ServiceOpenFailed", err);
    }

    ServiceStatus status = QueryStatusByHandle(svc);
    if (status.ok) {
        const bool already = start ? (status.label == "Running") : (status.label == "Stopped");
        if (already) {
            CloseServiceHandle(svc);
            CloseServiceHandle(scm);
            return status;
        }
    }

    bool ok = false;
    if (start) {
        ok = StartServiceW(svc, 0, nullptr) != 0;
    } else {
        SERVICE_STATUS ignored{};
        ok = ControlService(svc, SERVICE_CONTROL_STOP, &ignored) != 0;
    }
    if (!ok) {
        DWORD err = GetLastError();
        CloseServiceHandle(svc);
        CloseServiceHandle(scm);
        return MakeError(start ? "StartFailed" : "StopFailed", err);
    }

    ServiceStatus finalStatus = WaitForState(svc, start ? SERVICE_RUNNING : SERVICE_STOPPED, timeoutMs);
    CloseServiceHandle(svc);
    CloseServiceHandle(scm);
    return finalStatus;
}

ServiceStatus StartServiceAndWait(const wchar_t* serviceName, unsigned long timeoutMs) {
    return StartStopCore(serviceName, true, timeoutMs);
}

ServiceStatus StopServiceAndWait(const wchar_t* serviceName, unsigned long timeoutMs) {
    return StartStopCore(serviceName, false, timeoutMs);
}

ServiceStatus RestartServiceAndWait(const wchar_t* serviceName, unsigned long timeoutMs) {
    ServiceStatus stopped = StopServiceAndWait(serviceName, timeoutMs);
    if (!stopped.ok && stopped.error != 0 && stopped.error != ERROR_SERVICE_NOT_ACTIVE) {
        return stopped;
    }
    return StartServiceAndWait(serviceName, timeoutMs);
}

}
