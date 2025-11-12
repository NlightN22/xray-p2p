def define_routers(config)
  ip_prefix    = '10.0.0.'
  router_names = %w[r1 r2 r3]

  router_names.each do |name|
    suffix = name.delete_prefix('r').to_i

    ip               = "#{ip_prefix}#{suffix}"  # 10.0.0.1, 10.0.0.2, ...
    ssh_host_port    = 2220 + suffix            # 2221, 2222, ...
    https_host_port  = 8440 + suffix            # 8441, 8442, ...

    config.vm.define name do |r|
      r.vm.hostname = name

      r.vm.network "private_network",
        virtualbox__intnet: "#{name}_net_router", auto_config: false
      r.vm.network "private_network",
        virtualbox__intnet: "tunnel_net", auto_config: false

      r.vm.network "forwarded_port",
        guest: 22,  host: ssh_host_port,   id: "ssh"
      r.vm.network "forwarded_port",
        guest: 443, host: https_host_port, id: "https"

      r.ssh.username = "root"
      r.ssh.shell    = "/bin/ash"

      r.vm.provision "shell",
        path: "./router-provision.sh",
        args: [ip, suffix],
        privileged: false

      r.vm.provision "shell", name: "Check tunnel.ipaddr", privileged: false, inline: <<-SHELL
        TARGET=#{ip}
        for i in $(seq 1 5); do
          CURRENT=$(uci get network.tunnel.ipaddr 2>/dev/null || echo)
          if [ "$CURRENT" = "$TARGET" ]; then
            echo "OK: tunnel.ipaddr is $TARGET"
            exit 0
          fi
          echo "waiting for tunnel.ipaddr ($i/5): got '$CURRENT'"
          sleep 1
        done
        echo "ERROR: timeout waiting for tunnel.ipaddr to become $TARGET" >&2
        exit 1
      SHELL

      r.vm.provision "shell", name: "Install iperf3", privileged: false, inline: <<-SHELL
        set -ex
        if command -v iperf3 >/dev/null 2>&1; then
            echo "iperf3 is already installed at $(command -v iperf3)"
            exit 0
        fi
        opkg update
        opkg install iperf3
      SHELL

      r.vm.provision "shell", name: "Install OpenSSH client", privileged: false, inline: <<-SHELL
        set -ex
        if command -v ssh >/dev/null 2>&1 && ! ssh -V 2>&1 | grep -iq dropbear; then
            echo "OpenSSH client already installed"
            exit 0
        fi
        opkg update
        opkg install openssh-client >/dev/null 2>&1 || opkg install openssh-client
      SHELL

      r.vm.provision "shell", name: "Configure SSH host exceptions", privileged: false, inline: <<-SHELL
        set -ex
        CONFIG_DIR="/root/.ssh"
        CONFIG_FILE="$CONFIG_DIR/config"
        mkdir -p "$CONFIG_DIR"
        touch "$CONFIG_FILE"
        if ! grep -q '^Host 10\\.0\\.0\\.\\*$' "$CONFIG_FILE"; then
          cat <<'EOF' >> "$CONFIG_FILE"
Host 10.0.0.*
    StrictHostKeyChecking no
    UserKnownHostsFile /dev/null
EOF
          chmod 600 "$CONFIG_FILE"
        fi
      SHELL

      r.vm.provision "file",
        source: "./iperf3.openwrt.init",
        destination: "/tmp/iperf3.init"

      r.vm.provision "shell", name: "Ensure iperf3 service", privileged: false, inline: <<-SHELL
        set -ex
        cp /tmp/iperf3.init /etc/init.d/iperf3
        chown root:root /etc/init.d/iperf3
        chmod 755 /etc/init.d/iperf3
        /etc/init.d/iperf3 enable
        /etc/init.d/iperf3 stop >/dev/null 2>&1 || true
        /etc/init.d/iperf3 start
      SHELL

    end
  end
end


