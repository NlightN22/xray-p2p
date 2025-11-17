package servercmd

import (
	"io"
	"os"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/cli/tagprompt"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

var serverRedirectPromptReader = func() io.Reader {
	return os.Stdin
}

func promptServerRedirectBinding(installDir, configDir string) (tagprompt.Entry, error) {
	records, err := serverReverseListFunc(server.ReverseListOptions{
		InstallDir: installDir,
		ConfigDir:  configDir,
	})
	if err != nil {
		return tagprompt.Entry{}, err
	}

	entries := make([]tagprompt.Entry, 0, len(records))
	for _, rec := range records {
		if strings.TrimSpace(rec.Tag) == "" {
			continue
		}
		entries = append(entries, tagprompt.Entry{
			Tag:  rec.Tag,
			Host: rec.Host,
		})
	}

	return tagprompt.Select(entries, tagprompt.Options{
		Header: "Available reverse portals:",
		Reader: serverRedirectPromptReader(),
	})
}
