package servercmd

import (
	"errors"
	"io"
	"os"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/cli/bindingresolve"
	"github.com/NlightN22/xray-p2p/go/internal/cli/tagprompt"
	"github.com/NlightN22/xray-p2p/go/internal/redirect"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

var serverRedirectPromptReader = func() io.Reader {
	return os.Stdin
}

type serverBindingRequest struct {
	InstallDir string
	ConfigDir  string
	CIDR       string
	Domain     string
	Tag        string
	Host       string
	Header     string
	Reader     io.Reader
	Quiet      bool
	Matching   bool
}

func resolveServerBinding(req serverBindingRequest) (redirect.Binding, error) {
	var (
		candidates []redirect.Binding
		err        error
	)
	if req.Matching {
		candidates, err = listServerRedirectBindings(req.InstallDir, req.ConfigDir, req.CIDR, req.Domain)
	} else {
		candidates, err = listServerReverseBindings(req.InstallDir, req.ConfigDir)
	}
	if err != nil {
		return redirect.Binding{}, err
	}
	return bindingresolve.Resolve(candidates, bindingresolve.Options{
		Tag:    req.Tag,
		Host:   req.Host,
		Header: req.Header,
		Reader: req.Reader,
		Quiet:  req.Quiet,
	})
}

func listServerReverseBindings(installDir, configDir string) ([]redirect.Binding, error) {
	records, err := serverReverseListFunc(server.ReverseListOptions{
		InstallDir: installDir,
		ConfigDir:  configDir,
	})
	if err != nil {
		return nil, err
	}

	bindings := make([]redirect.Binding, 0, len(records))
	for _, rec := range records {
		if strings.TrimSpace(rec.Tag) == "" {
			continue
		}
		bindings = append(bindings, redirect.Binding{
			Tag:  rec.Tag,
			Host: rec.Host,
		})
	}
	return bindings, nil
}

func listServerRedirectBindings(installDir, configDir, cidr, domain string) ([]redirect.Binding, error) {
	target, err := redirect.ResolveRule(cidr, domain)
	if err != nil {
		return nil, err
	}
	records, err := serverRedirectListFunc(server.RedirectListOptions{
		InstallDir: installDir,
		ConfigDir:  configDir,
		Pending:    true,
	})
	if err != nil {
		return nil, err
	}
	bindings := make([]redirect.Binding, 0, len(records))
	for _, rec := range records {
		if !serverRedirectRecordMatches(rec, target) {
			continue
		}
		bindings = append(bindings, redirect.Binding{
			Tag:  rec.Tag,
			Host: rec.Hostname,
		})
	}
	return bindings, nil
}

func serverRedirectRecordMatches(rec server.RedirectRecord, target redirect.Target) bool {
	switch target.Kind {
	case redirect.KindDomain:
		return strings.EqualFold(strings.TrimSpace(rec.Domain), target.Value) ||
			strings.EqualFold(strings.TrimSpace(rec.Value), target.Value)
	default:
		return strings.EqualFold(strings.TrimSpace(rec.CIDR), target.Value) ||
			strings.EqualFold(strings.TrimSpace(rec.Value), target.Value)
	}
}

func serverBindingRequiredError(err error) bool {
	return errors.Is(err, bindingresolve.ErrNoCandidates) ||
		errors.Is(err, bindingresolve.ErrAmbiguous) ||
		errors.Is(err, tagprompt.ErrEmpty) ||
		errors.Is(err, tagprompt.ErrAborted) ||
		errors.Is(err, redirect.ErrBindingNotSpecified)
}
