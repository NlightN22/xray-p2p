package clientcmd

import (
	"fmt"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/tunnel"
)

type trojanLink = tunnel.Link

func parseTrojanLink(raw string) (trojanLink, error) {
	link, err := tunnel.ParseLink(raw)
	if err != nil {
		return trojanLink{}, err
	}
	if !strings.EqualFold(link.Endpoint.Protocol, "trojan") {
		return trojanLink{}, fmt.Errorf("connection link protocol %q is not trojan", link.Endpoint.Protocol)
	}
	if strings.TrimSpace(link.User.UserLabel) == "" {
		return trojanLink{}, fmt.Errorf("connection link missing user label (expected #label or email/remarks query parameter)")
	}
	return link, nil
}
