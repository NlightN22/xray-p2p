//go:build windows || linux

package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

var (
	errUserIDRequired   = errors.New("xp2p: user identifier is required")
	errPasswordRequired = errors.New("xp2p: password is required")
)

// AddUser ensures that a Trojan client with the provided identifier and password exists.
// When the client is already present with the same password the operation is a no-op.
// If the client exists with a different password it is updated in-place.
func AddUser(ctx context.Context, opts AddUserOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	userID := strings.TrimSpace(opts.UserID)
	if userID == "" {
		return errUserIDRequired
	}
	password := strings.TrimSpace(opts.Password)
	if password == "" {
		return errPasswordRequired
	}

	host := strings.TrimSpace(opts.Host)
	if host == "" {
		return errors.New("xp2p: host is required to derive reverse identifiers")
	}

	resolvedInstallDir, err := resolveInstallDir(opts.InstallDir)
	if err != nil {
		return err
	}

	configDir, err := resolveUserConfigDir(resolvedInstallDir, opts.ConfigDir)
	if err != nil {
		return err
	}

	channel, err := buildServerReverseChannel(userID, host)
	if err != nil {
		return err
	}
	store, err := openReverseStore(resolvedInstallDir)
	if err != nil {
		return err
	}
	if err := store.ensureAvailable(channel); err != nil {
		return err
	}

	configPath := filepath.Join(configDir, "inbounds.json")
	contents, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("xp2p: read %s: %w", configPath, err)
	}

	root, err := parseInbounds(contents)
	if err != nil {
		return err
	}

	trojan, err := selectTrojanInbound(root)
	if err != nil {
		return err
	}

	settings, err := extractSettings(trojan)
	if err != nil {
		return err
	}

	clients, err := extractClients(settings)
	if err != nil {
		return err
	}

	updated := false
	for idx := range clients {
		client := clients[idx]
		if !strings.EqualFold(client.Email, userID) {
			continue
		}
		if client.Password == password {
			logging.Info("xp2p server user add skipped; client already up-to-date",
				"user_id", userID,
				"config", configPath,
			)
			return applyServerReverseChannel(&store, configDir, channel)
		}
		clients[idx].Password = password
		updated = true
		break
	}

	if !updated {
		clients = append(clients, trojanClient{
			Email:    userID,
			Password: password,
		})
		updated = true
	}

	settings["clients"] = clientsToInterfaces(clients)
	if err := writeInbounds(configPath, root); err != nil {
		return err
	}

	logging.Info("xp2p server user added or updated",
		"user_id", userID,
		"config", configPath,
		"updated", updated,
	)
	return applyServerReverseChannel(&store, configDir, channel)
}

// RemoveUser removes the Trojan client with the provided identifier. The operation is idempotent.
func RemoveUser(ctx context.Context, opts RemoveUserOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	userID := strings.TrimSpace(opts.UserID)
	if userID == "" {
		return errUserIDRequired
	}

	host := strings.TrimSpace(opts.Host)
	if host == "" {
		return errors.New("xp2p: host is required to derive reverse identifiers")
	}

	resolvedInstallDir, err := resolveInstallDir(opts.InstallDir)
	if err != nil {
		return err
	}

	configDir, err := resolveUserConfigDir(resolvedInstallDir, opts.ConfigDir)
	if err != nil {
		return err
	}

	channel, err := buildServerReverseChannel(userID, host)
	if err != nil {
		return err
	}
	store, err := openReverseStore(resolvedInstallDir)
	if err != nil {
		return err
	}

	configPath := filepath.Join(configDir, "inbounds.json")
	contents, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("xp2p: %s: %w", configPath, err)
		}
		return fmt.Errorf("xp2p: read %s: %w", configPath, err)
	}

	root, err := parseInbounds(contents)
	if err != nil {
		return err
	}

	trojan, err := selectTrojanInbound(root)
	if err != nil {
		return err
	}

	settings, err := extractSettings(trojan)
	if err != nil {
		return err
	}

	clients, err := extractClients(settings)
	if err != nil {
		return err
	}

	filtered := clients[:0]
	removed := false
	for _, client := range clients {
		if strings.EqualFold(client.Email, userID) {
			removed = true
			continue
		}
		filtered = append(filtered, client)
	}

	if !removed {
		logging.Info("xp2p server user remove skipped; client not found",
			"user_id", userID,
			"config", configPath,
		)
		return purgeServerReverseChannel(&store, configDir, channel)
	}

	settings["clients"] = clientsToInterfaces(filtered)
	if err := writeInbounds(configPath, root); err != nil {
		return err
	}

	logging.Info("xp2p server user removed",
		"user_id", userID,
		"config", configPath,
	)
	return purgeServerReverseChannel(&store, configDir, channel)
}

func resolveUserConfigDir(installDir, configDir string) (string, error) {
	cfg := strings.TrimSpace(configDir)
	if cfg != "" && filepath.IsAbs(cfg) {
		return cfg, nil
	}

	base := strings.TrimSpace(installDir)
	if base == "" {
		return "", errors.New("xp2p: install directory is required when config dir is relative")
	}

	resolvedBase, err := resolveInstallDir(base)
	if err != nil {
		return "", err
	}
	return resolveConfigDir(resolvedBase, cfg)
}

func ListUsers(ctx context.Context, opts ListUsersOptions) ([]UserLink, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	configDir, err := resolveUserConfigDir(opts.InstallDir, opts.ConfigDir)
	if err != nil {
		return nil, err
	}

	return listUsersFromConfig(configDir, strings.TrimSpace(opts.Host))
}

func GetUserLink(ctx context.Context, opts UserLinkOptions) (UserLink, error) {
	if err := ctx.Err(); err != nil {
		return UserLink{}, err
	}

	configDir, err := resolveUserConfigDir(opts.InstallDir, opts.ConfigDir)
	if err != nil {
		return UserLink{}, err
	}

	return userLinkFromConfig(configDir, strings.TrimSpace(opts.Host), opts.UserID)
}
