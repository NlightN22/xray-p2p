//go:build windows || linux

package identitysync

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	ownedhttp "github.com/NlightN22/xray-p2p/go/internal/nethttp"
)

const maxSCIMResponseBytes = 4 << 20

func (f ConfigFetcher) fetchSCIM(ctx context.Context, provider ProviderRef) (snapshot Snapshot, returnErr error) {
	scim := f.Config.SCIM
	endpoint := strings.TrimRight(strings.TrimSpace(scim.Endpoint), "/")
	if endpoint == "" {
		return Snapshot{}, errors.New("scim endpoint is required")
	}
	timeout := 10 * time.Second
	if parsed, err := time.ParseDuration(strings.TrimSpace(scim.Timeout)); err == nil && parsed > 0 {
		timeout = parsed
	}
	tlsConfig := &tls.Config{InsecureSkipVerify: scim.InsecureTLS} //nolint:gosec
	client := ownedhttp.NewClient(ownedhttp.ClientOptions{Timeout: timeout, TLSConfig: tlsConfig})
	defer func() {
		returnErr = errors.Join(returnErr, shutdownSCIMClient(client))
	}()
	usersBody, err := f.scimGet(ctx, client, endpoint+"/users?briefRepresentation=true")
	if err != nil {
		return Snapshot{}, err
	}
	groupsBody, err := f.scimGet(ctx, client, endpoint+"/groups?briefRepresentation=true")
	if err != nil {
		return Snapshot{}, err
	}
	users, complete, err := parseSCIMUsers(usersBody)
	if err != nil {
		return Snapshot{}, fmt.Errorf("parse scim users: %w", err)
	}
	if !complete {
		return Snapshot{Provider: provider, Complete: false}, nil
	}
	groups, complete, err := parseSCIMGroups(groupsBody)
	if err != nil {
		return Snapshot{}, fmt.Errorf("parse scim groups: %w", err)
	}
	if !complete {
		return Snapshot{Provider: provider, Complete: false}, nil
	}
	snapshot = Snapshot{Provider: provider, Complete: true}
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
		membersBody, err := f.scimGet(ctx, client, endpoint+"/groups/"+group.ID+"/members")
		if err != nil {
			return Snapshot{}, err
		}
		members, complete, err := parseSCIMUsers(membersBody)
		if err != nil {
			return Snapshot{}, fmt.Errorf("parse scim group members: %w", err)
		}
		if !complete {
			return Snapshot{Provider: provider, Complete: false}, nil
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

func (f ConfigFetcher) scimGet(ctx context.Context, client ownedhttp.Doer, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if token := firstIdentityValue(f.Config.SCIM.Token, f.Config.Secret); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer ownedhttp.DrainAndClose(resp, maxSCIMResponseBytes)
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSCIMResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read scim response: %w", err)
	}
	if len(body) > maxSCIMResponseBytes {
		return nil, fmt.Errorf("scim response exceeds %d bytes", maxSCIMResponseBytes)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("scim request failed: %s", resp.Status)
	}
	return body, nil
}

func shutdownSCIMClient(client ownedhttp.OwnedClient) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown SCIM HTTP client: %w", err)
	}
	return nil
}
