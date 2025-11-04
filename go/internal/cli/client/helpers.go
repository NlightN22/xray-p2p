package clientcmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

var promptYesNoFunc = promptYesNo

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func promptYesNo(question string) (bool, error) {
	fmt.Printf("%s [Y/n]: ", question)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	answer := strings.TrimSpace(strings.ToLower(line))
	if answer == "" || answer == "y" || answer == "yes" {
		return true, nil
	}
	if answer == "n" || answer == "no" {
		return false, nil
	}
	fmt.Println("Please answer 'y' or 'n'.")
	return promptYesNo(question)
}

func startDiagnostics(ctx context.Context, port string) context.CancelFunc {
	bgCtx, cancel := context.WithCancel(ctx)
	if err := server.StartBackground(bgCtx, server.Options{Port: port}); err != nil {
		cancel()
		logging.Warn("xp2p diagnostics: failed to start ping responders", "port", port, "err", err)
		return nil
	}
	return cancel
}
