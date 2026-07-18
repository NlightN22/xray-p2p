//go:build windows || linux

package server

import (
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/identitysync"
	"github.com/NlightN22/xray-p2p/go/internal/redirect"
)

func TestReconcileAuthoritativeIdentityRemovalsCascadesOwnedResources(t *testing.T) {
	doc := map[string]any{}
	setServerUsers(doc, []trojanClient{
		{Email: "idp-alice@xp2p.local", Password: "secret", ManagedByIdentity: true},
		{Email: "operator@example.com", Password: "secret"},
	})
	doc[serverReverseStateKey] = map[string]serverReverseChannel{
		"idp-alice-example.rev": {UserID: "idp-alice@xp2p.local", Host: "example.com", Tag: "idp-alice-example.rev", Domain: "idp-alice-example.rev"},
		"operator-example.rev":  {UserID: "operator@example.com", Host: "example.com", Tag: "operator-example.rev", Domain: "operator-example.rev"},
	}
	doc[serverRedirectRulesKey] = []redirect.Rule{
		{Domain: "owned.example", OutboundTag: "idp-alice-example.rev"},
		{Domain: "operator.example", OutboundTag: "operator-example.rev", AccessPolicy: redirect.AccessPolicy{Access: "restricted", Users: []string{"idp-alice@xp2p.local", "operator@example.com"}}},
	}
	state := identitysync.State{
		Current: &identitysync.Generation{
			Subjects: map[string]identitysync.Subject{
				"alice": {ExternalSubject: "alice", UserLabel: "idp-alice@xp2p.local", Provisioned: true},
				"bob":   {ExternalSubject: "bob", UserLabel: "idp-bob@xp2p.local"},
			},
		},
		Pending: &identitysync.Generation{
			ID:               "next",
			ProviderSubjects: []string{"bob"},
			Subjects: map[string]identitysync.Subject{
				"bob": {ExternalSubject: "bob", UserLabel: "idp-bob@xp2p.local"},
			},
		},
		Transaction: &identitysync.Transaction{CandidateGenerationID: "next"},
	}

	changed, err := reconcileAuthoritativeIdentityRemovals(doc, state)
	if err != nil {
		t.Fatalf("reconcile removals: %v", err)
	}
	if !changed {
		t.Fatalf("expected reconciliation to change server Desired")
	}
	users, err := decodeServerTrojanUsers(doc)
	if err != nil {
		t.Fatalf("decode users: %v", err)
	}
	if len(users) != 1 || users[0].Email != "operator@example.com" {
		t.Fatalf("unexpected users after reconciliation: %+v", users)
	}
	reverse, err := decodeServerReverseState(doc)
	if err != nil {
		t.Fatalf("decode reverse: %v", err)
	}
	if _, ok := reverse["idp-alice-example.rev"]; ok {
		t.Fatalf("owned reverse channel remains: %+v", reverse)
	}
	redirects, err := decodeServerRedirectRules(doc)
	if err != nil {
		t.Fatalf("decode redirects: %v", err)
	}
	if len(redirects) != 1 || redirects[0].Domain != "operator.example" {
		t.Fatalf("unexpected redirects after reconciliation: %+v", redirects)
	}
	if got := redirects[0].AccessPolicy.Users; len(got) != 2 {
		t.Fatalf("explicit ACL labels were rewritten: %+v", redirects[0].AccessPolicy)
	}
}

func TestReconcileAuthoritativeIdentityRemovalsKeepsOutOfScopeSubjects(t *testing.T) {
	doc := map[string]any{}
	setServerUsers(doc, []trojanClient{
		{Email: "idp-alice@xp2p.local", Password: "secret", ManagedByIdentity: true},
		{Email: "idp-carol@xp2p.local", Password: "secret", ManagedByIdentity: true},
	})
	doc[serverReverseStateKey] = map[string]serverReverseChannel{
		"idp-alice-example.rev": {UserID: "idp-alice@xp2p.local", Host: "example.com", Tag: "idp-alice-example.rev", Domain: "idp-alice-example.rev"},
		"idp-carol-example.rev": {UserID: "idp-carol@xp2p.local", Host: "example.com", Tag: "idp-carol-example.rev", Domain: "idp-carol-example.rev"},
	}
	doc[serverRedirectRulesKey] = []redirect.Rule{
		{Domain: "carol.example", OutboundTag: "idp-carol-example.rev"},
	}
	state := identitysync.State{
		Current: &identitysync.Generation{
			Subjects: map[string]identitysync.Subject{
				"alice": {ExternalSubject: "alice", UserLabel: "idp-alice@xp2p.local", Provisioned: true},
				"carol": {ExternalSubject: "carol", UserLabel: "idp-carol@xp2p.local", Provisioned: true},
			},
		},
		Pending: &identitysync.Generation{
			ID:               "next",
			ProviderSubjects: []string{"alice", "carol"},
			Subjects: map[string]identitysync.Subject{
				"alice": {ExternalSubject: "alice", UserLabel: "idp-alice@xp2p.local", Provisioned: true},
			},
		},
		Transaction: &identitysync.Transaction{CandidateGenerationID: "next"},
	}

	changed, err := reconcileAuthoritativeIdentityRemovals(doc, state)
	if err != nil {
		t.Fatalf("reconcile removals: %v", err)
	}
	if changed {
		t.Fatalf("out-of-scope subject changed server Desired")
	}
}
