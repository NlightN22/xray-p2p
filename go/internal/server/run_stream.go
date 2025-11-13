package server

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

func streamPipe(r io.Reader, stream string, extra io.Writer) {
	logger := logging.Logger()
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if extra != nil {
			if _, err := fmt.Fprintln(extra, line); err != nil {
				logger.Error(fmt.Sprintf("xray_core log file write error: %v", err))
			}
		}
		if stream == "stderr" {
			logger.Error(fmt.Sprintf("xray_core %s: %s", stream, line))
		} else {
			logger.Info(fmt.Sprintf("xray_core %s: %s", stream, line))
		}
	}
	if err := scanner.Err(); err != nil {
		logger.Error(fmt.Sprintf("xray_core stream error: %v", err))
	}
}
