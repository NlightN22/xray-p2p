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

func TestForwardFlagsCollectsLocalFlags(t *testing.T) {
	dummyCfg := func() config.Config { return config.Config{} }
	makeCmd := func(builder func(commandConfig) *cobra.Command) *cobra.Command {
		return builder(dummyCfg)
	}

	cases := []struct {
		name      string
		builder   func(commandConfig) *cobra.Command
		localArgs []string
		passArgs  []string
		wantFlags []string
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
				"--host", "10.0.0.5",
				"--port", "62099",
				"--user", "alice@example.com",
				"--password", "secret",
				"--trojan-port", "8443",
			},
			wantFlags: []string{
				"--port=62099",
				"--password=secret",
				"--host=10.0.0.5",
				"--trojan-port=8443",
				"--user=alice@example.com",
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Helper()
			cmd := makeCmd(tc.builder)
			applyArgs(t, cmd.Flags(), tc.localArgs)
			got := forwardFlags(cmd, tc.passArgs)
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

func TestClientBundleFlags(t *testing.T) {
	cmd := NewCommand(func() config.Config { return config.Config{} })
	exportCmd, _, err := cmd.Find([]string{"export"})
	if err != nil {
		t.Fatalf("find export command: %v", err)
	}
	if flag := exportCmd.Flags().Lookup("config-root"); flag == nil || flag.Shorthand != "C" {
		t.Fatalf("expected export --config-root shorthand -C")
	}
	if flag := exportCmd.Flags().Lookup("output"); flag == nil || flag.Shorthand != "o" {
		t.Fatalf("expected export --output shorthand -o")
	}

	importCmd, _, err := cmd.Find([]string{"import"})
	if err != nil {
		t.Fatalf("find import command: %v", err)
	}
	if flag := importCmd.Flags().Lookup("config-root"); flag == nil || flag.Shorthand != "C" {
		t.Fatalf("expected import --config-root shorthand -C")
	}
	if flag := importCmd.Flags().Lookup("input"); flag == nil || flag.Shorthand != "i" {
		t.Fatalf("expected import --input shorthand -i")
	}
}
