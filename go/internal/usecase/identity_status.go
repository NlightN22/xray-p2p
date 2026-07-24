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
	Status       string                 `json:"status"`
	LastSuccess  string                 `json:"last_success"`
	Error        string                 `json:"error"`
	ProviderID   string                 `json:"provider_id"`
	ProviderKind string                 `json:"provider_kind"`
	Generation   string                 `json:"generation"`
	Detached     bool                   `json:"detached"`
	Subjects     []IdentitySubjectView  `json:"subjects"`
	Groups       []IdentityGroupView    `json:"groups"`
	Redirects    []IdentityRedirectView `json:"redirects"`
}

type IdentitySubjectView struct {
	Label        string   `json:"label"`
	ExternalID   string   `json:"external_id"`
	DirectGroups []string `json:"direct_groups"`
	Provisioned  bool     `json:"provisioned"`
	Active       bool     `json:"active"`
}

type IdentityGroupView struct {
	ID                string   `json:"id"`
	DirectMembers     []string `json:"direct_members"`
	DirectGroups      []string `json:"direct_groups"`
	TransitiveMembers []string `json:"transitive_members"`
}

type IdentityRedirectView struct {
	Type        string `json:"type"`
	Value       string `json:"value"`
	OutboundTag string `json:"outbound_tag"`
	Host        string `json:"host"`
	State       string `json:"state"`
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
	view := IdentityStatusView{
		Status: string(state.Status.State), LastSuccess: state.Status.LastSuccess, Error: state.Status.Error,
		Subjects: []IdentitySubjectView{}, Groups: []IdentityGroupView{}, Redirects: []IdentityRedirectView{},
	}
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
			DirectGroups: append([]string{}, subject.DirectGroups...),
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
			DirectMembers:     append([]string{}, group.DirectMembers...),
			DirectGroups:      append([]string{}, group.DirectGroups...),
			TransitiveMembers: transitiveIdentityGroupMembers(generation, group.ID),
		})
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].ID) < strings.ToLower(out[j].ID) })
	return out
}

func identityRedirectViews(redirects []IdentityRedirectView) []IdentityRedirectView {
	out := append([]IdentityRedirectView{}, redirects...)
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
