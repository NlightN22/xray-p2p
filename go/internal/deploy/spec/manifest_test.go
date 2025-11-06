package spec

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestMarshalUnmarshalRoundTrip(t *testing.T) {
	manifest := Manifest{
		Host:           "10.0.10.10",
		Version:        2,
		InstallDir:     `C:\xp2p`,
		TrojanPort:     "8443",
		TrojanUser:     "client@example.invalid",
		TrojanPassword: "secret",
		ExpiresAt:      1730916462,
	}

	data, err := Marshal(manifest)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	if !strings.Contains(string(data), `"host":"10.0.10.10"`) {
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
		Host:           "example.internal",
		Version:        2,
		InstallDir:     `D:\custom-xp2p`,
		TrojanPort:     "62022",
		TrojanUser:     "client@example.internal",
		TrojanPassword: "secret",
		ExpiresAt:      1893458400,
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
				Version: 2,
			},
			want: ErrHostEmpty,
		},
		{
			name: "missing version",
			m: Manifest{
				Host: "example",
			},
			want: ErrVersionInvalid,
		},
		{
			name: "credential pair missing password",
			m: Manifest{
				Host:           "example",
				Version:        2,
				TrojanUser:     "client@example",
				TrojanPassword: "",
			},
			want: ErrCredentialPair,
		},
		{
			name: "credential pair missing user",
			m: Manifest{
				Host:           "example",
				Version:        2,
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
