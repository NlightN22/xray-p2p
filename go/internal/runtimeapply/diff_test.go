package runtimeapply

import "testing"

func TestClassifyXrayConfigDiffDetectsRoutingOnlyAddRemove(t *testing.T) {
	current := []byte(`{
		"log": {"loglevel": "warning"},
		"routing": {
			"domainStrategy": "AsIs",
			"rules": [
				{"type": "field", "ruleTag": "keep", "outboundTag": "direct"},
				{"type": "field", "ruleTag": "remove", "outboundTag": "proxy-a"}
			]
		}
	}`)
	candidate := []byte(`{
		"log": {"loglevel": "warning"},
		"routing": {
			"domainStrategy": "AsIs",
			"rules": [
				{"type": "field", "ruleTag": "keep", "outboundTag": "direct"},
				{"type": "field", "ruleTag": "add", "outboundTag": "proxy-b"}
			]
		}
	}`)

	diff, err := ClassifyXrayConfigDiff(current, candidate)
	if err != nil {
		t.Fatalf("ClassifyXrayConfigDiff: %v", err)
	}
	if diff.Kind != DiffRoutingOnly {
		t.Fatalf("kind = %s, want %s: %+v", diff.Kind, DiffRoutingOnly, diff)
	}
	if len(diff.AddedRules) != 1 || diff.AddedRules[0].RuleTag != "add" {
		t.Fatalf("unexpected added rules: %+v", diff.AddedRules)
	}
	if len(diff.RemovedRuleTag) != 1 || diff.RemovedRuleTag[0] != "remove" {
		t.Fatalf("unexpected removed rules: %+v", diff.RemovedRuleTag)
	}
}

func TestClassifyXrayConfigDiffRejectsGlobalChange(t *testing.T) {
	current := []byte(`{"log":{"loglevel":"warning"},"routing":{"rules":[{"ruleTag":"a"}]}}`)
	candidate := []byte(`{"log":{"loglevel":"debug"},"routing":{"rules":[{"ruleTag":"a"}]}}`)

	diff, err := ClassifyXrayConfigDiff(current, candidate)
	if err != nil {
		t.Fatalf("ClassifyXrayConfigDiff: %v", err)
	}
	if diff.Kind != DiffUnsupported {
		t.Fatalf("kind = %s, want unsupported", diff.Kind)
	}
}

func TestClassifyXrayConfigDiffRejectsUntaggedRules(t *testing.T) {
	current := []byte(`{"routing":{"rules":[{"outboundTag":"direct"}]}}`)
	candidate := []byte(`{"routing":{"rules":[{"outboundTag":"direct"},{"ruleTag":"add"}]}}`)

	diff, err := ClassifyXrayConfigDiff(current, candidate)
	if err != nil {
		t.Fatalf("ClassifyXrayConfigDiff: %v", err)
	}
	if diff.Kind != DiffUnsupported {
		t.Fatalf("kind = %s, want unsupported", diff.Kind)
	}
}

func TestClassifyXrayConfigDiffRejectsTaggedRuleMutation(t *testing.T) {
	current := []byte(`{"routing":{"rules":[{"ruleTag":"a","outboundTag":"direct"}]}}`)
	candidate := []byte(`{"routing":{"rules":[{"ruleTag":"a","outboundTag":"proxy"}]}}`)

	diff, err := ClassifyXrayConfigDiff(current, candidate)
	if err != nil {
		t.Fatalf("ClassifyXrayConfigDiff: %v", err)
	}
	if diff.Kind != DiffUnsupported {
		t.Fatalf("kind = %s, want unsupported", diff.Kind)
	}
}

func TestClassifyXrayConfigDiffDetectsFullTunnelTargetReplacement(t *testing.T) {
	current := []byte(`{"routing":{"rules":[
		{"type":"field","ruleTag":"xp2p-full-old","ip":["0.0.0.0/0","::/0"],"outboundTag":"proxy-old"}
	]}}`)
	candidate := []byte(`{"routing":{"rules":[
		{"type":"field","ruleTag":"xp2p-full-new","ip":["0.0.0.0/0","::/0"],"outboundTag":"proxy-new"}
	]}}`)

	diff, err := ClassifyXrayConfigDiff(current, candidate)
	if err != nil {
		t.Fatalf("ClassifyXrayConfigDiff: %v", err)
	}
	if diff.Kind != DiffRoutingOnly {
		t.Fatalf("kind = %s, want %s: %+v", diff.Kind, DiffRoutingOnly, diff)
	}
	if len(diff.RemovedRules) != 1 || diff.RemovedRules[0].RuleTag != "xp2p-full-old" {
		t.Fatalf("unexpected removed rules: %+v", diff.RemovedRules)
	}
	if len(diff.AddedRules) != 1 || diff.AddedRules[0].RuleTag != "xp2p-full-new" {
		t.Fatalf("unexpected added rules: %+v", diff.AddedRules)
	}
}

func TestClassifyXrayConfigDiffDetectsInboundOnlyAddRemove(t *testing.T) {
	current := []byte(`{
		"log": {"loglevel": "warning"},
		"inbounds": [
			{"tag": "keep", "protocol": "socks"},
			{"tag": "old", "protocol": "dokodemo-door"}
		],
		"routing": {"rules": [{"ruleTag": "route-a"}]}
	}`)
	candidate := []byte(`{
		"log": {"loglevel": "warning"},
		"inbounds": [
			{"tag": "keep", "protocol": "socks"},
			{"tag": "new", "protocol": "dokodemo-door"}
		],
		"routing": {"rules": [{"ruleTag": "route-a"}]}
	}`)

	diff, err := ClassifyXrayConfigDiff(current, candidate)
	if err != nil {
		t.Fatalf("ClassifyXrayConfigDiff: %v", err)
	}
	if diff.Kind != DiffInboundOnly {
		t.Fatalf("kind = %s, want %s: %+v", diff.Kind, DiffInboundOnly, diff)
	}
	if len(diff.AddedInbounds) != 1 || diff.AddedInbounds[0].Tag != "new" {
		t.Fatalf("unexpected added inbounds: %+v", diff.AddedInbounds)
	}
	if len(diff.RemovedInboundTags) != 1 || diff.RemovedInboundTags[0] != "old" {
		t.Fatalf("unexpected removed inbounds: %+v", diff.RemovedInboundTags)
	}
}

func TestClassifyXrayConfigDiffRejectsInboundGlobalChange(t *testing.T) {
	current := []byte(`{"log":{"loglevel":"warning"},"inbounds":[{"tag":"a"}]}`)
	candidate := []byte(`{"log":{"loglevel":"debug"},"inbounds":[{"tag":"a"},{"tag":"b"}]}`)

	diff, err := ClassifyXrayConfigDiff(current, candidate)
	if err != nil {
		t.Fatalf("ClassifyXrayConfigDiff: %v", err)
	}
	if diff.Kind != DiffUnsupported {
		t.Fatalf("kind = %s, want unsupported", diff.Kind)
	}
}

func TestClassifyXrayConfigDiffDetectsTaggedInboundReplacement(t *testing.T) {
	current := []byte(`{"inbounds":[{"tag":"a","protocol":"socks"}]}`)
	candidate := []byte(`{"inbounds":[{"tag":"a","protocol":"dokodemo-door"}]}`)

	diff, err := ClassifyXrayConfigDiff(current, candidate)
	if err != nil {
		t.Fatalf("ClassifyXrayConfigDiff: %v", err)
	}
	if diff.Kind != DiffInboundOnly {
		t.Fatalf("kind = %s, want %s: %+v", diff.Kind, DiffInboundOnly, diff)
	}
	if len(diff.RemovedInboundTags) != 1 || diff.RemovedInboundTags[0] != "a" {
		t.Fatalf("unexpected removed inbound tags: %+v", diff.RemovedInboundTags)
	}
	if len(diff.AddedInbounds) != 1 || diff.AddedInbounds[0].Tag != "a" {
		t.Fatalf("unexpected added inbounds: %+v", diff.AddedInbounds)
	}
}

func TestClassifyXrayConfigDiffDetectsInboundUserAddRemove(t *testing.T) {
	current := []byte(`{
		"inbounds": [
			{
				"tag": "trojan-in",
				"protocol": "trojan",
				"settings": {"clients": [
					{"email": "keep@example.com", "password": "keep"},
					{"email": "old@example.com", "password": "old"}
				]},
				"streamSettings": {"network": "tcp"}
			}
		],
		"routing": {"rules": [{"ruleTag": "route-a"}]}
	}`)
	candidate := []byte(`{
		"inbounds": [
			{
				"tag": "trojan-in",
				"protocol": "trojan",
				"settings": {"clients": [
					{"email": "keep@example.com", "password": "keep"},
					{"email": "new@example.com", "password": "new"}
				]},
				"streamSettings": {"network": "tcp"}
			}
		],
		"routing": {"rules": [{"ruleTag": "route-a"}]}
	}`)

	diff, err := ClassifyXrayConfigDiff(current, candidate)
	if err != nil {
		t.Fatalf("ClassifyXrayConfigDiff: %v", err)
	}
	if diff.Kind != DiffInboundUsers {
		t.Fatalf("kind = %s, want %s: %+v", diff.Kind, DiffInboundUsers, diff)
	}
	if len(diff.AddedInboundUsers) != 1 || diff.AddedInboundUsers[0].Email != "new@example.com" || diff.AddedInboundUsers[0].Password != "new" {
		t.Fatalf("unexpected added users: %+v", diff.AddedInboundUsers)
	}
	if len(diff.RemovedInboundUsers) != 1 || diff.RemovedInboundUsers[0].Email != "old@example.com" {
		t.Fatalf("unexpected removed users: %+v", diff.RemovedInboundUsers)
	}
}

func TestClassifyXrayConfigDiffDetectsInboundUserPasswordReplacement(t *testing.T) {
	current := []byte(`{"inbounds":[{"tag":"trojan-in","protocol":"trojan","settings":{"clients":[{"email":"a@example.com","password":"old"}]}}]}`)
	candidate := []byte(`{"inbounds":[{"tag":"trojan-in","protocol":"trojan","settings":{"clients":[{"email":"a@example.com","password":"new"}]}}]}`)

	diff, err := ClassifyXrayConfigDiff(current, candidate)
	if err != nil {
		t.Fatalf("ClassifyXrayConfigDiff: %v", err)
	}
	if diff.Kind != DiffInboundUsers {
		t.Fatalf("kind = %s, want %s: %+v", diff.Kind, DiffInboundUsers, diff)
	}
	if len(diff.RemovedInboundUsers) != 1 || diff.RemovedInboundUsers[0].Email != "a@example.com" || diff.RemovedInboundUsers[0].Password != "old" {
		t.Fatalf("unexpected removed users: %+v", diff.RemovedInboundUsers)
	}
	if len(diff.AddedInboundUsers) != 1 || diff.AddedInboundUsers[0].Email != "a@example.com" || diff.AddedInboundUsers[0].Password != "new" {
		t.Fatalf("unexpected added users: %+v", diff.AddedInboundUsers)
	}
}

func TestClassifyXrayConfigDiffDetectsInboundUserMixedRoutingChange(t *testing.T) {
	current := []byte(`{
		"inbounds":[{"tag":"trojan-in","protocol":"trojan","settings":{"clients":[]}}],
		"routing":{"rules":[{"ruleTag":"old","outboundTag":"direct"}]}
	}`)
	candidate := []byte(`{
		"inbounds":[{"tag":"trojan-in","protocol":"trojan","settings":{"clients":[{"email":"a@example.com","password":"new"}]}}],
		"routing":{"rules":[{"ruleTag":"new","outboundTag":"direct"}]}
	}`)

	diff, err := ClassifyXrayConfigDiff(current, candidate)
	if err != nil {
		t.Fatalf("ClassifyXrayConfigDiff: %v", err)
	}
	if diff.Kind != DiffMixed {
		t.Fatalf("kind = %s, want mixed", diff.Kind)
	}
	if len(diff.AddedInboundUsers) != 1 || diff.AddedInboundUsers[0].Email != "a@example.com" {
		t.Fatalf("unexpected added users: %+v", diff.AddedInboundUsers)
	}
	if len(diff.AddedRules) != 1 || diff.AddedRules[0].RuleTag != "new" {
		t.Fatalf("unexpected added rules: %+v", diff.AddedRules)
	}
	if len(diff.RemovedRules) != 1 || diff.RemovedRules[0].RuleTag != "old" {
		t.Fatalf("unexpected removed rules: %+v", diff.RemovedRules)
	}
}

func TestClassifyXrayConfigDiffDetectsServerUserDisableShape(t *testing.T) {
	current := []byte(`{
		"api":{"listen":"127.0.0.1:52180","services":["HandlerService","RoutingService","StatsService","LoggerService"],"tag":"api"},
		"inbounds":[
			{"tag":"trojan-in","protocol":"trojan","settings":{"clients":[
				{"email":"alpha@example.com","password":"alpha-pass"},
				{"email":"bravo@example.com","password":"bravo-pass"}
			]}},
			{"tag":"socks-in","protocol":"socks","settings":{"udp":true}}
		],
		"outbounds":[{"tag":"direct","protocol":"freedom"}],
		"reverse":{"portals":[{"tag":"alpha-rev","domain":"alpha.rev"},{"tag":"bravo-rev","domain":"bravo.rev"}]},
		"routing":{"domainStrategy":"IPOnDemand","rules":[
			{"type":"field","ruleTag":"alpha-route","domain":["full:alpha.rev"],"outboundTag":"alpha-rev","user":["alpha@example.com"]},
			{"type":"field","ruleTag":"bravo-route","domain":["full:bravo.rev"],"outboundTag":"bravo-rev","user":["bravo@example.com"]}
		]},
		"stats":{},
		"policy":{"levels":{"0":{"statsUserUplink":true,"statsUserDownlink":true,"statsUserOnline":true}}}
	}`)
	candidate := []byte(`{
		"api":{"listen":"127.0.0.1:52180","services":["HandlerService","RoutingService","StatsService","LoggerService"],"tag":"api"},
		"inbounds":[
			{"tag":"trojan-in","protocol":"trojan","settings":{"clients":[
				{"email":"bravo@example.com","password":"bravo-pass"}
			]}},
			{"tag":"socks-in","protocol":"socks","settings":{"udp":true}}
		],
		"outbounds":[{"tag":"direct","protocol":"freedom"}],
		"reverse":{"portals":[{"tag":"alpha-rev","domain":"alpha.rev"},{"tag":"bravo-rev","domain":"bravo.rev"}]},
		"routing":{"domainStrategy":"IPOnDemand","rules":[
			{"type":"field","ruleTag":"alpha-route","domain":["full:alpha.rev"],"outboundTag":"alpha-rev","user":["alpha@example.com"]},
			{"type":"field","ruleTag":"bravo-route","domain":["full:bravo.rev"],"outboundTag":"bravo-rev","user":["bravo@example.com"]}
		]},
		"stats":{},
		"policy":{"levels":{"0":{"statsUserUplink":true,"statsUserDownlink":true,"statsUserOnline":true}}}
	}`)

	diff, err := ClassifyXrayConfigDiff(current, candidate)
	if err != nil {
		t.Fatalf("ClassifyXrayConfigDiff: %v", err)
	}
	if diff.Kind != DiffInboundUsers {
		t.Fatalf("kind = %s, want %s: %+v", diff.Kind, DiffInboundUsers, diff)
	}
	if len(diff.RemovedInboundUsers) != 1 || diff.RemovedInboundUsers[0].Email != "alpha@example.com" {
		t.Fatalf("unexpected removed users: %+v", diff.RemovedInboundUsers)
	}
}

func TestClassifyXrayConfigDiffDetectsRoutingOutboundMixedChange(t *testing.T) {
	current := []byte(`{
		"outbounds": [
			{"tag": "direct", "protocol": "freedom"},
			{"tag": "proxy-old", "protocol": "trojan"}
		],
		"routing": {"rules": [
			{"ruleTag": "keep", "outboundTag": "direct"},
			{"ruleTag": "old-route", "outboundTag": "proxy-old"}
		]}
	}`)
	candidate := []byte(`{
		"outbounds": [
			{"tag": "direct", "protocol": "freedom"},
			{"tag": "proxy-new", "protocol": "trojan"}
		],
		"routing": {"rules": [
			{"ruleTag": "keep", "outboundTag": "direct"},
			{"ruleTag": "new-route", "outboundTag": "proxy-new"}
		]}
	}`)

	diff, err := ClassifyXrayConfigDiff(current, candidate)
	if err != nil {
		t.Fatalf("ClassifyXrayConfigDiff: %v", err)
	}
	if diff.Kind != DiffMixed {
		t.Fatalf("kind = %s, want mixed: %+v", diff.Kind, diff)
	}
	if len(diff.RemovedOutbounds) != 1 || diff.RemovedOutbounds[0].Tag != "proxy-old" {
		t.Fatalf("unexpected removed outbounds: %+v", diff.RemovedOutbounds)
	}
	if len(diff.AddedOutbounds) != 1 || diff.AddedOutbounds[0].Tag != "proxy-new" {
		t.Fatalf("unexpected added outbounds: %+v", diff.AddedOutbounds)
	}
	if len(diff.RemovedRules) != 1 || diff.RemovedRules[0].RuleTag != "old-route" {
		t.Fatalf("unexpected removed rules: %+v", diff.RemovedRules)
	}
	if len(diff.AddedRules) != 1 || diff.AddedRules[0].RuleTag != "new-route" {
		t.Fatalf("unexpected added rules: %+v", diff.AddedRules)
	}
}

func TestClassifyXrayConfigDiffDetectsOutboundOnlyAddRemove(t *testing.T) {
	current := []byte(`{
		"log": {"loglevel": "warning"},
		"outbounds": [
			{"tag": "keep", "protocol": "freedom"},
			{"tag": "old", "protocol": "trojan"}
		],
		"routing": {"rules": [{"ruleTag": "route-a"}]}
	}`)
	candidate := []byte(`{
		"log": {"loglevel": "warning"},
		"outbounds": [
			{"tag": "keep", "protocol": "freedom"},
			{"tag": "new", "protocol": "trojan"}
		],
		"routing": {"rules": [{"ruleTag": "route-a"}]}
	}`)

	diff, err := ClassifyXrayConfigDiff(current, candidate)
	if err != nil {
		t.Fatalf("ClassifyXrayConfigDiff: %v", err)
	}
	if diff.Kind != DiffOutboundOnly {
		t.Fatalf("kind = %s, want %s: %+v", diff.Kind, DiffOutboundOnly, diff)
	}
	if len(diff.AddedOutbounds) != 1 || diff.AddedOutbounds[0].Tag != "new" {
		t.Fatalf("unexpected added outbounds: %+v", diff.AddedOutbounds)
	}
	if len(diff.RemovedOutboundTags) != 1 || diff.RemovedOutboundTags[0] != "old" {
		t.Fatalf("unexpected removed outbounds: %+v", diff.RemovedOutboundTags)
	}
}

func TestClassifyXrayConfigDiffDetectsTaggedOutboundReplacement(t *testing.T) {
	current := []byte(`{"outbounds":[{"tag":"a","protocol":"freedom"}]}`)
	candidate := []byte(`{"outbounds":[{"tag":"a","protocol":"trojan"}]}`)

	diff, err := ClassifyXrayConfigDiff(current, candidate)
	if err != nil {
		t.Fatalf("ClassifyXrayConfigDiff: %v", err)
	}
	if diff.Kind != DiffOutboundOnly {
		t.Fatalf("kind = %s, want %s: %+v", diff.Kind, DiffOutboundOnly, diff)
	}
	if len(diff.RemovedOutbounds) != 1 || diff.RemovedOutbounds[0].Tag != "a" {
		t.Fatalf("unexpected removed outbounds: %+v", diff.RemovedOutbounds)
	}
	if len(diff.AddedOutbounds) != 1 || diff.AddedOutbounds[0].Tag != "a" {
		t.Fatalf("unexpected added outbounds: %+v", diff.AddedOutbounds)
	}
}

func TestClassifyXrayConfigDiffNoop(t *testing.T) {
	current := []byte(`{"routing":{"rules":[{"ruleTag":"a","outboundTag":"direct"}]}}`)

	diff, err := ClassifyXrayConfigDiff(current, current)
	if err != nil {
		t.Fatalf("ClassifyXrayConfigDiff: %v", err)
	}
	if diff.Kind != DiffNoop {
		t.Fatalf("kind = %s, want noop", diff.Kind)
	}
}
