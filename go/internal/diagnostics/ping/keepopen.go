package ping

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

func runKeepOpen(ctx context.Context, targetAddr string, timeout time.Duration, opts Options) error {
	conn, err := dialTCP(ctx, targetAddr, opts.SocksProxy, timeout)
	if err != nil {
		return err
	}
	defer conn.Close()

	count := opts.Count
	if count < minCount {
		count = 4
	}

	var sent, received int
	logger := logging.With("target", targetAddr, "proto", protoTCP, "mode", "keep-open")
	if opts.SocksProxy != "" {
		logger = logging.With("target", targetAddr, "proto", protoTCP, "mode", "keep-open", "socks_proxy", opts.SocksProxy)
	}
	logger.Debug("keep-open ping session started", "count", count, "timeout", timeout)

	for seq := 1; opts.Continuous || seq <= count; seq++ {
		select {
		case <-ctx.Done():
			return finishInterrupted(ctx, opts, logger, sent, received)
		default:
		}

		rtt, err := exchangeTCPRequest(ctx, conn, targetAddr, keepOpenRequest, timeout, seq, opts.Reporter)
		sent++
		if err != nil {
			if !opts.Silent {
				fmt.Printf("Open connection failed at request %d: %v\n", seq, err)
			}
			logger.Warn("keep-open ping request failed", "seq", seq, "err", err)
			if received == 0 {
				return errors.New("no replies received")
			}
			return err
		}

		received++
		if !opts.Silent {
			fmt.Printf("Reply from %s: seq=%d time=%s proto=%s mode=keep-open\n",
				targetAddr, seq, rtt.Round(time.Millisecond), protoTCP)
		}
		logger.Debug("keep-open ping reply received", "seq", seq, "rtt", rtt)

		if opts.Continuous || seq < count {
			select {
			case <-time.After(1 * time.Second):
			case <-ctx.Done():
				return finishInterrupted(ctx, opts, logger, sent, received)
			}
		}
	}

	if !opts.Silent {
		printSummary(sent, received)
	}
	logger.Info("keep-open ping session completed", "sent", sent, "received", received)
	if received == 0 {
		return errors.New("no replies received")
	}
	return nil
}

func finishInterrupted(ctx context.Context, opts Options, logger *slog.Logger, sent, received int) error {
	if !opts.Silent {
		fmt.Println("interrupted")
	}
	if opts.Silent {
		logger.Debug("ping session interrupted", "sent", sent, "received", received)
	} else {
		logger.Info("ping session interrupted", "sent", sent, "received", received)
	}
	return ctx.Err()
}
