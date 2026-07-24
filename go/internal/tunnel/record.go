package tunnel

import (
	"fmt"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/normalize"
	"github.com/NlightN22/xray-p2p/go/internal/version"
)

const legacyFieldsRemovedSince = "0.3.0"

var currentAppVersion = version.Current

// Record is the canonical persisted tunnel record.
type Record struct {
	Endpoint Endpoint `json:"endpoint" toml:"endpoint"`
	User     User     `json:"user" toml:"user"`
}

// LegacyRecord accepts state written before endpoint/profile/credential existed.
// Deprecated fields are normalized into Record and never emitted by new writers.
type LegacyRecord struct {
	Endpoint   *Endpoint `json:"endpoint,omitempty" toml:"endpoint,omitempty"`
	User       *User     `json:"user,omitempty" toml:"user,omitempty"`
	Host       string    `json:"hostname,omitempty" toml:"hostname,omitempty"`
	Port       int       `json:"port,omitempty" toml:"port,omitempty"`
	UserLabel  string    `json:"user_label,omitempty" toml:"user_label,omitempty"`
	Credential string    `json:"credential,omitempty" toml:"credential,omitempty"`
	Password   string    `json:"password,omitempty" toml:"password,omitempty"`
	Disabled   bool      `json:"disabled,omitempty" toml:"disabled,omitempty"`
}

// NormalizeRecord applies the compatibility-normalization pipeline to persisted state.
func NormalizeRecord(raw LegacyRecord) (Record, normalize.Report, error) {
	pipeline := normalize.Pipeline[LegacyRecord, Record]{
		Defaults: func(value *LegacyRecord) {
			if value.Endpoint == nil {
				value.Endpoint = &Endpoint{}
			}
			if value.User == nil {
				value.User = &User{}
			}
		},
		Rules: []normalize.Rule[LegacyRecord]{legacyRecordRule()},
		Validate: func(value LegacyRecord) error {
			if strings.TrimSpace(value.User.Credential) == "" {
				return fmt.Errorf("connection credential is required")
			}
			_, err := Normalize(*value.Endpoint)
			return err
		},
		Build: func(value LegacyRecord) (Record, error) {
			endpoint, err := Normalize(*value.Endpoint)
			if err != nil {
				return Record{}, err
			}
			return Record{Endpoint: endpoint, User: *value.User}, nil
		},
	}
	return pipeline.Normalize(raw)
}

func legacyRecordRule() normalize.Rule[LegacyRecord] {
	rule := normalize.Rule[LegacyRecord]{
		Name:            "tunnel-state.endpoint-profile-credential",
		Description:     "Map legacy Trojan fields to the neutral tunnel record.",
		DeprecatedSince: "0.2.7",
		RemovedSince:    legacyFieldsRemovedSince,
		RemovalNote:     "Use endpoint and user records instead.",
	}
	rule.Apply = func(raw *LegacyRecord, report *normalize.Report) error {
		legacy := raw.Host != "" || raw.Port != 0 || raw.UserLabel != "" || raw.Credential != "" || raw.Password != ""
		if !legacy {
			return nil
		}
		if version.AtLeast(currentAppVersion(), rule.RemovedSince) {
			return fmt.Errorf("tunnel state uses removed legacy fields; use endpoint and user records")
		}
		report.AddAppliedRule(rule.Name)
		if raw.Host != "" {
			report.AddDeprecatedField("hostname")
			raw.Endpoint.Host = raw.Host
		}
		if raw.Port != 0 {
			report.AddDeprecatedField("port")
			raw.Endpoint.Port = raw.Port
		}
		if raw.UserLabel != "" {
			report.AddDeprecatedField("user_label")
			raw.User.UserLabel = raw.UserLabel
		}
		credential := first(raw.Credential, raw.Password)
		if credential != "" {
			report.AddDeprecatedField("password")
			raw.User.Credential = credential
		}
		raw.User.Disabled = raw.User.Disabled || raw.Disabled
		raw.Host, raw.Port, raw.UserLabel, raw.Credential, raw.Password = "", 0, "", "", ""
		return nil
	}
	return rule
}
