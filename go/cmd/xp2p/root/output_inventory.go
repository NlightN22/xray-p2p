package root

import (
	"strings"

	"github.com/spf13/cobra"

	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
)

type mutationResult struct {
	Status    string `json:"status"`
	Operation string `json:"operation"`
	Entity    string `json:"entity"`
}

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

func exceptionContract(class, reason string) outputContract {
	return outputContract{class: class, reason: reason}
}

var outputContractInventory = map[string]outputContract{
	"xp2p client debug bundle":                 payloadContract("xp2p client debug bundle"),
	"xp2p client deploy":                       payloadContract("xp2p client deploy"),
	"xp2p client disable":                      jsonContract("xp2p client disable"),
	"xp2p client dns-forward add":              jsonContract("xp2p client dns-forward add"),
	"xp2p client dns-forward list":             payloadContract("xp2p client dns-forward list"),
	"xp2p client dns-forward remove":           jsonContract("xp2p client dns-forward remove"),
	"xp2p client enable":                       jsonContract("xp2p client enable"),
	"xp2p client export":                       payloadContract("xp2p client export"),
	"xp2p client forward add":                  jsonContract("xp2p client forward add"),
	"xp2p client forward list":                 payloadContract("xp2p client forward list"),
	"xp2p client forward remove":               jsonContract("xp2p client forward remove"),
	"xp2p client group list":                   payloadContract("xp2p client group list"),
	"xp2p client import":                       jsonContract("xp2p client import"),
	"xp2p client install":                      jsonContract("xp2p client install"),
	"xp2p client list":                         payloadContract("xp2p client list"),
	"xp2p client mode":                         payloadContract("xp2p client mode"),
	"xp2p client obs":                          payloadContract("xp2p client obs"),
	"xp2p client redirect add":                 jsonContract("xp2p client redirect add"),
	"xp2p client redirect disable":             jsonContract("xp2p client redirect disable"),
	"xp2p client redirect enable":              jsonContract("xp2p client redirect enable"),
	"xp2p client redirect list":                payloadContract("xp2p client redirect list"),
	"xp2p client redirect remove":              jsonContract("xp2p client redirect remove"),
	"xp2p client remove":                       jsonContract("xp2p client remove"),
	"xp2p client reverse disable":              jsonContract("xp2p client reverse disable"),
	"xp2p client reverse enable":               jsonContract("xp2p client reverse enable"),
	"xp2p client reverse list":                 payloadContract("xp2p client reverse list"),
	"xp2p client service restart":              jsonContract("xp2p client service restart"),
	"xp2p client service start":                jsonContract("xp2p client service start"),
	"xp2p client service status":               payloadContract("xp2p client service status"),
	"xp2p client service stop":                 jsonContract("xp2p client service stop"),
	"xp2p client state":                        payloadContract("xp2p client state"),
	"xp2p client subscription add":             jsonContract("xp2p client subscription add"),
	"xp2p client subscription offers":          payloadContract("xp2p client subscription offers"),
	"xp2p client subscription refresh":         jsonContract("xp2p client subscription refresh"),
	"xp2p client subscription remove":          jsonContract("xp2p client subscription remove"),
	"xp2p client subscription status":          payloadContract("xp2p client subscription status"),
	"xp2p client update":                       jsonContract("xp2p client update"),
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
	"xp2p server forward add":                  jsonContract("xp2p server forward add"),
	"xp2p server forward list":                 payloadContract("xp2p server forward list"),
	"xp2p server forward remove":               jsonContract("xp2p server forward remove"),
	"xp2p server ha channel create":            jsonContract("xp2p server ha channel create"),
	"xp2p server ha channel disable":           jsonContract("xp2p server ha channel disable"),
	"xp2p server ha channel finalize":          jsonContract("xp2p server ha channel finalize"),
	"xp2p server ha channel inspect":           payloadContract("xp2p server ha channel inspect"),
	"xp2p server ha channel list":              payloadContract("xp2p server ha channel list"),
	"xp2p server ha channel rebind":            jsonContract("xp2p server ha channel rebind"),
	"xp2p server ha channel rebind-endpoint":   jsonContract("xp2p server ha channel rebind-endpoint"),
	"xp2p server ha group create":              jsonContract("xp2p server ha group create"),
	"xp2p server ha group inspect":             payloadContract("xp2p server ha group inspect"),
	"xp2p server ha group remove":              jsonContract("xp2p server ha group remove"),
	"xp2p server ha group update":              jsonContract("xp2p server ha group update"),
	"xp2p server ha member add":                jsonContract("xp2p server ha member add"),
	"xp2p server ha member list":               payloadContract("xp2p server ha member list"),
	"xp2p server ha member remove":             jsonContract("xp2p server ha member remove"),
	"xp2p server ha member reprioritize":       jsonContract("xp2p server ha member reprioritize"),
	"xp2p server ha peer add":                  jsonContract("xp2p server ha peer add"),
	"xp2p server ha peer list":                 payloadContract("xp2p server ha peer list"),
	"xp2p server ha peer remove":               jsonContract("xp2p server ha peer remove"),
	"xp2p server ha peer self":                 jsonContract("xp2p server ha peer self"),
	"xp2p server ha redirect add":              jsonContract("xp2p server ha redirect add"),
	"xp2p server ha redirect list":             payloadContract("xp2p server ha redirect list"),
	"xp2p server ha redirect remove":           jsonContract("xp2p server ha redirect remove"),
	"xp2p server ha status":                    payloadContract("xp2p server ha status"),
	"xp2p server ha sync":                      jsonContract("xp2p server ha sync"),
	"xp2p server identity detach":              jsonContract("xp2p server identity detach"),
	"xp2p server identity provision":           payloadContract("xp2p server identity provision"),
	"xp2p server identity select":              jsonContract("xp2p server identity select"),
	"xp2p server identity status":              payloadContract("xp2p server identity status"),
	"xp2p server identity sync":                jsonContract("xp2p server identity sync"),
	"xp2p server import":                       jsonContract("xp2p server import"),
	"xp2p server install":                      payloadContract("xp2p server install"),
	"xp2p server mode":                         payloadContract("xp2p server mode"),
	"xp2p server profile":                      payloadContract("xp2p server profile"),
	"xp2p server redirect access add-group":    jsonContract("xp2p server redirect access add-group"),
	"xp2p server redirect access add-user":     jsonContract("xp2p server redirect access add-user"),
	"xp2p server redirect access clear":        jsonContract("xp2p server redirect access clear"),
	"xp2p server redirect access remove-group": jsonContract("xp2p server redirect access remove-group"),
	"xp2p server redirect access remove-user":  jsonContract("xp2p server redirect access remove-user"),
	"xp2p server redirect access set":          jsonContract("xp2p server redirect access set"),
	"xp2p server redirect add":                 jsonContract("xp2p server redirect add"),
	"xp2p server redirect disable":             jsonContract("xp2p server redirect disable"),
	"xp2p server redirect enable":              jsonContract("xp2p server redirect enable"),
	"xp2p server redirect list":                payloadContract("xp2p server redirect list"),
	"xp2p server redirect remove":              jsonContract("xp2p server redirect remove"),
	"xp2p server remove":                       jsonContract("xp2p server remove"),
	"xp2p server reverse disable":              jsonContract("xp2p server reverse disable"),
	"xp2p server reverse enable":               jsonContract("xp2p server reverse enable"),
	"xp2p server reverse list":                 payloadContract("xp2p server reverse list"),
	"xp2p server service restart":              jsonContract("xp2p server service restart"),
	"xp2p server service start":                jsonContract("xp2p server service start"),
	"xp2p server service status":               payloadContract("xp2p server service status"),
	"xp2p server service stop":                 jsonContract("xp2p server service stop"),
	"xp2p server state":                        payloadContract("xp2p server state"),
	"xp2p server user add":                     payloadContract("xp2p server user add"),
	"xp2p server user disable":                 jsonContract("xp2p server user disable"),
	"xp2p server user enable":                  jsonContract("xp2p server user enable"),
	"xp2p server user list":                    payloadContract("xp2p server user list"),
	"xp2p server user remove":                  jsonContract("xp2p server user remove"),
	"xp2p server user rotate":                  payloadContract("xp2p server user rotate"),
	"xp2p server user update":                  jsonContract("xp2p server user update"),
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
