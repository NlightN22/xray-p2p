//go:build linux || windows

package server

import (
	"errors"
	"os"

	"github.com/NlightN22/xray-p2p/go/internal/config"
)

func openReverseStorePending() (reverseStore, error) {
	_ = config.ConfigRoot
	return openReverseStore("")
}

func openServerRedirectStorePending() (serverRedirectStore, error) {
	path := pendingConfigPath()
	doc, err := loadServerStateDoc(path)
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
		path:      path,
		doc:       doc,
		reverse:   reverseState,
		redirects: redirects,
	}, nil
}

func openServerForwardStorePending() (serverForwardStore, error) {
	path := pendingConfigPath()
	doc, err := loadServerStateDoc(path)
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
		path:      path,
		doc:       doc,
		forwards:  forwards,
		redirects: redirects,
	}, nil
}

func loadServerStateDocWithFallback(pendingPath, livePath string) (map[string]any, error) {
	_ = livePath
	if pendingPath == "" {
		return loadServerStateDoc(pendingConfigPath())
	}
	if _, err := os.Stat(pendingPath); err == nil {
		return loadServerStateDoc(pendingPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return loadServerStateDoc(pendingConfigPath())
}
