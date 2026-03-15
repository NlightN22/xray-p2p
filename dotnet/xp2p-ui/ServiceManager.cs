using System;
using System.ServiceProcess;

namespace Xp2pUi;

internal static class ServiceNames
{
    public const string Client = "xp2p-client";
    public const string Server = "xp2p-server";
}

internal sealed class ServiceManager
{
    public event EventHandler<bool>? ActivityChanged;
    public event EventHandler<ServiceStatusSnapshot>? StatusChanged;
    public bool IsBusy { get; private set; }

    public ServiceStatusSnapshot GetSnapshot()
    {
        return new ServiceStatusSnapshot(
            GetStatus(ServiceNames.Client),
            GetStatus(ServiceNames.Server));
    }

    public string GetStatus(string serviceName)
    {
        try
        {
            using var controller = new ServiceController(serviceName);
            return controller.Status.ToString();
        }
        catch (Exception ex)
        {
            return $"Error ({ex.Message})";
        }
    }

    public async System.Threading.Tasks.Task<string> StartServiceAsync(string serviceName)
    {
        IsBusy = true;
        ActivityChanged?.Invoke(this, true);
        try
        {
            return await System.Threading.Tasks.Task.Run(() =>
            {
                using var controller = new ServiceController(serviceName);
                if (controller.Status == ServiceControllerStatus.Running)
                {
                    return $"{serviceName} already running.";
                }
                controller.Start();
                controller.WaitForStatus(ServiceControllerStatus.Running, TimeSpan.FromSeconds(20));
                return $"{serviceName} started.";
            });
        }
        catch (Exception ex)
        {
            return $"{serviceName} start failed: {ex.Message}";
        }
        finally
        {
            IsBusy = false;
            ActivityChanged?.Invoke(this, false);
            StatusChanged?.Invoke(this, GetSnapshot());
        }
    }

    public async System.Threading.Tasks.Task<string> StopServiceAsync(string serviceName)
    {
        IsBusy = true;
        ActivityChanged?.Invoke(this, true);
        try
        {
            return await System.Threading.Tasks.Task.Run(() =>
            {
                using var controller = new ServiceController(serviceName);
                if (controller.Status == ServiceControllerStatus.Stopped)
                {
                    return $"{serviceName} already stopped.";
                }
                controller.Stop();
                controller.WaitForStatus(ServiceControllerStatus.Stopped, TimeSpan.FromSeconds(20));
                return $"{serviceName} stopped.";
            });
        }
        catch (Exception ex)
        {
            return $"{serviceName} stop failed: {ex.Message}";
        }
        finally
        {
            IsBusy = false;
            ActivityChanged?.Invoke(this, false);
            StatusChanged?.Invoke(this, GetSnapshot());
        }
    }
}

internal sealed record ServiceStatusSnapshot(string ClientStatus, string ServerStatus);
