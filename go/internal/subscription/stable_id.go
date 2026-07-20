package subscription

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/tunnel"
)

func stableOfferID(source SourceRef, endpoint tunnel.Endpoint) string {
	values := url.Values{}
	values.Set("flow", strings.TrimSpace(endpoint.Metadata["flow"]))
	values.Set("host", strings.ToLower(strings.TrimSpace(endpoint.Host)))
	values.Set("profile", strings.ToLower(strings.TrimSpace(string(endpoint.Profile))))
	values.Set("protocol", strings.ToLower(strings.TrimSpace(endpoint.Protocol)))
	values.Set("security", strings.ToLower(strings.TrimSpace(endpoint.Security)))
	values.Set("server_name", strings.ToLower(strings.TrimSpace(endpoint.ServerName)))
	values.Set("source", strings.TrimSpace(source.ID))
	values.Set("transport", strings.ToLower(strings.TrimSpace(endpoint.Transport)))
	values.Set("target", net.JoinHostPort(endpoint.Host, strconv.Itoa(endpoint.Port)))
	sum := sha256.Sum256([]byte(values.Encode()))
	return "offer-" + hex.EncodeToString(sum[:12])
}
