package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	defaultTarget   = "10.0.10.10"
	defaultPort     = "62022"
	defaultAttempts = 3
	xp2pExecutable  = `C:\tools\xp2p\xp2p.exe`
)

func main() {
	target := flag.String("target", defaultTarget, "Target address to ping through xp2p")
	port := flag.String("port", defaultPort, "TCP port for xp2p ping")
	attempts := flag.Int("attempts", defaultAttempts, "Number of ping attempts")
	flag.Parse()

	if err := runXP2PPing(*target, *port, *attempts); err != nil {
		fmt.Fprintf(os.Stderr, "xp2p ping failed: %v\n", err)
		os.Exit(1)
	}
}

func runXP2PPing(target, port string, attempts int) error {
	if _, err := os.Stat(xp2pExecutable); err != nil {
		return fmt.Errorf("xp2p executable not found at %s: %w", xp2pExecutable, err)
	}

	args := []string{
		"ping",
		target,
		"--port", port,
		"--attempts", fmt.Sprintf("%d", attempts),
	}

	cmd := exec.Command(xp2pExecutable, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = filepath.Dir(xp2pExecutable)

	if err := cmd.Run(); err != nil {
		// Provide a cleaner message if the process exited with a failure status.
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("xp2p returned non-zero exit code (%d)", exitErr.ExitCode())
		}
		return fmt.Errorf("failed to execute xp2p: %w", err)
	}

	return nil
}
