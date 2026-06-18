package bindingresolve

import (
	"errors"
	"io"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/cli/tagprompt"
	"github.com/NlightN22/xray-p2p/go/internal/redirect"
)

type Options struct {
	Tag    string
	Host   string
	Header string
	Reader io.Reader
	Quiet  bool
}

var (
	ErrNoCandidates = errors.New("bindingresolve: no candidates")
	ErrAmbiguous    = errors.New("bindingresolve: multiple candidates")
)

func Resolve(candidates []redirect.Binding, opts Options) (redirect.Binding, error) {
	filtered := compact(candidates)
	tag := strings.TrimSpace(opts.Tag)
	host := strings.TrimSpace(opts.Host)
	if tag != "" || host != "" {
		return redirect.ResolveBinding(tag, host, filtered)
	}

	if len(filtered) == 0 {
		return redirect.Binding{}, ErrNoCandidates
	}
	if len(filtered) == 1 {
		return filtered[0], nil
	}
	if opts.Quiet {
		return redirect.Binding{}, ErrAmbiguous
	}

	entries := make([]tagprompt.Entry, 0, len(filtered))
	for _, candidate := range filtered {
		entries = append(entries, tagprompt.Entry{
			Tag:  candidate.Tag,
			Host: candidate.Host,
		})
	}
	selected, err := tagprompt.Select(entries, tagprompt.Options{
		Header: opts.Header,
		Reader: opts.Reader,
	})
	if err != nil {
		return redirect.Binding{}, err
	}
	return redirect.Binding{
		Tag:  selected.Tag,
		Host: selected.Host,
	}, nil
}

func compact(candidates []redirect.Binding) []redirect.Binding {
	result := make([]redirect.Binding, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		tag := strings.TrimSpace(candidate.Tag)
		if tag == "" {
			continue
		}
		host := strings.TrimSpace(candidate.Host)
		key := strings.ToLower(tag) + "|" + strings.ToLower(host)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, redirect.Binding{
			Tag:  tag,
			Host: host,
		})
	}
	return result
}
