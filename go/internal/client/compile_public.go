package client

// CompileDesiredXrayJSON compiles Desired inputs into the final xray.json without applying it.
func CompileDesiredXrayJSON(configPath string, extensionsDir string) ([]byte, error) {
	artifacts, err := compileDesired(configPath, extensionsDir)
	if err != nil {
		return nil, err
	}
	return artifacts.XrayJSON, nil
}
