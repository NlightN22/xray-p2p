package clientcmd

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/NlightN22/xray-p2p/go/internal/config"
)

func TestForwardFlagsSkipsPersistentDiagnosticsFlags(t *testing.T) {
	dummyCfg := func() config.Config { return config.Config{} }
	makeCmd := func(builder func(commandConfig) *cobra.Command) *cobra.Command {
		cmd := builder(dummyCfg)
		cmd.Flags().String("diag-service-port", "", "")
		cmd.Flags().String("diag-service-mode", "", "")
		cmd.Flags().String("client-install-dir", "", "")
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
			name:      "install strings and bool",
			builder:   newClientInstallCmd,
			localArgs: []string{"--path", `C:\xp2p`, "--force"},
			passArgs:  []string{"--extra"},
			wantFlags: []string{"--force", "--path=C:\\xp2p"},
		},
		{
			name:      "remove booleans",
			builder:   newClientRemoveCmd,
			localArgs: []string{"--path", `D:\xp2p`, "--config-dir", "cfg-client", "--keep-files", "--ignore-missing"},
			wantFlags: []string{"--config-dir=cfg-client", "--ignore-missing", "--keep-files", "--path=D:\\xp2p"},
		},
		{
			name:      "run bool true/false and strings",
			builder:   newClientRunCmd,
			localArgs: []string{"--quiet", "--auto-install=false", "--xray-log-file", `logs\client.err`},
			passArgs:  []string{"--relay"},
			wantFlags: []string{"--auto-install=false", "--quiet", "--xray-log-file=logs\\client.err"},
		},
		{
			name:    "deploy string flags",
			builder: newClientDeployCmd,
			localArgs: []string{
				"--remote-host", "10.0.0.5",
				"--deploy-port", "62099",
				"--user", "alice@example.com",
				"--password", "secret",
				"--trojan-port", "8443",
			},
			wantFlags: []string{
				"--deploy-port=62099",
				"--password=secret",
				"--remote-host=10.0.0.5",
				"--trojan-port=8443",
				"--user=alice@example.com",
			},
		},
		{
			name:           "persistent overrides forwarded",
			builder:        newClientInstallCmd,
			persistentArgs: []string{"--client-install-dir", `E:\xp2p`},
			localArgs:      []string{"--server-address", "10.0.10.10", "--user", "demo@example.com", "--password", "p@ss"},
			wantFlags: []string{
				"--client-install-dir=E:\\xp2p",
				"--password=p@ss",
				"--server-address=10.0.10.10",
				"--user=demo@example.com",
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Helper()
			cmd := makeCmd(tc.builder)
			applyArgs(t, cmd.Flags(), []string{"--diag-service-port=62023", "--diag-service-mode=manual"})
			applyArgs(t, cmd.Flags(), tc.persistentArgs)
			applyArgs(t, cmd.Flags(), tc.localArgs)
			got := forwardFlags(cmd, tc.passArgs)
			for _, entry := range got {
				if strings.Contains(entry, "diag-service") {
					t.Fatalf("unexpected diagnostics flag forwarded: %s", entry)
				}
			}
			if len(got) != len(tc.wantFlags)+len(tc.passArgs) {
				t.Fatalf("forwardFlags = %v, want flags=%v args=%v", got, tc.wantFlags, tc.passArgs)
			}
			gotFlags := append([]string(nil), got[:len(tc.wantFlags)]...)
			wantFlags := append([]string(nil), tc.wantFlags...)
			sort.Strings(gotFlags)
			sort.Strings(wantFlags)
			if !reflect.DeepEqual(gotFlags, wantFlags) {
				t.Fatalf("forwardFlags flags = %v, want %v", gotFlags, wantFlags)
			}
			tail := got[len(tc.wantFlags):]
			if len(tc.passArgs) == 0 {
				if len(tail) != 0 {
					t.Fatalf("forwardFlags args = %v, want empty", tail)
				}
			} else if !reflect.DeepEqual(tail, tc.passArgs) {
				t.Fatalf("forwardFlags args = %v, want %v", tail, tc.passArgs)
			}
		})
	}
}

func applyArgs(t *testing.T, flags *pflag.FlagSet, args []string) {
	t.Helper()
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "--") {
			t.Fatalf("invalid flag syntax %q", arg)
			return
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
