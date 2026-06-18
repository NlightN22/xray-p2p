package servercmd

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

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

func generateDefaultServerCredential(ctx context.Context, installOpts server.InstallOptions, host string) error {
	userID, err := generateDefaultUserID()
	if err != nil {
		return err
	}
	password, err := generateRandomSecret(18)
	if err != nil {
		return err
	}

	result, err := provisionCredential(ctx, installOpts, host, userID, password)
	if err != nil {
		return err
	}

	announceCredential("Generated server credential", result)
	return nil
}

func generateRandomSecret(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func generateDefaultUserID() (string, error) {
	token, err := randomToken(5)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("client-%s@xp2p.local", token), nil
}

func randomToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf)), nil
}
