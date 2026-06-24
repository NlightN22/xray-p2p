package common

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

func LogLevelFromFlags(cmd *cobra.Command) (string, bool, error) {
	if cmd == nil {
		return "", false, nil
	}
	flags := cmd.Flags()
	if flags != nil && flags.Changed("log-level") {
		value, err := flags.GetString("log-level")
		if err != nil {
			return "", false, err
		}
		return value, true, nil
	}
	flags = cmd.InheritedFlags()
	if flags == nil || !flags.Changed("log-level") {
		return "", false, nil
	}
	value, err := flags.GetString("log-level")
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

func ApplyProcessLogLevel(value string) error {
	normalized, err := logging.NormalizeLevel(value)
	if err != nil {
		return err
	}
	if err := os.Setenv(logging.EnvLogLevel, normalized); err != nil {
		return fmt.Errorf("set %s: %w", logging.EnvLogLevel, err)
	}
	logging.SetLevel(normalized)
	return nil
}
