//go:build linux || windows

package server

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func openReverseStorePending() (reverseStore, error) {
	pendingPath := pendingConfigPath()
	livePath := filepath.Clean(config.LiveConfigPath(layout.ServerConfigFileName))
	if err := ensurePendingServerConfigFile(pendingPath, livePath); err != nil {
		return reverseStore{}, err
	}
	doc, err := loadServerStateDocWithFallback(pendingPath, livePath)
	if err != nil {
		return reverseStore{}, err
	}
	state, err := decodeServerReverseState(doc)
	if err != nil {
		return reverseStore{}, err
	}
	return reverseStore{
		path:  pendingPath,
		doc:   doc,
		state: state,
	}, nil
}

func openServerRedirectStorePending() (serverRedirectStore, error) {
	pendingPath := pendingConfigPath()
	livePath := filepath.Clean(config.LiveConfigPath(layout.ServerConfigFileName))
	if err := ensurePendingServerConfigFile(pendingPath, livePath); err != nil {
		return serverRedirectStore{}, err
	}
	doc, err := loadServerStateDocWithFallback(pendingPath, livePath)
	if err != nil {
		return serverRedirectStore{}, err
	}
	reverseState, err := decodeServerReverseState(doc)
	if err != nil {
		return serverRedirectStore{}, err
	}
	redirects, err := decodeServerRedirectRules(doc)
	if err != nil {
		return serverRedirectStore{}, err
	}
	return serverRedirectStore{
		path:      pendingPath,
		doc:       doc,
		reverse:   reverseState,
		redirects: redirects,
	}, nil
}

func openServerForwardStorePending() (serverForwardStore, error) {
	pendingPath := pendingConfigPath()
	livePath := filepath.Clean(config.LiveConfigPath(layout.ServerConfigFileName))
	if err := ensurePendingServerConfigFile(pendingPath, livePath); err != nil {
		return serverForwardStore{}, err
	}
	doc, err := loadServerStateDocWithFallback(pendingPath, livePath)
	if err != nil {
		return serverForwardStore{}, err
	}
	forwards, err := decodeServerForwardRules(doc)
	if err != nil {
		return serverForwardStore{}, err
	}
	redirects, err := decodeServerRedirectRules(doc)
	if err != nil {
		return serverForwardStore{}, err
	}
	return serverForwardStore{
		path:      pendingPath,
		doc:       doc,
		forwards:  forwards,
		redirects: redirects,
	}, nil
}

func loadServerStateDocWithFallback(pendingPath, livePath string) (map[string]any, error) {
	if pendingPath != "" {
		if _, err := os.Stat(pendingPath); err == nil {
			return loadServerStateDoc(pendingPath)
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	return loadServerStateDoc(livePath)
}
