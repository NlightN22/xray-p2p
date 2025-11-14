# Test Environments

## Windows smoke test VM

- Install VirtualBox and Vagrant on the host machine.
- Boot the Windows 10 playground (two VMs: server + client) from the repository root:
  make vagrant-win10              # default (--all) boots both VMs
  make vagrant-win10 --server     # server only
  make vagrant-win10 --client     # client only

  Each guest uses the gusztavvargadr/windows-10 box (version 2202.0.2505), syncs the repo into C:\\xp2p, installs Go/xp2p, and finishes provisioning with the smoke test (xp2p ping 127.0.0.1 --port 62022).
- Networking:
  - NAT interface remains for outbound internet access.
  - Host-only subnet 10.0.10.0/24: server 10.0.10.1, client 10.0.10.10. The profile is forced to Private and firewall rules for XP2P are pre-created.
- Host access:
  - WinRM (plaintext): server localhost:55985, client localhost:55986 (vagrant/vagrant).
- Re-run provisioning/tests:
  cd infra/vagrant/windows10
  vagrant provision win10-server
  vagrant provision win10-client
- Cleanup (flags --server, --client, --all apply):
  make vagrant-win10-destroy
- Optional: XP2P_GO_VERSION=1.22.3 make vagrant-win10 --client pins a specific Go toolchain for that VM.

## Debian deb-test VM trio

- Install VirtualBox and Vagrant on the host.
- Boot the Debian 12 playground (three guests) manually:
  cd infra/vagrant/debian12/deb-test
  vagrant up

  The Vagrantfile defines `deb-test-a/b/c`. Each guest uses `generic/debian12` (4.3.12), gets 4 vCPUs / 4 GB RAM, attaches the repo as `/srv/xray-p2p`, and receives a host-only address in the 10.62.10.0/24 subnet.
- Provisioning installs build prerequisites (Go, build-essential, rsync, etc.), ensures the shared repo is available under `/srv/xray-p2p`, and leaves xp2p installation/testing to the host-side pytest suite so different scenarios can reuse the machines without reprovisioning.
- Re-run provisioning per machine if needed:
  vagrant provision deb-test-a
- Cleanup:
  vagrant destroy

## Host integration tests

- Prerequisites:
  - Windows: boot both guests (make vagrant-win10) so that C:\\xp2p is available inside each VM.
  - Linux: boot all three Debian guests from infra/vagrant/debian12/deb-test and wait for provisioning to finish.
- Execution:
  - Windows suite: pytest tests/host/win. These tests build MSI packages inside the guests, install xp2p into `C:\Program Files\xp2p`, manage services, and exercise client/server install/update flows via WinRM.
  - Linux suite: pytest tests/host/linux. The helpers connect over SSH, work directly from `/srv/xray-p2p`, build xp2p from source with Go, install the binary into `/usr/local/bin`, and verify `xp2p --version` on every Debian VM. These checks are the base layer for future multi-role scenarios.
- Both suites rely on the shared helpers under tests/host/common.py for Vagrant orchestration.
