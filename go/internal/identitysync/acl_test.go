package identitysync

import "testing"

func TestACLResolverResolvesExplicitAndProvisionedNestedGroupMembers(t *testing.T) {
	resolver := ACLResolver{Generation: &Generation{
		Subjects: map[string]Subject{
			"alice": {ExternalSubject: "alice", UserLabel: "idp-alice@xp2p.local", Active: true, Provisioned: true},
			"bob":   {ExternalSubject: "bob", UserLabel: "idp-bob@xp2p.local", Active: true},
		},
		Groups: map[string]Group{
			"root": {ID: "root", DirectGroups: []string{"child"}},
			"child": {ID: "child", DirectMembers: []string{
				"alice",
				"bob",
			}},
		},
	}}

	labels, err := resolver.Resolve([]string{"manual@xp2p.local"}, []string{"root"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := []string{"idp-alice@xp2p.local", "manual@xp2p.local"}
	if len(labels) != len(want) {
		t.Fatalf("labels = %#v, want %#v", labels, want)
	}
	for i := range want {
		if labels[i] != want[i] {
			t.Fatalf("labels = %#v, want %#v", labels, want)
		}
	}
}

func TestACLResolverDeletedGroupResolvesOnlyExplicitUsers(t *testing.T) {
	resolver := ACLResolver{Generation: &Generation{}}
	labels, err := resolver.Resolve([]string{"future@xp2p.local"}, []string{"deleted"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(labels) != 1 || labels[0] != "future@xp2p.local" {
		t.Fatalf("labels = %#v", labels)
	}
}
