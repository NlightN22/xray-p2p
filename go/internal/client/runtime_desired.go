package client

func runtimeDesiredToClientInstallState(desired runtimeDesired) clientInstallState {
	state := clientInstallState{
		Redirects: desired.Redirects,
	}
	if len(desired.Endpoints) > 0 {
		state.Endpoints = make([]clientEndpointRecord, 0, len(desired.Endpoints))
		for _, ep := range desired.Endpoints {
			state.Endpoints = append(state.Endpoints, clientEndpointRecord{
				Hostname:             ep.Hostname,
				Address:              ep.Address,
				Tag:                  ep.Tag,
				Port:                 ep.Port,
				User:                 ep.User,
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
