//go:build linux

package server

import "testing"

func TestResolveServerLogPath(t *testing.T) {
	t.Run("Absolute", func(t *testing.T) {
		path := "/tmp/server.log"
		got, err := resolveServerLogPath(path)
		if err != nil {
			t.Fatalf("resolveServerLogPath: %v", err)
		}
		if got != path {
			t.Fatalf("expected %s got %s", path, got)
		}
	})

	t.Run("Relative", func(t *testing.T) {
		got, err := resolveServerLogPath("server.err")
		if err != nil {
			t.Fatalf("resolveServerLogPath: %v", err)
		}
		want := "/var/log/xp2p/server.err"
		if got != want {
			t.Fatalf("expected %s got %s", want, got)
		}
	})

	t.Run("WithLogsPrefix", func(t *testing.T) {
		got, err := resolveServerLogPath("logs/server.err")
		if err != nil {
			t.Fatalf("resolveServerLogPath: %v", err)
		}
		want := "/var/log/xp2p/server.err"
		if got != want {
			t.Fatalf("expected %s got %s", want, got)
		}
	})

	t.Run("Empty", func(t *testing.T) {
		if _, err := resolveServerLogPath(""); err == nil {
			t.Fatalf("expected error for empty path")
		}
	})
}
