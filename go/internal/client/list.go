package client

// ListOptions controls endpoint listing.
type ListOptions struct {
	InstallDir string
	ConfigDir  string
}

// EndpointRecord represents a configured client endpoint.
type EndpointRecord struct {
	Hostname      string
	Tag           string
	Address       string
	Port          int
	User          string
	ServerName    string
	AllowInsecure bool
}

// ListEndpoints returns all configured endpoints.
func ListEndpoints(opts ListOptions) ([]EndpointRecord, error) {
	paths, err := resolveClientPaths(opts.InstallDir, opts.ConfigDir)
	if err != nil {
		return nil, err
	}

	state, err := loadClientInstallState(paths.stateFile)
	if err != nil {
		return nil, err
	}

	records := make([]EndpointRecord, 0, len(state.Endpoints))
	for _, ep := range state.Endpoints {
		records = append(records, EndpointRecord{
			Hostname:      ep.Hostname,
			Tag:           ep.Tag,
			Address:       ep.Address,
			Port:          ep.Port,
			User:          ep.User,
			ServerName:    ep.ServerName,
			AllowInsecure: ep.AllowInsecure,
		})
	}
	return records, nil
}
