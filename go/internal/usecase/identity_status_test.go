package usecase

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/identitysync"
)

func TestIdentityStatusViewIncludesTransitiveMembers(t *testing.T) {
	store := identitysync.StoreAt(filepath.Join(t.TempDir(), "state.json"))
	if err := store.Save(identitysync.State{
		Provider: &identitysync.ProviderRef{InstanceID: "corp", Kind: identitysync.ProviderLDAP},
		Current: &identitysync.Generation{
			ID:                 "gen-1",
			ProviderInstanceID: "corp",
			Subjects: map[string]identitysync.Subject{
				"alice": {ExternalSubject: "alice", UserLabel: "idp-alice@xp2p.local", Active: true, Provisioned: true, DirectGroups: []string{"engineering"}},
				"bob":   {ExternalSubject: "bob", UserLabel: "idp-bob@xp2p.local", Active: true, DirectGroups: []string{"platform"}},
			},
			Groups: map[string]identitysync.Group{
				"engineering": {ID: "engineering", DirectMembers: []string{"alice"}, DirectGroups: []string{"platform"}},
				"platform":    {ID: "platform", DirectMembers: []string{"bob"}},
			},
		},
		Status: identitysync.Status{State: identitysync.SyncStatusSuccess},
	}); err != nil {
		t.Fatalf("save identity state: %v", err)
	}

	view, err := NewIdentityStatus(store).
		WithRedirects(staticIdentityRedirects{items: []IdentityRedirectView{
			{Type: "domain", Value: "z.internal", OutboundTag: "z.rev", State: "enabled"},
			{Type: "domain", Value: "a.internal", OutboundTag: "a.rev", State: "disabled_by_policy"},
		}}).
		View(context.Background())
	if err != nil {
		t.Fatalf("identity status view: %v", err)
	}
	if view.ProviderID != "corp" || view.ProviderKind != "ldap" || view.Generation != "gen-1" {
		t.Fatalf("unexpected status view header: %+v", view)
	}
	if len(view.Subjects) != 2 || view.Subjects[0].Label != "idp-alice@xp2p.local" {
		t.Fatalf("subjects are not sorted or complete: %+v", view.Subjects)
	}
	if len(view.Groups) != 2 || view.Groups[0].ID != "engineering" {
		t.Fatalf("groups are not sorted or complete: %+v", view.Groups)
	}
	got := view.Groups[0].TransitiveMembers
	if len(got) != 2 || got[0] != "alice" || got[1] != "bob" {
		t.Fatalf("transitive members = %+v", got)
	}
	if len(view.Redirects) != 2 || view.Redirects[0].Value != "a.internal" || view.Redirects[0].State != "disabled_by_policy" {
		t.Fatalf("redirect status view is not sorted or complete: %+v", view.Redirects)
	}
}

type staticIdentityRedirects struct {
	items []IdentityRedirectView
}

func (s staticIdentityRedirects) ListIdentityRedirects(context.Context) ([]IdentityRedirectView, error) {
	return s.items, nil
}
