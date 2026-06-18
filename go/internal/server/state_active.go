package server

import "github.com/NlightN22/xray-p2p/go/internal/redirect"

func activeServerUsers(users []trojanClient) []trojanClient {
	active := make([]trojanClient, 0, len(users))
	for _, user := range users {
		if !user.Disabled {
			active = append(active, user)
		}
	}
	return active
}

func activeServerRedirects(redirects []redirect.Rule) []redirect.Rule {
	active := make([]redirect.Rule, 0, len(redirects))
	for _, rule := range redirects {
		if !rule.Disabled {
			active = append(active, rule)
		}
	}
	return active
}

func activeServerReverseRules(reverse serverReverseState) serverReverseState {
	active := make(serverReverseState)
	for tag, channel := range reverse {
		if !channel.Disabled {
			active[tag] = channel
		}
	}
	return active
}
