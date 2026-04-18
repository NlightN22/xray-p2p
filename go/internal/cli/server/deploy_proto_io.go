package servercmd

import (
	"bufio"
	"strings"
	"time"
)

const (
	serverDeployIOTimeout         = 60 * time.Second
	serverDeployCompletionTimeout = 5 * time.Minute
)

func readLine(rw *bufio.ReadWriter) (string, error) {
	s, err := rw.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(s, "\r\n"), nil
}

func writeLine(rw *bufio.ReadWriter, line string) error {
	if _, err := rw.WriteString(line + "\n"); err != nil {
		return err
	}
	return rw.Flush()
}

func writeSegment(rw *bufio.ReadWriter, begin, end string, lines []string) error {
	if err := writeLine(rw, begin); err != nil {
		return err
	}
	for _, l := range lines {
		if _, err := rw.WriteString(l + "\n"); err != nil {
			return err
		}
	}
	if err := rw.Flush(); err != nil {
		return err
	}
	return writeLine(rw, end)
}

func notifyFailure(results chan<- runSignal) {
	if results != nil {
		results <- runSignal{ok: false}
	}
}
