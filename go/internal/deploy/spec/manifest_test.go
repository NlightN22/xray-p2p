package spec

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestMarshalUnmarshalRoundTrip(t *testing.T) {
	manifest := Manifest{
		RemoteHost:     "10.0.10.10",
		XP2PVersion:    "1.2.3",
		GeneratedAt:    time.Date(2025, 11, 4, 7, 47, 42, 0, time.UTC),
		InstallDir:     `C:\xp2p`,
		TrojanUser:     "client@example.invalid",
		TrojanPassword: "secret",
	}

	data, err := Marshal(manifest)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	if !strings.Contains(string(data), `"remote_host": "10.0.10.10"`) {
		t.Fatalf("unexpected marshalled data: %s", string(data))
	}

	decoded, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded != manifest {
		t.Fatalf("round-trip mismatch: %#v != %#v", decoded, manifest)
	}
}

func TestReadWrite(t *testing.T) {
	manifest := Manifest{
		RemoteHost:     "example.internal",
		XP2PVersion:    "0.5.0",
		GeneratedAt:    time.Date(2024, 6, 1, 14, 0, 0, 0, time.UTC),
		InstallDir:     `D:\custom-xp2p`,
		TrojanUser:     "client@example.internal",
		TrojanPassword: "secret",
	}

	var buf bytes.Buffer
	if err := Write(&buf, manifest); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if buf.Len() == 0 || buf.Bytes()[buf.Len()-1] != '\n' {
		t.Fatalf("expected newline terminated output, got %q", buf.String())
	}

	decoded, err := Read(&buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if decoded != manifest {
		t.Fatalf("decoded mismatch: %#v != %#v", decoded, manifest)
	}
}

func TestValidateFailures(t *testing.T) {
	tests := []struct {
		name string
		m    Manifest
		want error
	}{
		{
			name: "missing host",
			m: Manifest{
				XP2PVersion: "0.1.0",
				GeneratedAt: time.Now(),
			},
			want: ErrRemoteHostEmpty,
		},
		{
			name: "missing version",
			m: Manifest{
				RemoteHost:  "example",
				GeneratedAt: time.Now(),
			},
			want: ErrVersionEmpty,
		},
		{
			name: "missing timestamp",
			m: Manifest{
				RemoteHost:  "example",
				XP2PVersion: "0.1.0",
			},
			want: ErrGeneratedZero,
		},
		{
			name: "credential pair missing password",
			m: Manifest{
				RemoteHost:     "example",
				XP2PVersion:    "0.1.0",
				GeneratedAt:    time.Now(),
				TrojanUser:     "client@example",
				TrojanPassword: "",
			},
			want: ErrCredentialPair,
		},
		{
			name: "credential pair missing user",
			m: Manifest{
				RemoteHost:     "example",
				XP2PVersion:    "0.1.0",
				GeneratedAt:    time.Now(),
				TrojanUser:     "",
				TrojanPassword: "secret",
			},
			want: ErrCredentialPair,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Validate(tt.m); !errors.Is(err, tt.want) {
				t.Fatalf("Validate error = %v, want %v", err, tt.want)
			}
			if _, err := Marshal(tt.m); !errors.Is(err, tt.want) {
				t.Fatalf("Marshal error = %v, want %v", err, tt.want)
			}
		})
	}
}
