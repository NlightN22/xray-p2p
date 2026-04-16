package server

type DesiredArtifacts struct {
	XrayJSON        []byte
	RuntimeMetaJSON []byte
	Extra           map[string][]byte
}

// CompileDesiredXrayJSON compiles Desired inputs into the final xray.json without applying it.
func CompileDesiredXrayJSON(configPath string, extensionsDir string) ([]byte, error) {
	artifacts, err := compileDesired(configPath, extensionsDir)
	if err != nil {
		return nil, err
	}
	return artifacts.XrayJSON, nil
}

// CompileDesiredArtifacts compiles Desired inputs into the runtime artifact set without applying it.
func CompileDesiredArtifacts(configPath string, extensionsDir string) (DesiredArtifacts, error) {
	artifacts, err := compileDesired(configPath, extensionsDir)
	if err != nil {
		return DesiredArtifacts{}, err
	}
	return DesiredArtifacts{
		XrayJSON:        artifacts.XrayJSON,
		RuntimeMetaJSON: artifacts.MetaJSON,
		Extra:           artifacts.Extra,
	}, nil
}
