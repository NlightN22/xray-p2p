using System;

namespace Xp2pUi;

internal interface IBackend
{
    OperationResult ClientInstall(ClientInstallRequest request);
    OperationResult ClientDeploy(ClientDeployRequest request);
    OperationResult ServerInstall(ServerInstallRequest request);
    OperationResult ServerDeploy(ServerDeployRequest request);
}

internal static class BackendFactory
{
    public static IBackend Create()
    {
        var backendMode = Environment.GetEnvironmentVariable("XP2P_UI_BACKEND");
        if (string.Equals(backendMode, "ipc", StringComparison.OrdinalIgnoreCase))
        {
            return new IpcBackend();
        }
        return new StubBackend();
    }
}

internal sealed class StubBackend : IBackend
{
    public OperationResult ClientInstall(ClientInstallRequest request)
        => OperationResult.NotImplemented("Client install backend not configured.");

    public OperationResult ClientDeploy(ClientDeployRequest request)
        => OperationResult.NotImplemented("Client deploy backend not configured.");

    public OperationResult ServerInstall(ServerInstallRequest request)
        => OperationResult.NotImplemented("Server install backend not configured.");

    public OperationResult ServerDeploy(ServerDeployRequest request)
        => OperationResult.NotImplemented("Server deploy backend not configured.");
}

internal sealed class IpcBackend : IBackend
{
    public OperationResult ClientInstall(ClientInstallRequest request)
        => OperationResult.NotImplemented("IPC backend is not configured.");

    public OperationResult ClientDeploy(ClientDeployRequest request)
        => OperationResult.NotImplemented("IPC backend is not configured.");

    public OperationResult ServerInstall(ServerInstallRequest request)
        => OperationResult.NotImplemented("IPC backend is not configured.");

    public OperationResult ServerDeploy(ServerDeployRequest request)
        => OperationResult.NotImplemented("IPC backend is not configured.");
}

internal sealed record OperationResult(bool Success, string Message)
{
    public static OperationResult Ok(string message) => new OperationResult(true, message);
    public static OperationResult Fail(string message) => new OperationResult(false, message);
    public static OperationResult NotImplemented(string message) => new OperationResult(false, message);
}

internal sealed record ClientInstallRequest(
    bool UseLink,
    string Link,
    string InstallDir,
    string ConfigDir,
    string Host,
    string Port,
    string User,
    string Password,
    string ServerName,
    bool AllowInsecure,
    bool StrictTls);

internal sealed record ClientDeployRequest(
    string Host,
    string Port,
    string InstallDir,
    string User,
    string Password,
    string TrojanPort);

internal sealed record ServerInstallRequest(
    string Path,
    string ConfigDir,
    string Port,
    string CertStore,
    string CertFile,
    string KeyFile,
    string Host);

internal sealed record ServerDeployRequest(
    string Listen,
    string Link,
    string DiagPort,
    string Timeout);
