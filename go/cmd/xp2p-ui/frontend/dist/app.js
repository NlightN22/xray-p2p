      const app = () => (window.go && window.go.ui && window.go.ui.App ? window.go.ui.App : null);

      const tabs = Array.from(document.querySelectorAll(".tab"));
      const scenarios = Array.from(document.querySelectorAll(".scenario"));

      function selectTab(id) {
        scenarios.forEach((section) => {
          section.classList.toggle("active", section.id === id);
        });
        tabs.forEach((tab) => {
          tab.classList.toggle("active", tab.dataset.tab === id);
        });
      }

      tabs.forEach((tab) => {
        tab.addEventListener("click", () => selectTab(tab.dataset.tab));
      });

      function setStatus(node, message, ok) {
        node.textContent = message;
        node.className = "status" + (ok ? " ok" : " error");
      }

      function clearStatus(node) {
        node.textContent = "";
        node.className = "status";
      }

      const clientInstall = {
        installDir: document.getElementById("clientInstallDir"),
        configDir: document.getElementById("clientConfigDir"),
        serverAddress: document.getElementById("clientServerAddress"),
        serverPort: document.getElementById("clientServerPort"),
        user: document.getElementById("clientUser"),
        password: document.getElementById("clientPassword"),
        serverName: document.getElementById("clientServerName"),
        allowInsecure: document.getElementById("clientAllowInsecure"),
        pinnedCert: document.getElementById("clientPinnedCert"),
        verifyPeerName: document.getElementById("clientVerifyPeerName"),
        tunEnabled: document.getElementById("clientTunEnabled"),
        tunName: document.getElementById("clientTunName"),
        tunMtu: document.getElementById("clientTunMtu"),
        tunAddr: document.getElementById("clientTunAddr"),
        installButton: document.getElementById("clientInstallButton"),
        resetButton: document.getElementById("clientInstallResetButton"),
        status: document.getElementById("clientInstallStatus"),
        installLink: document.getElementById("clientInstallLink"),
        installLinkButton: document.getElementById("clientInstallLinkButton"),
        linkStatus: document.getElementById("clientInstallLinkStatus"),
        cardLink: document.getElementById("clientInstallCardLink"),
        cardManual: document.getElementById("clientInstallCardManual"),
        panelLink: document.getElementById("clientInstallPanelLink"),
        panelManual: document.getElementById("clientInstallPanelManual")
      };

      const clientDeploy = {
        cardLink: document.getElementById("clientDeployCardLink"),
        cardManual: document.getElementById("clientDeployCardManual"),
        panelLink: document.getElementById("clientDeployPanelLink"),
        panelManual: document.getElementById("clientDeployPanelManual"),
        hostLink: document.getElementById("clientDeployHostLink"),
        portLink: document.getElementById("clientDeployPortLink"),
        linkButton: document.getElementById("clientDeployLinkButton"),
        linkStatus: document.getElementById("clientDeployLinkStatus"),
        host: document.getElementById("clientDeployHost"),
        port: document.getElementById("clientDeployPort"),
        installDir: document.getElementById("clientDeployInstallDir"),
        user: document.getElementById("clientDeployUser"),
        password: document.getElementById("clientDeployPassword"),
        trojanPort: document.getElementById("clientDeployTrojanPort"),
        button: document.getElementById("clientDeployButton"),
        resetButton: document.getElementById("clientDeployResetButton"),
        status: document.getElementById("clientDeployStatus")
      };

      const serverInstall = {
        cardLink: document.getElementById("serverInstallCardLink"),
        cardManual: document.getElementById("serverInstallCardManual"),
        panelLink: document.getElementById("serverInstallPanelLink"),
        panelManual: document.getElementById("serverInstallPanelManual"),
        link: document.getElementById("serverInstallLink"),
        linkButton: document.getElementById("serverInstallLinkButton"),
        linkStatus: document.getElementById("serverInstallLinkStatus"),
        installDir: document.getElementById("serverInstallDir"),
        configDir: document.getElementById("serverConfigDir"),
        port: document.getElementById("serverPort"),
        host: document.getElementById("serverHost"),
        certStore: document.getElementById("serverCertStore"),
        certFile: document.getElementById("serverCertFile"),
        keyFile: document.getElementById("serverKeyFile"),
        button: document.getElementById("serverInstallButton"),
        resetButton: document.getElementById("serverInstallResetButton"),
        status: document.getElementById("serverInstallStatus")
      };

      const serverDeploy = {
        cardLink: document.getElementById("serverDeployCardLink"),
        cardManual: document.getElementById("serverDeployCardManual"),
        panelLink: document.getElementById("serverDeployPanelLink"),
        panelManual: document.getElementById("serverDeployPanelManual"),
        link: document.getElementById("serverDeployLink"),
        linkButton: document.getElementById("serverDeployLinkButton"),
        linkStatus: document.getElementById("serverDeployLinkStatus"),
        linkManual: document.getElementById("serverDeployLinkManual"),
        listen: document.getElementById("serverDeployListen"),
        diagPort: document.getElementById("serverDeployDiagPort"),
        timeout: document.getElementById("serverDeployTimeout"),
        button: document.getElementById("serverDeployButton"),
        resetButton: document.getElementById("serverDeployResetButton"),
        status: document.getElementById("serverDeployStatus")
      };

      function selectClientInstallMode(mode) {
        const linkActive = mode === "link";
        clientInstall.cardLink.classList.toggle("active", linkActive);
        clientInstall.cardManual.classList.toggle("active", !linkActive);
        clientInstall.cardLink.setAttribute("aria-pressed", linkActive ? "true" : "false");
        clientInstall.cardManual.setAttribute("aria-pressed", !linkActive ? "true" : "false");
        clientInstall.panelLink.classList.toggle("hidden", !linkActive);
        clientInstall.panelManual.classList.toggle("hidden", linkActive);
        clearStatus(clientInstall.status);
        clearStatus(clientInstall.linkStatus);
      }

      function selectClientDeployMode(mode) {
        const linkActive = mode === "link";
        clientDeploy.cardLink.classList.toggle("active", linkActive);
        clientDeploy.cardManual.classList.toggle("active", !linkActive);
        clientDeploy.cardLink.setAttribute("aria-pressed", linkActive ? "true" : "false");
        clientDeploy.cardManual.setAttribute("aria-pressed", !linkActive ? "true" : "false");
        clientDeploy.panelLink.classList.toggle("hidden", !linkActive);
        clientDeploy.panelManual.classList.toggle("hidden", linkActive);
        clearStatus(clientDeploy.status);
        clearStatus(clientDeploy.linkStatus);
      }

      function selectServerInstallMode(mode) {
        const linkActive = mode === "link";
        serverInstall.cardLink.classList.toggle("active", linkActive);
        serverInstall.cardManual.classList.toggle("active", !linkActive);
        serverInstall.cardLink.setAttribute("aria-pressed", linkActive ? "true" : "false");
        serverInstall.cardManual.setAttribute("aria-pressed", !linkActive ? "true" : "false");
        serverInstall.panelLink.classList.toggle("hidden", !linkActive);
        serverInstall.panelManual.classList.toggle("hidden", linkActive);
        clearStatus(serverInstall.status);
        clearStatus(serverInstall.linkStatus);
      }

      function selectServerDeployMode(mode) {
        const linkActive = mode === "link";
        serverDeploy.cardLink.classList.toggle("active", linkActive);
        serverDeploy.cardManual.classList.toggle("active", !linkActive);
        serverDeploy.cardLink.setAttribute("aria-pressed", linkActive ? "true" : "false");
        serverDeploy.cardManual.setAttribute("aria-pressed", !linkActive ? "true" : "false");
        serverDeploy.panelLink.classList.toggle("hidden", !linkActive);
        serverDeploy.panelManual.classList.toggle("hidden", linkActive);
        clearStatus(serverDeploy.status);
        clearStatus(serverDeploy.linkStatus);
      }

      function bindCard(card, handler) {
        card.addEventListener("click", handler);
        card.addEventListener("keydown", (ev) => {
          if (ev.key === "Enter" || ev.key === " ") {
            ev.preventDefault();
            handler();
          }
        });
      }

      async function loadClientInstallDefaults() {
        clearStatus(clientInstall.status);
        const api = app();
        if (!api || !api.GetClientInstallDefaults) {
          setStatus(clientInstall.status, "UI bindings are not available.", false);
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
          setStatus(clientInstall.status, "Failed to load defaults: " + (err?.message || err), false);
        }
      }

      async function loadClientDeployDefaults() {
        clearStatus(clientDeploy.status);
        const api = app();
        if (!api || !api.GetClientDeployDefaults) {
          setStatus(clientDeploy.status, "UI bindings are not available.", false);
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
          clientDeploy.hostLink.value = defaults.host || "";
          clientDeploy.portLink.value = defaults.deployPort || "";
        } catch (err) {
          setStatus(clientDeploy.status, "Failed to load defaults: " + (err?.message || err), false);
        }
      }

      async function loadServerInstallDefaults() {
        clearStatus(serverInstall.status);
        const api = app();
        if (!api || !api.GetServerInstallDefaults) {
          setStatus(serverInstall.status, "UI bindings are not available.", false);
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
          setStatus(serverInstall.status, "Failed to load defaults: " + (err?.message || err), false);
        }
      }

      async function loadServerDeployDefaults() {
        clearStatus(serverDeploy.status);
        const api = app();
        if (!api || !api.GetServerDeployDefaults) {
          setStatus(serverDeploy.status, "UI bindings are not available.", false);
          return;
        }
        try {
          const defaults = await api.GetServerDeployDefaults();
          serverDeploy.listen.value = defaults.listen || "";
          serverDeploy.diagPort.value = defaults.diagPort || "";
          serverDeploy.timeout.value = defaults.timeout || "";
        } catch (err) {
          setStatus(serverDeploy.status, "Failed to load defaults: " + (err?.message || err), false);
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

      function buildClientDeployPayload(source) {
        if (source === "link") {
          return {
            host: clientDeploy.hostLink.value.trim(),
            deployPort: clientDeploy.portLink.value.trim(),
            installDir: "",
            user: "",
            password: "",
            trojanPort: ""
          };
        }
        return {
          host: clientDeploy.host.value.trim(),
          deployPort: clientDeploy.port.value.trim(),
          installDir: clientDeploy.installDir.value.trim(),
          user: clientDeploy.user.value.trim(),
          password: clientDeploy.password.value.trim(),
          trojanPort: clientDeploy.trojanPort.value.trim()
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

      function buildServerDeployPayload(source) {
        if (source === "link") {
          return {
            link: serverDeploy.link.value.trim(),
            listen: "",
            diagPort: "",
            timeout: ""
          };
        }
        return {
          link: serverDeploy.linkManual.value.trim(),
          listen: serverDeploy.listen.value.trim(),
          diagPort: serverDeploy.diagPort.value.trim(),
          timeout: serverDeploy.timeout.value.trim()
        };
      }

      async function installClient() {
        clearStatus(clientInstall.status);
        const payload = buildClientInstallPayload();
        if (!payload.serverAddress) {
          setStatus(clientInstall.status, "Server address is required.", false);
          return;
        }
        if (!payload.user) {
          setStatus(clientInstall.status, "User is required.", false);
          return;
        }
        if (!payload.password) {
          setStatus(clientInstall.status, "Password is required.", false);
          return;
        }
        const api = app();
        if (!api || !api.InstallClient) {
          setStatus(clientInstall.status, "UI bindings are not available.", false);
          return;
        }
        clientInstall.installButton.disabled = true;
        try {
          await api.InstallClient(payload);
          setStatus(clientInstall.status, "Client install completed.", true);
        } catch (err) {
          setStatus(clientInstall.status, "Client install failed: " + (err?.message || err), false);
        } finally {
          clientInstall.installButton.disabled = false;
        }
      }

      async function installClientFromLink() {
        clearStatus(clientInstall.linkStatus);
        const linkValue = clientInstall.installLink.value.trim();
        if (!linkValue) {
          setStatus(clientInstall.linkStatus, "Link is required.", false);
          return;
        }
        const api = app();
        if (!api || !api.InstallFromLink) {
          setStatus(clientInstall.linkStatus, "UI bindings are not available.", false);
          return;
        }
        clientInstall.installLinkButton.disabled = true;
        try {
          await api.InstallFromLink(linkValue);
          setStatus(clientInstall.linkStatus, "Link install completed.", true);
        } catch (err) {
          setStatus(clientInstall.linkStatus, "Link install failed: " + (err?.message || err), false);
        } finally {
          clientInstall.installLinkButton.disabled = false;
        }
      }

      async function deployClient(source) {
        const statusNode = source === "link" ? clientDeploy.linkStatus : clientDeploy.status;
        clearStatus(statusNode);
        const payload = buildClientDeployPayload(source);
        if (!payload.host) {
          setStatus(statusNode, "Deploy host is required.", false);
          return;
        }
        const api = app();
        if (!api || !api.DeployClient) {
          setStatus(statusNode, "UI bindings are not available.", false);
          return;
        }
        const button = source === "link" ? clientDeploy.linkButton : clientDeploy.button;
        button.disabled = true;
        try {
          await api.DeployClient(payload);
          setStatus(statusNode, "Client deploy completed.", true);
        } catch (err) {
          setStatus(statusNode, "Client deploy failed: " + (err?.message || err), false);
        } finally {
          button.disabled = false;
        }
      }

      async function installServer(source) {
        const statusNode = source === "link" ? serverInstall.linkStatus : serverInstall.status;
        clearStatus(statusNode);
        const api = app();
        if (!api) {
          setStatus(statusNode, "UI bindings are not available.", false);
          return;
        }
        const button = source === "link" ? serverInstall.linkButton : serverInstall.button;
        button.disabled = true;
        try {
          if (source === "link") {
            const linkValue = serverInstall.link.value.trim();
            if (!linkValue) {
              setStatus(statusNode, "Link is required.", false);
              return;
            }
            if (!api.InstallServerFromLink) {
              setStatus(statusNode, "UI bindings are not available.", false);
              return;
            }
            await api.InstallServerFromLink(linkValue);
            setStatus(statusNode, "Server install completed.", true);
          } else {
            if (!api.InstallServer) {
              setStatus(statusNode, "UI bindings are not available.", false);
              return;
            }
            const payload = buildServerInstallPayload();
            await api.InstallServer(payload);
            setStatus(statusNode, "Server install completed.", true);
          }
        } catch (err) {
          setStatus(statusNode, "Server install failed: " + (err?.message || err), false);
        } finally {
          button.disabled = false;
        }
      }

      async function deployServer(source) {
        const statusNode = source === "link" ? serverDeploy.linkStatus : serverDeploy.status;
        clearStatus(statusNode);
        const payload = buildServerDeployPayload(source);
        if (!payload.link) {
          setStatus(statusNode, "Deploy link is required.", false);
          return;
        }
        const api = app();
        if (!api || !api.DeployServer) {
          setStatus(statusNode, "UI bindings are not available.", false);
          return;
        }
        const button = source === "link" ? serverDeploy.linkButton : serverDeploy.button;
        button.disabled = true;
        try {
          await api.DeployServer(payload);
          setStatus(statusNode, "Server deploy completed.", true);
        } catch (err) {
          setStatus(statusNode, "Server deploy failed: " + (err?.message || err), false);
        } finally {
          button.disabled = false;
        }
      }

      clientInstall.installButton.addEventListener("click", installClient);
      clientInstall.resetButton.addEventListener("click", loadClientInstallDefaults);
      clientInstall.installLinkButton.addEventListener("click", installClientFromLink);

      clientDeploy.linkButton.addEventListener("click", () => deployClient("link"));
      clientDeploy.button.addEventListener("click", () => deployClient("manual"));
      clientDeploy.resetButton.addEventListener("click", loadClientDeployDefaults);

      serverInstall.linkButton.addEventListener("click", () => installServer("link"));
      serverInstall.button.addEventListener("click", () => installServer("manual"));
      serverInstall.resetButton.addEventListener("click", loadServerInstallDefaults);

      serverDeploy.linkButton.addEventListener("click", () => deployServer("link"));
      serverDeploy.button.addEventListener("click", () => deployServer("manual"));
      serverDeploy.resetButton.addEventListener("click", loadServerDeployDefaults);

      bindCard(clientInstall.cardLink, () => selectClientInstallMode("link"));
      bindCard(clientInstall.cardManual, () => selectClientInstallMode("manual"));
      bindCard(clientDeploy.cardLink, () => selectClientDeployMode("link"));
      bindCard(clientDeploy.cardManual, () => selectClientDeployMode("manual"));
      bindCard(serverInstall.cardLink, () => selectServerInstallMode("link"));
      bindCard(serverInstall.cardManual, () => selectServerInstallMode("manual"));
      bindCard(serverDeploy.cardLink, () => selectServerDeployMode("link"));
      bindCard(serverDeploy.cardManual, () => selectServerDeployMode("manual"));

      loadClientInstallDefaults();
      loadClientDeployDefaults();
      loadServerInstallDefaults();
      loadServerDeployDefaults();
