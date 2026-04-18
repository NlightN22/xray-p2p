package servercmd

import (
	"bufio"
	"context"
	"net"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

func (s *deployServer) waitForCompletion(conn net.Conn, rw *bufio.ReadWriter, results chan<- runSignal, installDir, configDir string) {
	if conn != nil {
		_ = conn.SetDeadline(time.Now().Add(serverDeployCompletionTimeout))
	}
	line, err := readLine(rw)
	status := ""
	if err != nil {
		logging.Warn("xp2p server deploy: completion wait failed", "err", err)
		status = "error"
	} else if !strings.HasPrefix(line, "COMPLETE") {
		logging.Warn("xp2p server deploy: unexpected completion signal", "line", line)
		status = strings.TrimSpace(line)
	} else {
		status = strings.TrimSpace(strings.TrimPrefix(line, "COMPLETE"))
		if ackErr := writeLine(rw, "BYE"); ackErr != nil {
			logging.Warn("xp2p server deploy: completion ack failed", "err", ackErr)
		}
	}
	applyHandled := false
	if s.Once {
		s.applyTunAndStartService(context.Background(), installDir, configDir)
		applyHandled = true
	}
	if results != nil {
		results <- runSignal{completed: true, status: status, applyHandled: applyHandled}
	}
}
