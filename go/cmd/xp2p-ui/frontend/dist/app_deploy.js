(() => {
  const ui = window.ui;
  if (!ui) {
    return;
  }

  const { clientDeploy, serverDeploy } = ui;

  async function loadClientDeployDefaults() {
    ui.clearStatus(clientDeploy.status);
    const api = ui.app();
    if (!api || !api.GetClientDeployDefaults) {
      ui.setStatus(clientDeploy.status, "UI bindings are not available.", false);
      return;
    }
    try {
      const defaults = await api.GetClientDeployDefaults();
      clientDeploy.host.value = defaults.host || "";
      clientDeploy.port.value = defaults.deployPort || "";
      clientDeploy.installDir.value = defaults.installDir || "";
      clientDeploy.user.value = defaults.user || "";
      clientDeploy.password.value = defaults.password || "";
      clientDeploy.trojanPort.value = defaults.trojanPort || "";
    } catch (err) {
      ui.setStatus(clientDeploy.status, "Failed to load defaults: " + (err?.message || err), false);
    }
  }

  async function loadServerDeployDefaults() {
    ui.clearStatus(serverDeploy.status);
    const api = ui.app();
    if (!api || !api.GetServerDeployDefaults) {
      ui.setStatus(serverDeploy.status, "UI bindings are not available.", false);
      return;
    }
    try {
      const defaults = await api.GetServerDeployDefaults();
      serverDeploy.listen.value = defaults.listen || "";
      serverDeploy.diagPort.value = defaults.diagPort || "";
      serverDeploy.timeout.value = defaults.timeout || "";
    } catch (err) {
      ui.setStatus(serverDeploy.status, "Failed to load defaults: " + (err?.message || err), false);
    }
  }

  function buildClientDeployPayload() {
    return {
      host: clientDeploy.host.value.trim(),
      deployPort: clientDeploy.port.value.trim(),
      installDir: clientDeploy.installDir.value.trim(),
      user: clientDeploy.user.value.trim(),
      password: clientDeploy.password.value.trim(),
      trojanPort: clientDeploy.trojanPort.value.trim()
    };
  }

  function buildServerDeployPayload() {
    return {
      link: serverDeploy.link.value.trim(),
      listen: serverDeploy.listen.value.trim(),
      diagPort: serverDeploy.diagPort.value.trim(),
      timeout: serverDeploy.timeout.value.trim()
    };
  }

  async function deployClient() {
    ui.clearStatus(clientDeploy.status);
    const payload = buildClientDeployPayload();
    if (!payload.host) {
      ui.setStatus(clientDeploy.status, "Deploy host is required.", false);
      return;
    }
    const api = ui.app();
    if (!api || !api.DeployClient) {
      ui.setStatus(clientDeploy.status, "UI bindings are not available.", false);
      return;
    }
    clientDeploy.button.disabled = true;
    try {
      await api.DeployClient(payload);
      ui.setStatus(clientDeploy.status, "Client deploy completed.", true);
    } catch (err) {
      ui.setStatus(clientDeploy.status, "Client deploy failed: " + (err?.message || err), false);
    } finally {
      clientDeploy.button.disabled = false;
    }
  }

  async function deployServer() {
    ui.clearStatus(serverDeploy.status);
    const payload = buildServerDeployPayload();
    if (!payload.link) {
      ui.setStatus(serverDeploy.status, "Deploy link is required.", false);
      return;
    }
    const api = ui.app();
    if (!api || !api.DeployServer) {
      ui.setStatus(serverDeploy.status, "UI bindings are not available.", false);
      return;
    }
    serverDeploy.button.disabled = true;
    try {
      await api.DeployServer(payload);
      ui.setStatus(serverDeploy.status, "Server deploy completed.", true);
    } catch (err) {
      ui.setStatus(serverDeploy.status, "Server deploy failed: " + (err?.message || err), false);
    } finally {
      serverDeploy.button.disabled = false;
    }
  }

  clientDeploy.button.addEventListener("click", deployClient);
  clientDeploy.resetButton.addEventListener("click", loadClientDeployDefaults);

  serverDeploy.button.addEventListener("click", deployServer);
  serverDeploy.resetButton.addEventListener("click", loadServerDeployDefaults);

  loadClientDeployDefaults();
  loadServerDeployDefaults();
})();
