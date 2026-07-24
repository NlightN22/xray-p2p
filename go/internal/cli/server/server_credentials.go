package servercmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/identity"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

type credentialResult struct {
	details server.UserLink
	linkErr error
}

func provisionCredential(ctx context.Context, installOpts server.InstallOptions, host, userID, password string) (credentialResult, error) {
	user := strings.TrimSpace(userID)
	pass := strings.TrimSpace(password)
	if user == "" || pass == "" {
		return credentialResult{}, errors.New("credential requires user and password")
	}

	addOpts := server.AddUserOptions{
		InstallDir: installOpts.InstallDir,
		ConfigDir:  installOpts.ConfigDir,
		UserID:     user,
		Password:   pass,
		Host:       host,
	}
	if err := serverUserAddFunc(ctx, addOpts); err != nil {
		return credentialResult{}, err
	}

	link, err := serverUserLinkFunc(ctx, server.UserLinkOptions{
		InstallDir: installOpts.InstallDir,
		ConfigDir:  installOpts.ConfigDir,
		Host:       host,
		UserID:     user,
		Pending:    true,
	})
	if err != nil {
		return credentialResult{
			details: server.UserLink{
				UserID:   user,
				Password: pass,
			},
			linkErr: err,
		}, nil
	}

	if strings.TrimSpace(link.UserID) == "" {
		link.UserID = user
	}
	if strings.TrimSpace(link.Password) == "" {
		link.Password = pass
	}
	return credentialResult{details: link}, nil
}

func announceCredential(prefix string, result credentialResult) {
	fmt.Printf("%s:\n  user: %s\n  password: %s\n", prefix, result.details.UserID, result.details.Password)
	if result.linkErr == nil && strings.TrimSpace(result.details.Link) != "" {
		fmt.Printf("  link: %s\n", result.details.Link)
	} else if result.linkErr != nil {
		fmt.Printf("  link: unavailable (%v)\n", result.linkErr)
	}
}

func generateDefaultServerCredential(ctx context.Context, installOpts server.InstallOptions, host string) (credentialResult, error) {
	userID, err := identity.NewUserID()
	if err != nil {
		return credentialResult{}, err
	}
	password, err := identity.NewTunnelCredential()
	if err != nil {
		return credentialResult{}, err
	}

	result, err := provisionCredential(ctx, installOpts, host, userID, password)
	if err != nil {
		return credentialResult{}, err
	}
	return result, nil
}
