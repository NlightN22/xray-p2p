package clientcmd

import (
	"strconv"

	"github.com/NlightN22/xray-p2p/go/internal/tunnel"
)

type installLink struct {
	ServerAddress    string
	ServerPort       string
	User             string
	Password         string
	ServerName       string
	ServerNameSet    bool
	ALPN             []string
	AllowInsecure    bool
	AllowInsecureSet bool
	PinnedPeerSHA256 string
	VerifyPeerName   string
	Profile          string
	Protocol         string
	Transport        string
	Security         string
	Flow             string
}

func parseInstallLink(raw string) (installLink, error) {
	parsed, err := tunnel.ParseLink(raw)
	if err != nil {
		return installLink{}, err
	}
	endpoint := parsed.Endpoint
	return installLink{
		ServerAddress:    endpoint.Host,
		ServerPort:       strconv.Itoa(endpoint.Port),
		User:             parsed.User.UserLabel,
		Password:         tunnel.ActiveCredential(parsed.User),
		ServerName:       endpoint.ServerName,
		ServerNameSet:    endpoint.ServerName != "",
		ALPN:             endpoint.TLS.ALPN,
		AllowInsecure:    endpoint.TLS.AllowInsecure,
		AllowInsecureSet: endpoint.TLS.AllowInsecure,
		PinnedPeerSHA256: endpoint.TLS.PinnedPeerCertSHA256,
		VerifyPeerName:   endpoint.TLS.VerifyPeerCertByName,
		Profile:          string(endpoint.Profile),
		Protocol:         endpoint.Protocol,
		Transport:        endpoint.Transport,
		Security:         endpoint.Security,
		Flow:             endpoint.Metadata["flow"],
	}, nil
}
