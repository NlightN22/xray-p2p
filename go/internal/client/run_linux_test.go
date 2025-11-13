//go:build linux

package client

import "testing"

func TestResolveClientLogPath(t *testing.T) {
	t.Run("AbsolutePath", func(t *testing.T) {
		path := "/tmp/custom.log"
		resolved, err := resolveClientLogPath(path)
		if err != nil {
			t.Fatalf("resolveClientLogPath: %v", err)
		}
		if resolved != path {
			t.Fatalf("expected %s got %s", path, resolved)
		}
	})

	t.Run("RelativePath", func(t *testing.T) {
		resolved, err := resolveClientLogPath("client.err")
		if err != nil {
			t.Fatalf("resolveClientLogPath: %v", err)
		}
		want := "/var/log/xp2p/client.err"
		if resolved != want {
			t.Fatalf("expected %s got %s", want, resolved)
		}
	})

	t.Run("RelativeWithLogsPrefix", func(t *testing.T) {
		resolved, err := resolveClientLogPath("logs/client.err")
		if err != nil {
			t.Fatalf("resolveClientLogPath: %v", err)
		}
		want := "/var/log/xp2p/client.err"
		if resolved != want {
			t.Fatalf("expected %s got %s", want, resolved)
		}
	})

	t.Run("EmptyPath", func(t *testing.T) {
		if _, err := resolveClientLogPath(""); err == nil {
			t.Fatalf("expected error for empty path")
		}
	})
}
