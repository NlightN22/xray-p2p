# Test Environments

## Windows smoke test VM

- Install [VirtualBox](https://www.virtualbox.org/) and [Vagrant](https://developer.hashicorp.com/vagrant/downloads) on the host machine.
- Boot the Windows 10 playground (two VMs: server + client) from the repository root:
  ```bash
  make vagrant-win10              # default (--all) boots both VMs
  make vagrant-win10 --server     # server only
  make vagrant-win10 --client     # client only
  ```
  Each guest uses the `gusztavvargadr/windows-10` box (version `2202.0.2505`), syncs the repo into `C:\xp2p`, installs Go/xp2p, and finishes provisioning with the smoke test (`xp2p ping 127.0.0.1 --port 62022`).
- Networking:
  - NAT interface remains for outbound internet access.
  - Host-only subnet `10.0.10.0/24`: server `10.0.10.1`, client `10.0.10.10`. The profile is forced to Private and firewall rules for XP2P are pre-created.
- Host access:
  - Server SSH: `ssh vagrant@localhost -p 55922` (`vagrant/vagrant`).
  - Client SSH: `ssh vagrant@localhost -p 55923` (`vagrant/vagrant`).
- Re-run provisioning/tests:
  ```bash
  cd infra/vagrant-win/windows10
  vagrant provision win10-server
  vagrant provision win10-client
  ```
- Cleanup (flags `--server`, `--client`, `--all` apply):
  ```bash
  make vagrant-win10-destroy
  ```
- Optional: `XP2P_GO_VERSION=1.22.3 make vagrant-win10 --client` pins a specific Go toolchain for that VM.
