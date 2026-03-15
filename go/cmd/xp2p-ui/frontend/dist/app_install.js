(() => {
  const ui = window.ui;
  if (!ui) {
    return;
  }

  const { clientInstall, serverInstall } = ui;

  async function loadClientInstallDefaults() {
    ui.clearStatus(clientInstall.status);
    const api = ui.app();
    if (!api || !api.GetClientInstallDefaults) {
      ui.setStatus(clientInstall.status, "UI bindings are not available.", false);
      return;
    }
    try {
      const defaults = await api.GetClientInstallDefaults();
      clientInstall.installDir.value = defaults.installDir || "";
      clientInstall.configDir.value = defaults.configDir || "";
      clientInstall.serverAddress.value = defaults.serverAddress || "";
      clientInstall.serverPort.value = defaults.serverPort || "";
      clientInstall.user.value = defaults.user || "";
      clientInstall.password.value = defaults.password || "";
      clientInstall.serverName.value = defaults.serverName || "";
      clientInstall.allowInsecure.checked = !!defaults.allowInsecure;
      clientInstall.pinnedCert.value = defaults.pinnedPeerCertSha256 || "";
      clientInstall.verifyPeerName.value = defaults.verifyPeerCertByName || "";
      clientInstall.tunEnabled.checked = defaults.tunEnabled !== false;
      clientInstall.tunName.value = defaults.tunName || "";
      clientInstall.tunMtu.value = defaults.tunMtu || "";
      clientInstall.tunAddr.value = defaults.tunAddr || "";
    } catch (err) {
      ui.setStatus(clientInstall.status, "Failed to load defaults: " + (err?.message || err), false);
    }
  }

  async function loadServerInstallDefaults() {
    ui.clearStatus(serverInstall.status);
    const api = ui.app();
    if (!api || !api.GetServerInstallDefaults) {
      ui.setStatus(serverInstall.status, "UI bindings are not available.", false);
      return;
    }
    try {
      const defaults = await api.GetServerInstallDefaults();
      serverInstall.installDir.value = defaults.installDir || "";
      serverInstall.configDir.value = defaults.configDir || "";
      serverInstall.port.value = defaults.port || "";
      serverInstall.certStore.value = defaults.certStore || "";
      serverInstall.certFile.value = defaults.certFile || "";
      serverInstall.keyFile.value = defaults.keyFile || "";
      serverInstall.host.value = defaults.host || "";
    } catch (err) {
      ui.setStatus(serverInstall.status, "Failed to load defaults: " + (err?.message || err), false);
    }
  }

  function buildClientInstallPayload() {
    return {
      installDir: clientInstall.installDir.value.trim(),
      configDir: clientInstall.configDir.value.trim(),
      serverAddress: clientInstall.serverAddress.value.trim(),
      serverPort: clientInstall.serverPort.value.trim(),
      user: clientInstall.user.value.trim(),
      password: clientInstall.password.value,
      serverName: clientInstall.serverName.value.trim(),
      allowInsecure: !!clientInstall.allowInsecure.checked,
      pinnedPeerCertSha256: clientInstall.pinnedCert.value.trim(),
      verifyPeerCertByName: clientInstall.verifyPeerName.value.trim(),
      tunEnabled: !!clientInstall.tunEnabled.checked,
      tunName: clientInstall.tunName.value.trim(),
      tunMtu: Number(clientInstall.tunMtu.value || 0),
      tunAddr: clientInstall.tunAddr.value.trim()
    };
  }

  function buildServerInstallPayload() {
    return {
      installDir: serverInstall.installDir.value.trim(),
      configDir: serverInstall.configDir.value.trim(),
      port: serverInstall.port.value.trim(),
      certStore: serverInstall.certStore.value.trim(),
      certFile: serverInstall.certFile.value.trim(),
      keyFile: serverInstall.keyFile.value.trim(),
      host: serverInstall.host.value.trim()
    };
  }

  async function installClient() {
    ui.clearStatus(clientInstall.status);
    const payload = buildClientInstallPayload();
    if (!payload.serverAddress) {
      ui.setStatus(clientInstall.status, "Server address is required.", false);
      return;
    }
    if (!payload.user) {
      ui.setStatus(clientInstall.status, "User is required.", false);
      return;
    }
    if (!payload.password) {
      ui.setStatus(clientInstall.status, "Password is required.", false);
      return;
    }
    const api = ui.app();
    if (!api || !api.InstallClient) {
      ui.setStatus(clientInstall.status, "UI bindings are not available.", false);
      return;
    }
    clientInstall.installButton.disabled = true;
    try {
      await api.InstallClient(payload);
      ui.setStatus(clientInstall.status, "Client install completed.", true);
    } catch (err) {
      ui.setStatus(clientInstall.status, "Client install failed: " + (err?.message || err), false);
    } finally {
      clientInstall.installButton.disabled = false;
    }
  }

  async function installClientFromLink() {
    ui.clearStatus(clientInstall.linkStatus);
    const linkValue = clientInstall.installLink.value.trim();
    if (!linkValue) {
      ui.setStatus(clientInstall.linkStatus, "Link is required.", false);
      return;
    }
    const api = ui.app();
    if (!api || !api.InstallFromLink) {
      ui.setStatus(clientInstall.linkStatus, "UI bindings are not available.", false);
      return;
    }
    clientInstall.installLinkButton.disabled = true;
    try {
      await api.InstallFromLink(linkValue);
      ui.setStatus(clientInstall.linkStatus, "Link install completed.", true);
    } catch (err) {
      ui.setStatus(clientInstall.linkStatus, "Link install failed: " + (err?.message || err), false);
    } finally {
      clientInstall.installLinkButton.disabled = false;
    }
  }

  async function installServer(source) {
    const statusNode = source === "link" ? serverInstall.linkStatus : serverInstall.status;
    ui.clearStatus(statusNode);
    const api = ui.app();
    if (!api) {
      ui.setStatus(statusNode, "UI bindings are not available.", false);
      return;
    }
    const button = source === "link" ? serverInstall.linkButton : serverInstall.button;
    button.disabled = true;
    try {
      if (source === "link") {
        const linkValue = serverInstall.link.value.trim();
        if (!linkValue) {
          ui.setStatus(statusNode, "Link is required.", false);
          return;
        }
        if (!api.InstallServerFromLink) {
          ui.setStatus(statusNode, "UI bindings are not available.", false);
          return;
        }
        await api.InstallServerFromLink(linkValue);
        ui.setStatus(statusNode, "Server install completed.", true);
      } else {
        if (!api.InstallServer) {
          ui.setStatus(statusNode, "UI bindings are not available.", false);
          return;
        }
        const payload = buildServerInstallPayload();
        await api.InstallServer(payload);
        ui.setStatus(statusNode, "Server install completed.", true);
      }
    } catch (err) {
      ui.setStatus(statusNode, "Server install failed: " + (err?.message || err), false);
    } finally {
      button.disabled = false;
    }
  }

  clientInstall.installButton.addEventListener("click", installClient);
  clientInstall.resetButton.addEventListener("click", loadClientInstallDefaults);
  clientInstall.installLinkButton.addEventListener("click", installClientFromLink);
  ui.bindCard(clientInstall.cardLink, () => ui.selectClientInstallMode("link"));
  ui.bindCard(clientInstall.cardManual, () => ui.selectClientInstallMode("manual"));

  serverInstall.linkButton.addEventListener("click", () => installServer("link"));
  serverInstall.button.addEventListener("click", () => installServer("manual"));
  serverInstall.resetButton.addEventListener("click", loadServerInstallDefaults);
  ui.bindCard(serverInstall.cardLink, () => ui.selectServerInstallMode("link"));
  ui.bindCard(serverInstall.cardManual, () => ui.selectServerInstallMode("manual"));

  loadClientInstallDefaults();
  loadServerInstallDefaults();
})();
