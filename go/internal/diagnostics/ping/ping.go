package ping

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base32"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
	ownedhttp "github.com/NlightN22/xray-p2p/go/internal/nethttp"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

// Options describes how Ping should behave.
type Options struct {
	Count                int
	Timeout              time.Duration
	Port                 int
	SocksProxy           string
	User                 string
	Credential           string
	ServerName           string
	AllowInsecure        bool
	PinnedPeerCertSHA256 string
	Continuous           bool
	Reporter             Reporter
	Silent               bool
	HTTPClient           ownedhttp.Doer
}

// Reporter is invoked when an HTTPS ping succeeds.
type Reporter interface {
	Report(ctx context.Context, result Result) error
}

// Result captures statistics associated with a single ping request.
type Result struct {
	Seq    int
	Target string
	Proto  string
	RTT    time.Duration
}

const (
	defaultTimeout = 3 * time.Second
	minCount       = 1
	protocolHTTPS  = "https"
)

// Run performs application-level ping against the xp2p service.
func Run(ctx context.Context, target string, opts Options) error {
	if target == "" {
		return errors.New("ping target is required")
	}

	count := opts.Count
	if count < minCount {
		count = 4
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	if opts.HTTPClient == nil {
		tlsConfig := &tls.Config{
			ServerName:         opts.ServerName,
			InsecureSkipVerify: opts.AllowInsecure || opts.PinnedPeerCertSHA256 != "",
		}
		if opts.PinnedPeerCertSHA256 != "" {
			tlsConfig.VerifyPeerCertificate = func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
				return verifyPinnedPeerCertificate(rawCerts, opts.PinnedPeerCertSHA256)
			}
		}
		client := ownedhttp.NewClient(ownedhttp.ClientOptions{
			Timeout:   timeout,
			TLSConfig: tlsConfig,
			Dialer:    newPingDialer(opts.SocksProxy, timeout),
		})
		defer shutdownHTTPClient(client)
		opts.HTTPClient = client
	}

	port := opts.Port
	if port == 0 {
		p, err := strconv.Atoi(server.DefaultPort)
		if err != nil {
			return fmt.Errorf("invalid default port: %s", server.DefaultPort)
		}
		port = p
	}

	targetAddr := fmt.Sprintf("%s:%d", target, port)

	var sent, received int
	fields := []any{"target", targetAddr, "protocol", protocolHTTPS}
	if opts.SocksProxy != "" {
		fields = append(fields, "socks_proxy", opts.SocksProxy)
	}
	logger := logging.With(fields...)
	logger.Debug("ping session started", "count", count, "timeout", timeout)
	for seq := 1; opts.Continuous || seq <= count; seq++ {
		select {
		case <-ctx.Done():
			if !opts.Silent {
				fmt.Println("interrupted")
			}
			if opts.Silent {
				logger.Debug("ping session interrupted", "sent", sent, "received", received)
			} else {
				logger.Info("ping session interrupted", "sent", sent, "received", received)
			}
			return ctx.Err()
		default:
		}

		logger.Debug("sending HTTPS control ping", "seq", seq)
		rtt, err := pingHTTPS(ctx, targetAddr, timeout, seq, opts)

		sent++
		if err != nil {
			if !opts.Silent {
				fmt.Printf("Request %d failed: %v\n", seq, err)
			}
			if opts.Silent {
				logger.Debug("ping request failed", "seq", seq, "err", err)
			} else {
				logger.Warn("ping request failed", "seq", seq, "err", err)
			}
		} else {
			received++
			formatted := rtt.Round(time.Millisecond)
			if !opts.Silent {
				fmt.Printf("Reply from %s: seq=%d time=%s proto=%s\n",
					targetAddr, seq, formatted, protocolHTTPS)
			}
			logger.Debug("ping reply received", "seq", seq, "rtt", rtt)
		}

		if opts.Continuous || seq < count {
			select {
			case <-time.After(1 * time.Second):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	if !opts.Silent {
		printSummary(sent, received)
	}
	if opts.Silent {
		logger.Debug("ping session completed", "sent", sent, "received", received)
	} else {
		logger.Info("ping session completed", "sent", sent, "received", received)
	}
	if received == 0 {
		return errors.New("no replies received")
	}
	return nil
}

func shutdownHTTPClient(client ownedhttp.OwnedClient) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	_ = client.Shutdown(ctx)
}

func newNonce() (string, error) {
	raw := make([]byte, 10)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	enc := base32.StdEncoding.WithPadding(base32.NoPadding)
	return enc.EncodeToString(raw), nil
}

func printSummary(sent, received int) {
	lost := sent - received
	var lossPercent float64
	if sent > 0 {
		lossPercent = float64(lost) / float64(sent) * 100
	}
	fmt.Printf("\nPackets: sent = %d, received = %d, lost = %d (%.0f%% loss)\n",
		sent, received, lost, lossPercent)
}
