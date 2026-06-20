package client

func runtimeDesiredToClientInstallState(desired runtimeDesired) clientInstallState {
	state := clientInstallState{
		Redirects: desired.Redirects,
		Reverse:   desired.Reverse,
		Forwards:  desired.Forwards,
	}
	if len(desired.Endpoints) > 0 {
		state.Endpoints = make([]clientEndpointRecord, 0, len(desired.Endpoints))
		for _, ep := range desired.Endpoints {
			state.Endpoints = append(state.Endpoints, clientEndpointRecord{
				Profile:              ep.Profile,
				Protocol:             ep.Protocol,
				Transport:            ep.Transport,
				Security:             ep.Security,
				Flow:                 ep.Flow,
				Hostname:             ep.Hostname,
				Address:              ep.Address,
				Tag:                  ep.Tag,
				Port:                 ep.Port,
				User:                 ep.User,
				Password:             ep.Credential,
				ServerName:           ep.ServerName,
				AllowInsecure:        ep.AllowInsecure,
				PinnedPeerCertSHA256: ep.PinnedPeerCertSHA256,
				VerifyPeerCertByName: ep.VerifyPeerCertByName,
			})
		}
	}
	state.normalize()
	return state
}
