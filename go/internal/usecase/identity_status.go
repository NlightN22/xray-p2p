package usecase

import (
	"context"
	"sort"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/identitysync"
)

type IdentityStatus struct {
	store     identitysync.Store
	redirects IdentityRedirectLister
}

type IdentityStatusView struct {
	Status       string
	LastSuccess  string
	Error        string
	ProviderID   string
	ProviderKind string
	Generation   string
	Detached     bool
	Subjects     []IdentitySubjectView
	Groups       []IdentityGroupView
	Redirects    []IdentityRedirectView
}

type IdentitySubjectView struct {
	Label        string
	ExternalID   string
	DirectGroups []string
	Provisioned  bool
	Active       bool
}

type IdentityGroupView struct {
	ID                string
	DirectMembers     []string
	DirectGroups      []string
	TransitiveMembers []string
}

type IdentityRedirectView struct {
	Type        string
	Value       string
	OutboundTag string
	Host        string
	State       string
}

type IdentityRedirectLister interface {
	ListIdentityRedirects(context.Context) ([]IdentityRedirectView, error)
}

func NewIdentityStatus(store identitysync.Store) *IdentityStatus {
	return &IdentityStatus{store: store}
}

func (i *IdentityStatus) WithRedirects(redirects IdentityRedirectLister) *IdentityStatus {
	i.redirects = redirects
	return i
}

func (i *IdentityStatus) View(ctx context.Context) (IdentityStatusView, error) {
	if err := ctx.Err(); err != nil {
		return IdentityStatusView{}, err
	}
	store := i.store
	if store.IsZero() {
		store = identitysync.DefaultStore()
	}
	state, err := store.Load()
	if err != nil {
		return IdentityStatusView{}, err
	}
	view := IdentityStatusView{Status: string(state.Status.State), LastSuccess: state.Status.LastSuccess, Error: state.Status.Error}
	if state.Provider != nil {
		view.ProviderID = state.Provider.InstanceID
		view.ProviderKind = string(state.Provider.Kind)
	}
	if i.redirects != nil {
		redirects, err := i.redirects.ListIdentityRedirects(ctx)
		if err != nil {
			return IdentityStatusView{}, err
		}
		view.Redirects = identityRedirectViews(redirects)
	}
	if state.Current == nil {
		return view, nil
	}
	view.Generation = state.Current.ID
	view.Detached = state.Current.Detached
	view.Subjects = identitySubjectViews(state.Current.Subjects)
	view.Groups = identityGroupViews(state.Current)
	return view, nil
}

func identitySubjectViews(subjects map[string]identitysync.Subject) []IdentitySubjectView {
	out := make([]IdentitySubjectView, 0, len(subjects))
	for _, subject := range subjects {
		out = append(out, IdentitySubjectView{
			Label:        subject.UserLabel,
			ExternalID:   subject.ExternalSubject,
			DirectGroups: append([]string(nil), subject.DirectGroups...),
			Provisioned:  subject.Provisioned,
			Active:       subject.Active,
		})
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Label) < strings.ToLower(out[j].Label) })
	return out
}

func identityGroupViews(generation *identitysync.Generation) []IdentityGroupView {
	out := make([]IdentityGroupView, 0, len(generation.Groups))
	for _, group := range generation.Groups {
		out = append(out, IdentityGroupView{
			ID:                group.ID,
			DirectMembers:     append([]string(nil), group.DirectMembers...),
			DirectGroups:      append([]string(nil), group.DirectGroups...),
			TransitiveMembers: transitiveIdentityGroupMembers(generation, group.ID),
		})
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].ID) < strings.ToLower(out[j].ID) })
	return out
}

func identityRedirectViews(redirects []IdentityRedirectView) []IdentityRedirectView {
	out := append([]IdentityRedirectView(nil), redirects...)
	sort.Slice(out, func(i, j int) bool {
		left := strings.ToLower(out[i].Type + "\x00" + out[i].Value + "\x00" + out[i].OutboundTag)
		right := strings.ToLower(out[j].Type + "\x00" + out[j].Value + "\x00" + out[j].OutboundTag)
		return left < right
	})
	return out
}

func transitiveIdentityGroupMembers(generation *identitysync.Generation, groupID string) []string {
	seenGroups := map[string]bool{}
	members := map[string]struct{}{}
	var walk func(string)
	walk = func(id string) {
		id = strings.TrimSpace(id)
		if id == "" || seenGroups[id] {
			return
		}
		seenGroups[id] = true
		group, ok := generation.Groups[id]
		if !ok {
			return
		}
		for _, subjectID := range group.DirectMembers {
			subjectID = strings.TrimSpace(subjectID)
			if subjectID != "" {
				members[subjectID] = struct{}{}
			}
		}
		for _, childID := range group.DirectGroups {
			walk(childID)
		}
	}
	walk(groupID)
	out := make([]string, 0, len(members))
	for subjectID := range members {
		out = append(out, subjectID)
	}
	sort.Strings(out)
	return out
}
