package forward

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"syscall"
)

// ErrPortUnavailable indicates the address is already bound.
var ErrPortUnavailable = errors.New("port unavailable")

// CheckPort ensures that the provided listener address is available for all requested protocols.
func CheckPort(listen string, port int, proto Protocol) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid --listen-port %d", port)
	}
	if proto.RequiresTCP() {
		if err := probeTCP(listen, port); err != nil {
			return err
		}
	}
	if proto.RequiresUDP() {
		if err := probeUDP(listen, port); err != nil {
			return err
		}
	}
	return nil
}

// FindAvailablePort searches for the first available port from start..65535 skipping reserved entries.
func FindAvailablePort(listen string, start int, proto Protocol, reserved map[int]struct{}) (int, error) {
	if start < 1 {
		start = DefaultBasePort
	}
	for port := start; port <= 65535; port++ {
		if _, taken := reserved[port]; taken {
			continue
		}
		if err := CheckPort(listen, port, proto); err != nil {
			if errors.Is(err, ErrPortUnavailable) {
				continue
			}
			return 0, err
		}
		return port, nil
	}
	return 0, fmt.Errorf("no free ports available from %d to 65535", start)
}

func probeTCP(listen string, port int) error {
	addr := net.JoinHostPort(listen, strconv.Itoa(port))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		if isAddrUnavailable(err) {
			return ErrPortUnavailable
		}
		return fmt.Errorf("bind TCP %s: %w", addr, err)
	}
	return ln.Close()
}

func probeUDP(listen string, port int) error {
	ip := net.ParseIP(listen)
	if ip == nil {
		return fmt.Errorf("invalid listen address %q", listen)
	}
	addr := &net.UDPAddr{IP: ip, Port: port}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		if isAddrUnavailable(err) {
			return ErrPortUnavailable
		}
		return fmt.Errorf("bind UDP %s: %w", addr.String(), err)
	}
	return conn.Close()
}

func isAddrInUse(err error) bool {
	if errors.Is(err, syscall.EADDRINUSE) {
		return true
	}
	var opErr *net.OpError
	if !errors.As(err, &opErr) {
		return false
	}
	var syscallErr *os.SyscallError
	if !errors.As(opErr.Err, &syscallErr) {
		return false
	}
	return errors.Is(syscallErr.Err, syscall.EADDRINUSE)
}

func isAddrUnavailable(err error) bool {
	if isAddrInUse(err) {
		return true
	}
	if errors.Is(err, syscall.EACCES) || errors.Is(err, syscall.EPERM) {
		return true
	}
	if errno, ok := unwrapSyscallErrno(err); ok && errno == 10013 {
		return true
	}
	return false
}

func unwrapSyscallErrno(err error) (syscall.Errno, bool) {
	if err == nil {
		return 0, false
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		err = opErr.Err
	}
	var syscallErr *os.SyscallError
	if errors.As(err, &syscallErr) {
		err = syscallErr.Err
	}
	if errno, ok := err.(syscall.Errno); ok {
		return errno, true
	}
	return 0, false
}
