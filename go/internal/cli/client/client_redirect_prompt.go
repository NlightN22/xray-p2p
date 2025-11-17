package clientcmd

import (
	"io"
	"os"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/cli/tagprompt"
	"github.com/NlightN22/xray-p2p/go/internal/client"
)

var clientRedirectPromptReader = func() io.Reader {
	return os.Stdin
}

func promptClientRedirectBinding(installDir, configDir string) (tagprompt.Entry, error) {
	records, err := clientListFunc(client.ListOptions{
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
			Host: rec.Hostname,
		})
	}

	return tagprompt.Select(entries, tagprompt.Options{
		Header: "Available client endpoints:",
		Reader: clientRedirectPromptReader(),
	})
}
