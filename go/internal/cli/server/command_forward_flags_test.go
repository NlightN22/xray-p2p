package servercmd

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/NlightN22/xray-p2p/go/internal/config"
)

func TestForwardFlagsFiltersDiagnostics(t *testing.T) {
	makeCmd := func(builder func(commandConfig) *cobra.Command) *cobra.Command {
		cmd := builder(func() config.Config { return config.Config{} })
		cmd.Flags().String("diag-service-port", "", "")
		cmd.Flags().String("diag-service-mode", "", "")
		cmd.Flags().String("server-install-dir", "", "")
		return cmd
	}

	cases := []struct {
		name           string
		builder        func(commandConfig) *cobra.Command
		persistentArgs []string
		localArgs      []string
		passArgs       []string
		wantFlags      []string
	}{
		{
			name:      "install command",
			builder:   newServerInstallCmd,
			localArgs: []string{"--path", `C:\srv`, "--config-dir", "cfg", "--host", "edge.internal", "--port", "62099", "--force"},
			wantFlags: []string{"--config-dir=cfg", "--force", "--host=edge.internal", "--path=C:\\srv", "--port=62099"},
		},
		{
			name:      "forward add flags",
			builder:   newServerForwardAddCmd,
			localArgs: []string{"--target", "192.0.2.10:22", "--listen", "0.0.0.0", "--listen-port", "60022", "--proto", "tcp"},
			wantFlags: []string{"--listen-port=60022", "--listen=0.0.0.0", "--proto=tcp", "--target=192.0.2.10:22"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			cmd := makeCmd(tc.builder)
			setFlags(t, cmd.Flags(), []string{"--diag-service-port=62023", "--diag-service-mode=manual"})
			setFlags(t, cmd.Flags(), tc.persistentArgs)
			setFlags(t, cmd.Flags(), tc.localArgs)
			got := forwardFlags(cmd, tc.passArgs)
			for _, entry := range got {
				if strings.Contains(entry, "diag-service") {
					t.Fatalf("unexpected diagnostics flag forwarded: %s", entry)
				}
			}
			if len(got) != len(tc.wantFlags)+len(tc.passArgs) {
				t.Fatalf("forwardFlags returned %v, want %v+%v", got, tc.wantFlags, tc.passArgs)
			}
			gotFlags := append([]string(nil), got[:len(tc.wantFlags)]...)
			wantFlags := append([]string(nil), tc.wantFlags...)
			sort.Strings(gotFlags)
			sort.Strings(wantFlags)
			if !reflect.DeepEqual(gotFlags, wantFlags) {
				t.Fatalf("forwardFlags flags = %v, want %v", gotFlags, wantFlags)
			}
			if len(tc.passArgs) > 0 {
				tail := got[len(tc.wantFlags):]
				if !reflect.DeepEqual(tail, tc.passArgs) {
					t.Fatalf("forwardFlags args = %v, want %v", tail, tc.passArgs)
				}
			}
		})
	}
}

func setFlags(t *testing.T, flags *pflag.FlagSet, args []string) {
	t.Helper()
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "--") {
			t.Fatalf("invalid flag %q", arg)
		}
		name := strings.TrimPrefix(arg, "--")
		value := ""
		if eq := strings.IndexRune(name, '='); eq >= 0 {
			value = name[eq+1:]
			name = name[:eq]
		} else if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
			value = args[i+1]
			i++
		} else {
			value = "true"
		}
		if err := flags.Set(name, value); err != nil {
			t.Fatalf("set flag %s: %v", name, err)
		}
	}
}
