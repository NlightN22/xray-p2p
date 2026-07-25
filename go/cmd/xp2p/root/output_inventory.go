package root

import (
	"strings"

	"github.com/spf13/cobra"

	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
)

type mutationResult = clioutput.MutationResult

type outputContract struct {
	class         string
	reason        string
	successResult func(*cobra.Command, []string) any
}

func jsonContract(path string) outputContract {
	return outputContract{
		class: clioutput.ClassJSON,
		successResult: func(cmd *cobra.Command, args []string) any {
			return mutationResult{
				Status:    "completed",
				Operation: strings.TrimPrefix(path, "xp2p "),
				Entity:    mutationEntity(cmd, args),
			}
		},
	}
}

func mutationEntity(cmd *cobra.Command, args []string) string {
	if cmd.CommandPath() == "xp2p server ha group update" {
		return "server ha group"
	}
	for _, name := range []string{"id", "domain", "cidr", "target", "listen-port", "tag", "host"} {
		flag := cmd.Flags().Lookup(name)
		if flag != nil && flag.Changed && strings.TrimSpace(flag.Value.String()) != "" {
			return strings.TrimSpace(flag.Value.String())
		}
	}
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		return strings.TrimSpace(args[0])
	}
	parts := strings.Fields(strings.TrimPrefix(cmd.CommandPath(), "xp2p "))
	if len(parts) > 1 {
		return strings.Join(parts[:len(parts)-1], " ")
	}
	return cmd.CommandPath()
}

func payloadContract(_ string) outputContract {
	return outputContract{class: clioutput.ClassJSON}
}

func handlerContract() outputContract {
	return outputContract{class: clioutput.ClassJSON}
}

func exceptionContract(class, reason string) outputContract {
	return outputContract{class: class, reason: reason}
}

var outputContractInventory = map[string]outputContract{
	"xp2p client debug bundle":                 payloadContract("xp2p client debug bundle"),
	"xp2p client deploy":                       payloadContract("xp2p client deploy"),
	"xp2p client disable":                      handlerContract(),
	"xp2p client dns-forward add":              jsonContract("xp2p client dns-forward add"),
	"xp2p client dns-forward list":             payloadContract("xp2p client dns-forward list"),
	"xp2p client dns-forward remove":           jsonContract("xp2p client dns-forward remove"),
	"xp2p client enable":                       handlerContract(),
	"xp2p client export":                       payloadContract("xp2p client export"),
	"xp2p client forward add":                  handlerContract(),
	"xp2p client forward list":                 payloadContract("xp2p client forward list"),
	"xp2p client forward remove":               handlerContract(),
	"xp2p client group list":                   payloadContract("xp2p client group list"),
	"xp2p client import":                       jsonContract("xp2p client import"),
	"xp2p client install":                      jsonContract("xp2p client install"),
	"xp2p client list":                         payloadContract("xp2p client list"),
	"xp2p client mode":                         payloadContract("xp2p client mode"),
	"xp2p client obs":                          payloadContract("xp2p client obs"),
	"xp2p client redirect add":                 handlerContract(),
	"xp2p client redirect disable":             handlerContract(),
	"xp2p client redirect enable":              handlerContract(),
	"xp2p client redirect list":                payloadContract("xp2p client redirect list"),
	"xp2p client redirect remove":              handlerContract(),
	"xp2p client remove":                       jsonContract("xp2p client remove"),
	"xp2p client reverse disable":              handlerContract(),
	"xp2p client reverse enable":               handlerContract(),
	"xp2p client reverse list":                 payloadContract("xp2p client reverse list"),
	"xp2p client service restart":              jsonContract("xp2p client service restart"),
	"xp2p client service start":                jsonContract("xp2p client service start"),
	"xp2p client service status":               payloadContract("xp2p client service status"),
	"xp2p client service stop":                 jsonContract("xp2p client service stop"),
	"xp2p client state":                        payloadContract("xp2p client state"),
	"xp2p client subscription add":             handlerContract(),
	"xp2p client subscription offers":          payloadContract("xp2p client subscription offers"),
	"xp2p client subscription refresh":         handlerContract(),
	"xp2p client subscription remove":          handlerContract(),
	"xp2p client subscription status":          payloadContract("xp2p client subscription status"),
	"xp2p client update":                       handlerContract(),
	"xp2p heartbeat contract":                  payloadContract("xp2p heartbeat contract"),
	"xp2p nat-redirect add":                    jsonContract("xp2p nat-redirect add"),
	"xp2p nat-redirect list":                   payloadContract("xp2p nat-redirect list"),
	"xp2p nat-redirect remove":                 jsonContract("xp2p nat-redirect remove"),
	"xp2p server cert set":                     jsonContract("xp2p server cert set"),
	"xp2p server cert state":                   payloadContract("xp2p server cert state"),
	"xp2p server debug bundle":                 payloadContract("xp2p server debug bundle"),
	"xp2p server dns-forward add":              jsonContract("xp2p server dns-forward add"),
	"xp2p server dns-forward list":             payloadContract("xp2p server dns-forward list"),
	"xp2p server dns-forward remove":           jsonContract("xp2p server dns-forward remove"),
	"xp2p server export":                       payloadContract("xp2p server export"),
	"xp2p server forward add":                  handlerContract(),
	"xp2p server forward list":                 payloadContract("xp2p server forward list"),
	"xp2p server forward remove":               handlerContract(),
	"xp2p server ha channel create":            handlerContract(),
	"xp2p server ha channel disable":           handlerContract(),
	"xp2p server ha channel finalize":          handlerContract(),
	"xp2p server ha channel inspect":           payloadContract("xp2p server ha channel inspect"),
	"xp2p server ha channel list":              payloadContract("xp2p server ha channel list"),
	"xp2p server ha channel rebind":            handlerContract(),
	"xp2p server ha channel rebind-endpoint":   handlerContract(),
	"xp2p server ha group create":              handlerContract(),
	"xp2p server ha group inspect":             payloadContract("xp2p server ha group inspect"),
	"xp2p server ha group remove":              handlerContract(),
	"xp2p server ha group update":              handlerContract(),
	"xp2p server ha member add":                handlerContract(),
	"xp2p server ha member list":               payloadContract("xp2p server ha member list"),
	"xp2p server ha member remove":             handlerContract(),
	"xp2p server ha member reprioritize":       handlerContract(),
	"xp2p server ha peer add":                  handlerContract(),
	"xp2p server ha peer list":                 payloadContract("xp2p server ha peer list"),
	"xp2p server ha peer remove":               handlerContract(),
	"xp2p server ha peer self":                 handlerContract(),
	"xp2p server ha redirect add":              handlerContract(),
	"xp2p server ha redirect list":             payloadContract("xp2p server ha redirect list"),
	"xp2p server ha redirect remove":           handlerContract(),
	"xp2p server ha status":                    payloadContract("xp2p server ha status"),
	"xp2p server ha sync":                      handlerContract(),
	"xp2p server identity detach":              handlerContract(),
	"xp2p server identity provision":           payloadContract("xp2p server identity provision"),
	"xp2p server identity select":              handlerContract(),
	"xp2p server identity status":              payloadContract("xp2p server identity status"),
	"xp2p server identity sync":                handlerContract(),
	"xp2p server import":                       jsonContract("xp2p server import"),
	"xp2p server install":                      payloadContract("xp2p server install"),
	"xp2p server mode":                         payloadContract("xp2p server mode"),
	"xp2p server profile":                      payloadContract("xp2p server profile"),
	"xp2p server redirect access add-group":    handlerContract(),
	"xp2p server redirect access add-user":     handlerContract(),
	"xp2p server redirect access clear":        handlerContract(),
	"xp2p server redirect access remove-group": handlerContract(),
	"xp2p server redirect access remove-user":  handlerContract(),
	"xp2p server redirect access set":          handlerContract(),
	"xp2p server redirect add":                 handlerContract(),
	"xp2p server redirect disable":             handlerContract(),
	"xp2p server redirect enable":              handlerContract(),
	"xp2p server redirect list":                payloadContract("xp2p server redirect list"),
	"xp2p server redirect remove":              handlerContract(),
	"xp2p server remove":                       jsonContract("xp2p server remove"),
	"xp2p server reverse disable":              handlerContract(),
	"xp2p server reverse enable":               handlerContract(),
	"xp2p server reverse list":                 payloadContract("xp2p server reverse list"),
	"xp2p server service restart":              jsonContract("xp2p server service restart"),
	"xp2p server service start":                jsonContract("xp2p server service start"),
	"xp2p server service status":               payloadContract("xp2p server service status"),
	"xp2p server service stop":                 jsonContract("xp2p server service stop"),
	"xp2p server state":                        payloadContract("xp2p server state"),
	"xp2p server user add":                     payloadContract("xp2p server user add"),
	"xp2p server user disable":                 handlerContract(),
	"xp2p server user enable":                  handlerContract(),
	"xp2p server user list":                    payloadContract("xp2p server user list"),
	"xp2p server user remove":                  handlerContract(),
	"xp2p server user rotate":                  payloadContract("xp2p server user rotate"),
	"xp2p server user update":                  handlerContract(),
	"xp2p completion": exceptionContract(clioutput.ClassGenerator,
		"the result is a shell completion script"),
	"xp2p docs command-map": exceptionContract(clioutput.ClassGenerator,
		"the result is a generated Markdown file"),
	"xp2p client render xray": exceptionContract(clioutput.ClassGenerator,
		"the result is an Xray JSON document"),
	"xp2p server render xray": exceptionContract(clioutput.ClassGenerator,
		"the result is an Xray JSON document"),
	"xp2p diag": exceptionContract(clioutput.ClassLifecycle,
		"the command runs a foreground diagnostics service"),
	"xp2p client run": exceptionContract(clioutput.ClassLifecycle,
		"the command runs Xray in the foreground"),
	"xp2p server run": exceptionContract(clioutput.ClassLifecycle,
		"the command runs Xray in the foreground"),
	"xp2p client service run": exceptionContract(clioutput.ClassLifecycle,
		"the command is an internal service entry point"),
	"xp2p server service run": exceptionContract(clioutput.ClassLifecycle,
		"the command is an internal service entry point"),
	"xp2p server deploy": exceptionContract(clioutput.ClassLifecycle,
		"the command runs a deployment listener"),
	"xp2p ping": exceptionContract(clioutput.ClassStreaming,
		"continuous ping output requires a future NDJSON contract"),
}

// platformSpecificOutputContracts are intentionally absent from command trees on
// unsupported hosts but remain explicit members of the public CLI inventory.
var platformSpecificOutputContracts = map[string]bool{
	"xp2p client dns-forward add":    true,
	"xp2p client dns-forward list":   true,
	"xp2p client dns-forward remove": true,
	"xp2p server dns-forward add":    true,
	"xp2p server dns-forward list":   true,
	"xp2p server dns-forward remove": true,
	"xp2p nat-redirect add":          true,
	"xp2p nat-redirect list":         true,
	"xp2p nat-redirect remove":       true,
}
