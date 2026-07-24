package root

type outputAudit struct {
	operation     string
	stdoutSources string
	stderrSources string
	runtime       string
	credentials   string
	interaction   string
	consumers     string
}

func mergeOutputAudits(groups ...map[string]outputAudit) map[string]outputAudit {
	result := make(map[string]outputAudit)
	for _, group := range groups {
		for path, audit := range group {
			if _, exists := result[path]; exists {
				panic("duplicate output audit: " + path)
			}
			result[path] = audit
		}
	}
	return result
}

var outputAuditInventory = mergeOutputAudits(
	rootOutputAudits,
	clientOutputAudits,
	serverOutputAudits,
)

var jsonQuietFlagCommands = map[string]bool{
	"xp2p client mode":             true,
	"xp2p client redirect add":     true,
	"xp2p client redirect disable": true,
	"xp2p client redirect enable":  true,
	"xp2p client redirect remove":  true,
	"xp2p client remove":           true,
	"xp2p server cert set":         true,
	"xp2p server redirect add":     true,
	"xp2p server redirect disable": true,
	"xp2p server redirect enable":  true,
	"xp2p server redirect remove":  true,
	"xp2p server remove":           true,
}

var jsonLegacyQuietCommands = map[string]bool{
	"xp2p client install": true,
	"xp2p server install": true,
}

var rootOutputAudits = map[string]outputAudit{
	"xp2p heartbeat contract":  {"show heartbeat schema contract", "heartbeat_cmd.go typed contract renderer", "Cobra validation", "none", "none", "none", "Linux/OpenWrt heartbeat tests"},
	"xp2p nat-redirect add":    {"add or print NAT redirect", "natredirect/command_linux.go plan or mutation renderer", "flag validation; firewall planner and apply diagnostics", "Linux firewall and Desired state", "none", "JSON requires explicit port when selection is ambiguous; apply forces quiet", "Linux NAT redirect host tests"},
	"xp2p nat-redirect list":   {"list NAT redirects", "natredirect/command_linux.go list renderer", "firewall state read diagnostics", "Linux firewall state", "none", "none", "Linux NAT redirect host tests"},
	"xp2p nat-redirect remove": {"remove or print NAT redirect", "natredirect/command_linux.go plan or mutation renderer", "flag validation; firewall planner and apply diagnostics", "Linux firewall and Desired state", "none", "JSON apply forces quiet", "Linux NAT redirect host tests"},
	"xp2p completion":          {"generate shell completion", "Cobra completion generator", "Cobra shell validation", "none", "none", "bounded generator; JSON rejected", "shell users and packaging"},
	"xp2p docs command-map":    {"generate command map", "docs.go Markdown generator", "filesystem and command-tree diagnostics", "repository filesystem", "none", "bounded generator; JSON rejected", "commands_map generation"},
	"xp2p diag":                {"run diagnostics listener", "diag.go foreground event stream", "listener and runtime diagnostics", "network listener and diagnostics runtime", "none", "foreground lifecycle; JSON rejected", "xp2pdiag integration"},
	"xp2p ping":                {"stream tunnel probes", "ping_cmd.go human probe stream", "network and Xray probe diagnostics", "network and optional Xray runtime", "none", "continuous stream; JSON rejected", "operators and host connectivity tests"},
}
