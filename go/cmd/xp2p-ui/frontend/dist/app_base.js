(() => {
  const ui = {};

  ui.app = () => (window.go && window.go.ui && window.go.ui.App ? window.go.ui.App : null);

  ui.setStatus = (node, message, ok) => {
    node.textContent = message;
    node.className = "status" + (ok ? " ok" : " error");
  };

  ui.clearStatus = (node) => {
    node.textContent = "";
    node.className = "status";
  };

  ui.showScenario = (id) => {
    const sections = Array.from(document.querySelectorAll(".scenario"));
    let found = false;
    sections.forEach((section) => {
      const active = section.id === id;
      section.classList.toggle("active", active);
      if (active) {
        found = true;
      }
    });
    if (!found && sections.length > 0) {
      sections[0].classList.add("active");
    }
  };

  ui.bindCard = (card, handler) => {
    card.addEventListener("click", handler);
    card.addEventListener("keydown", (ev) => {
      if (ev.key === "Enter" || ev.key === " ") {
        ev.preventDefault();
        handler();
      }
    });
  };

  ui.clientInstall = {
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

  ui.serverInstall = {
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

  ui.clientDeploy = {
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

  ui.serverDeploy = {
    link: document.getElementById("serverDeployLink"),
    listen: document.getElementById("serverDeployListen"),
    diagPort: document.getElementById("serverDeployDiagPort"),
    timeout: document.getElementById("serverDeployTimeout"),
    button: document.getElementById("serverDeployButton"),
    resetButton: document.getElementById("serverDeployResetButton"),
    status: document.getElementById("serverDeployStatus")
  };

  ui.selectClientInstallMode = (mode) => {
    const linkActive = mode === "link";
    ui.clientInstall.cardLink.classList.toggle("active", linkActive);
    ui.clientInstall.cardManual.classList.toggle("active", !linkActive);
    ui.clientInstall.cardLink.setAttribute("aria-pressed", linkActive ? "true" : "false");
    ui.clientInstall.cardManual.setAttribute("aria-pressed", !linkActive ? "true" : "false");
    ui.clientInstall.panelLink.classList.toggle("hidden", !linkActive);
    ui.clientInstall.panelManual.classList.toggle("hidden", linkActive);
    ui.clearStatus(ui.clientInstall.status);
    ui.clearStatus(ui.clientInstall.linkStatus);
  };

  if (window.runtime && window.runtime.EventsOn) {
    window.runtime.EventsOn("xp2p-ui:open", (name) => ui.showScenario(name));
  }

  ui.showScenario("client-install");
  window.ui = ui;
})();
