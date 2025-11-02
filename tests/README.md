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
  cd infra/vagrant-win/windows10
  vagrant provision win10-server
  vagrant provision win10-client
- Cleanup (flags --server, --client, --all apply):
  make vagrant-win10-destroy
- Optional: XP2P_GO_VERSION=1.22.3 make vagrant-win10 --client pins a specific Go toolchain for that VM.

## Host integration tests

- Prerequisites: boot both guests (make vagrant-win10) and ensure the repository is built so that C:\\xp2p\\build\\windows-amd64\\xp2p.exe exists in each VM.
- Execution: run pytest tests/host from the repository root. Individual suites:
  - tests/host/test_client.py - provisions the client tree under C:\\Program Files\\xp2p, verifies templated configs, re-runs xp2p client install --force with overrides, and confirms xp2p client run spawns xray-core while creating logs.
  - tests/host/test_server.py - provisions the server tree under C:\\Program Files\\xp2p, deploys TLS assets, ensures --force refreshes configs/certificates, and checks xp2p server run launches xray-core and writes logs.
- Both suites operate through WinRM via testinfra helpers in tests/host/_env.py, which stage binaries in C:\\Program Files\\xp2p and clean up installations between tests.