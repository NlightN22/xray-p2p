//go:build linux

package dnsforward

const forwardOwnerDNSForward = "dns-forward"

type state struct {
	Entries map[string]stateEntry `json:"entries,omitempty"`
}

type stateEntry struct {
	Target            string `json:"target"`
	Server            string `json:"server"`
	ForwardListenPort int    `json:"forward_listen_port,omitempty"`
	ForwardTag        string `json:"forward_tag,omitempty"`
	ForwardOwner      string `json:"forward_owner,omitempty"`
	RebindDomain      string `json:"rebind_domain,omitempty"`
}

func (e stateEntry) forwardOwnedByDNSForward() bool {
	return e.ForwardOwner == forwardOwnerDNSForward
}
