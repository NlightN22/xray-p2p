package clientcmd

import "github.com/NlightN22/xray-p2p/go/internal/link"

type trojanLink = link.TrojanLink

func parseTrojanLink(raw string) (trojanLink, error) {
	return link.ParseTrojanLink(raw)
}
