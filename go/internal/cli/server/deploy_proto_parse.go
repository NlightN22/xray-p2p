package servercmd

import (
	"strings"

	deploylink "github.com/NlightN22/xray-p2p/go/internal/deploy/link"
	"github.com/NlightN22/xray-p2p/go/internal/netutil"
)

func parseDeployLink(raw string) (deploylink.EncryptedLink, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return deploylink.EncryptedLink{}, nil
	}

	enc, err := deploylink.Parse(raw)
	if err != nil {
		return deploylink.EncryptedLink{}, err
	}
	if err := netutil.ValidateHost(enc.Host); err != nil {
		return deploylink.EncryptedLink{}, err
	}
	return enc, nil
}
