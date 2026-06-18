//go:build linux

package xrayguard

import (
	"bufio"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var ErrUnsupported = errors.New("xrayguard collector is not supported on this platform")

type procCollector struct{}

func DefaultCollector() Collector {
	return procCollector{}
}

func (procCollector) Sample(ctx context.Context, pid int) (Sample, error) {
	if err := ctx.Err(); err != nil {
		return Sample{}, err
	}

	fdCount, socketFDCount, err := countFDs(pid)
	if err != nil {
		return Sample{}, err
	}
	established, _ := countEstablishedTCP()
	return Sample{
		Timestamp:           time.Now().UTC(),
		FDCount:             fdCount,
		SocketFDCount:       socketFDCount,
		EstablishedTCPCount: established,
	}, nil
}

func countFDs(pid int) (int, int, error) {
	entries, err := os.ReadDir(filepath.Join("/proc", itoa(pid), "fd"))
	if err != nil {
		return 0, 0, err
	}

	socketCount := 0
	for _, entry := range entries {
		target, err := os.Readlink(filepath.Join("/proc", itoa(pid), "fd", entry.Name()))
		if err == nil && strings.HasPrefix(target, "socket:[") {
			socketCount++
		}
	}
	return len(entries), socketCount, nil
}

func countEstablishedTCP() (int, error) {
	total := 0
	for _, path := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		n, err := countEstablishedTCPFile(path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return total, err
		}
		total += n
	}
	return total, nil
}

func countEstablishedTCPFile(path string) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	count := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) > 3 && fields[3] == "01" {
			count++
		}
	}
	return count, scanner.Err()
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
