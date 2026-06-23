//go:build windows || linux

package identitysync

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
)

type ConfigFetcher struct {
	Config config.IdentityProviderConfig
}

func (f ConfigFetcher) FetchSnapshot(ctx context.Context, provider ProviderRef) (Snapshot, error) {
	switch provider.Kind {
	case ProviderLDAP:
		return f.fetchLDAP(ctx, provider)
	case ProviderSCIM:
		return f.fetchSCIM(ctx, provider)
	default:
		return Snapshot{}, fmt.Errorf("identity provider kind %q is unsupported", provider.Kind)
	}
}

func (f ConfigFetcher) fetchLDAP(ctx context.Context, provider ProviderRef) (Snapshot, error) {
	ldap := f.Config.LDAP
	if strings.TrimSpace(ldap.URL) == "" || strings.TrimSpace(ldap.BaseDN) == "" {
		return Snapshot{}, errors.New("ldap url and base_dn are required")
	}
	subjectAttr := firstIdentityValue(ldap.SubjectAttribute, "employeeNumber")
	memberAttr := firstIdentityValue(ldap.MembershipAttribute, "memberUid")
	displayAttr := firstIdentityValue(ldap.DisplayNameAttribute, "displayName")
	userFilter := firstIdentityValue(ldap.UserFilter, "(objectClass=inetOrgPerson)")
	groupFilter := firstIdentityValue(ldap.GroupFilter, "(objectClass=posixGroup)")

	users, err := f.ldapSearch(ctx, ldap, userFilter, subjectAttr, displayAttr)
	if err != nil {
		return Snapshot{}, err
	}
	groups, err := f.ldapSearch(ctx, ldap, groupFilter, "cn", memberAttr)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{
		Provider: provider,
		Complete: true,
		Subjects: ldapSubjects(users, subjectAttr, displayAttr),
		Groups:   ldapGroups(groups, memberAttr),
	}, nil
}

func (f ConfigFetcher) ldapSearch(ctx context.Context, ldap config.LDAPProviderConfig, filter string, attrs ...string) ([]ldifEntry, error) {
	args := []string{"-LLL", "-x", "-H", ldap.URL}
	if strings.TrimSpace(ldap.BindDN) != "" {
		args = append(args, "-D", ldap.BindDN, "-w", f.Config.Secret)
	}
	args = append(args, "-b", ldap.BaseDN, filter)
	args = append(args, attrs...)
	cmd := exec.CommandContext(ctx, "ldapsearch", args...)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return nil, fmt.Errorf("ldap search failed: %s", detail)
	}
	return parseLDIF(out.String()), nil
}

func (f ConfigFetcher) fetchSCIM(ctx context.Context, provider ProviderRef) (Snapshot, error) {
	scim := f.Config.SCIM
	endpoint := strings.TrimRight(strings.TrimSpace(scim.Endpoint), "/")
	if endpoint == "" {
		return Snapshot{}, errors.New("scim endpoint is required")
	}
	usersBody, err := f.scimGet(ctx, endpoint+"/users?briefRepresentation=true")
	if err != nil {
		return Snapshot{}, err
	}
	groupsBody, err := f.scimGet(ctx, endpoint+"/groups?briefRepresentation=true")
	if err != nil {
		return Snapshot{}, err
	}
	var users []scimUser
	if err := json.Unmarshal(usersBody, &users); err != nil {
		return Snapshot{}, fmt.Errorf("parse scim users: %w", err)
	}
	var groups []scimGroup
	if err := json.Unmarshal(groupsBody, &groups); err != nil {
		return Snapshot{}, fmt.Errorf("parse scim groups: %w", err)
	}
	snapshot := Snapshot{Provider: provider, Complete: true}
	for _, user := range users {
		id := firstIdentityValue(user.ID, user.UserName)
		if id == "" {
			continue
		}
		snapshot.Subjects = append(snapshot.Subjects, SnapshotSubject{
			ExternalSubject: id,
			DisplayName:     strings.TrimSpace(strings.Join([]string{user.FirstName, user.LastName}, " ")),
		})
	}
	for _, group := range groups {
		if strings.TrimSpace(group.ID) == "" || strings.TrimSpace(group.Name) == "" {
			continue
		}
		membersBody, err := f.scimGet(ctx, endpoint+"/groups/"+group.ID+"/members")
		if err != nil {
			return Snapshot{}, err
		}
		var members []scimUser
		if err := json.Unmarshal(membersBody, &members); err != nil {
			return Snapshot{}, fmt.Errorf("parse scim group members: %w", err)
		}
		item := SnapshotGroup{ID: group.Name, DisplayName: group.Name}
		for _, member := range members {
			id := firstIdentityValue(member.ID, member.UserName)
			if id != "" {
				item.MemberSubjects = append(item.MemberSubjects, id)
			}
		}
		snapshot.Groups = append(snapshot.Groups, item)
	}
	return snapshot, nil
}

func (f ConfigFetcher) scimGet(ctx context.Context, url string) ([]byte, error) {
	timeout := 10 * time.Second
	if parsed, err := time.ParseDuration(strings.TrimSpace(f.Config.SCIM.Timeout)); err == nil && parsed > 0 {
		timeout = parsed
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if token := firstIdentityValue(f.Config.SCIM.Token, f.Config.Secret); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("scim request failed: %s", resp.Status)
	}
	return body, nil
}

type ldifEntry map[string][]string

func parseLDIF(raw string) []ldifEntry {
	var entries []ldifEntry
	current := ldifEntry{}
	lastKey := ""
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			if len(current) > 0 {
				entries = append(entries, current)
				current = ldifEntry{}
				lastKey = ""
			}
			continue
		}
		if strings.HasPrefix(line, " ") && lastKey != "" {
			values := current[lastKey]
			values[len(values)-1] += strings.TrimPrefix(line, " ")
			current[lastKey] = values
			continue
		}
		key, value, ok := strings.Cut(line, ": ")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		current[key] = append(current[key], strings.TrimSpace(value))
		lastKey = key
	}
	if len(current) > 0 {
		entries = append(entries, current)
	}
	return entries
}

func ldapSubjects(entries []ldifEntry, subjectAttr, displayAttr string) []SnapshotSubject {
	var subjects []SnapshotSubject
	for _, entry := range entries {
		id := entryFirst(entry, subjectAttr)
		if id == "" {
			continue
		}
		subjects = append(subjects, SnapshotSubject{
			ExternalSubject: id,
			DisplayName:     entryFirst(entry, displayAttr),
			DN:              entryFirst(entry, "dn"),
		})
	}
	return subjects
}

func ldapGroups(entries []ldifEntry, memberAttr string) []SnapshotGroup {
	var groups []SnapshotGroup
	for _, entry := range entries {
		id := entryFirst(entry, "cn")
		if id == "" {
			continue
		}
		groups = append(groups, SnapshotGroup{
			ID:             id,
			DisplayName:    id,
			DN:             entryFirst(entry, "dn"),
			MemberSubjects: entry[strings.ToLower(memberAttr)],
		})
	}
	return groups
}

func entryFirst(entry ldifEntry, key string) string {
	values := entry[strings.ToLower(key)]
	if len(values) == 0 {
		return ""
	}
	return strings.TrimSpace(values[0])
}

func firstIdentityValue(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

type scimUser struct {
	ID        string `json:"id"`
	UserName  string `json:"userName"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

type scimGroup struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
