def define_clients(config)
  client_names = %w[c1 c2 c3]

  client_names.each do |name|
    suffix       = name.delete_prefix('c').to_i        # c1 -> 1, c2 -> 2
    router_name  = "r#{suffix}"                        # r1, r2, ...
    network_name = "#{router_name}_net_router"        
    ssh_host_port = 2230 + suffix           # c1 -> 2231, c2 -> 2232, c3 -> 2233

    config.vm.define name do |c|
        c.vm.box      = "generic/alpine318"
        c.vm.hostname = name

        c.vm.network "forwarded_port",
                    guest: 22,  host: ssh_host_port,   id: "ssh"                            

        c.vm.network "private_network",
                    virtualbox__intnet: network_name,
                    adapter: 2,
                    auto_config: false

        c.vm.provision "shell", name: "Config eth1 & DHCP", run: "always", inline: <<-SHELL
            set -ex
            ip link set eth1 up
            udhcpc -i eth1 -t 5 -T 5 -n
            apk add --no-cache curl
            if ! ip route del default dev eth0 2>/dev/null; then
                if command -v sudo >/dev/null 2>&1; then
                    sudo ip route del default dev eth0 || true
                fi
            fi
        SHELL

        c.vm.provision "shell", name: "Persist eth0 route removal", run: "always", inline: <<-SHELL
            set -ex
            mkdir -p /etc/network/interfaces.d
            cat <<'EOF' >/etc/network/interfaces.d/eth0-xray
auto eth0
iface eth0 inet dhcp
    udhcpc_opts -o
    post-up ip route del default dev $IFACE 2>/dev/null || true
    post-down ip route del default dev $IFACE 2>/dev/null || true
EOF
            if command -v rc-service >/dev/null 2>&1; then
                if rc-service networking status >/dev/null 2>&1; then
                    ifdown eth0 || true
                    ifup eth0 || true
                fi
            fi
            ip route del default dev eth0 2>/dev/null || true
        SHELL

        c.vm.provision "shell", name: "Check default route", run: "always", inline: <<-SHELL
            set -ex
            routes=$(ip route show | grep default)
            count=$(printf '%s\n' "$routes" | wc -l)

            if [ "$count" -ne 1 ] || ! printf '%s\n' "$routes" | grep -q "dev eth1"; then
                echo "Error: expected exactly one default route via eth1, found $count:"
                printf '%s\n' "$routes"
                exit 1
            fi

            echo "OK: single default route via eth1 - $routes"
        SHELL

        c.vm.provision "shell", name: "Install iperf3", inline: <<-SHELL
            set -ex
            if command -v iperf3 >/dev/null 2>&1; then
                echo "iperf3 is already installed at $(command -v iperf3)"
                exit 0
            fi
            apk update
            apk add --no-cache iperf3
        SHELL

        c.vm.provision "file",
            source: "./dnsmasq-install-alpine.sh",
            destination: "/tmp/dnsmasq-install-alpine.sh"

        c.vm.provision "file",
            source: "./iperf3.init",
            destination: "/tmp/iperf3.init"

        c.vm.provision "shell", name: "Ensure iperf3 service", inline: <<-SHELL
            set -ex
            install -m 755 -o root -g root /tmp/iperf3.init /etc/init.d/iperf3
            rc-update add iperf3 default
            rc-service iperf3 restart || rc-service iperf3 start
        SHELL

        c.vm.provision "shell", name: "Delete root password and allow ssh", inline: <<-SHELL
            set -ex
            passwd -d root

            sed -i 's/^#*PermitRootLogin.*/PermitRootLogin yes/' /etc/ssh/sshd_config
            sed -i 's/^#*PermitEmptyPasswords.*/PermitEmptyPasswords yes/' /etc/ssh/sshd_config
            if ! grep -q '^PermitRootLogin' /etc/ssh/sshd_config; then
                echo 'PermitRootLogin yes' >> /etc/ssh/sshd_config
            fi
            if ! grep -q '^PermitEmptyPasswords' /etc/ssh/sshd_config; then
                echo 'PermitEmptyPasswords yes' >> /etc/ssh/sshd_config
            fi

            rc-service sshd restart
        SHELL
    end
  end
end
