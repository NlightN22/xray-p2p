package client

import (
	"net"
	"runtime"
	"strconv"
	"strings"
)

func detectLocalIP(targetHost string) string {
	targetIP := net.ParseIP(strings.TrimSpace(targetHost))
	if targetIP != nil {
		targetIP = targetIP.To4()
	}

	candidates := make([]string, 0, 4)

	ifaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range ifaces {
			if (iface.Flags&net.FlagUp) == 0 || (iface.Flags&net.FlagLoopback) != 0 {
				continue
			}
			addrs, _ := iface.Addrs()
			for _, addr := range addrs {
				var ipnet *net.IPNet
				switch v := addr.(type) {
				case *net.IPNet:
					ipnet = v
				case *net.IPAddr:
					ipnet = &net.IPNet{IP: v.IP, Mask: net.CIDRMask(32, 32)}
				}
				if ipnet == nil || ipnet.IP == nil || ipnet.IP.IsLoopback() {
					continue
				}
				ip := ipnet.IP.To4()
				if ip == nil {
					continue
				}
				if targetIP != nil && ipnet.Contains(targetIP) {
					if runtime.GOOS == "windows" {
						return ip.String()
					}
					candidates = append(candidates, ip.String())
					continue
				}
				candidates = append(candidates, ip.String())
			}
		}
	}
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err == nil {
		defer conn.Close()
		if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok && addr.IP != nil {
			if v4 := addr.IP.To4(); v4 != nil {
				candidates = append(candidates, v4.String())
			}
		}
	}

	if runtime.GOOS != "windows" {
		for _, ip := range candidates {
			if strings.HasPrefix(ip, "10.0.2.") {
				return ip
			}
		}
	}

	for _, ip := range candidates {
		if runtime.GOOS == "windows" && strings.HasPrefix(ip, "10.0.2.") {
			continue
		}
		if isPrivateIPv4(ip) {
			return ip
		}
	}
	for _, ip := range candidates {
		if runtime.GOOS == "windows" && strings.HasPrefix(ip, "10.0.2.") {
			continue
		}
		if strings.HasPrefix(ip, "169.254.") {
			continue
		}
		return ip
	}
	return "127.0.0.1"
}

func isPrivateIPv4(ip string) bool {
	if strings.HasPrefix(ip, "10.") {
		return true
	}
	if strings.HasPrefix(ip, "192.168.") {
		return true
	}
	if strings.HasPrefix(ip, "172.") {
		parts := strings.Split(ip, ".")
		if len(parts) >= 2 {
			if oct, err := strconv.Atoi(parts[1]); err == nil && oct >= 16 && oct <= 31 {
				return true
			}
		}
	}
	return false
}
