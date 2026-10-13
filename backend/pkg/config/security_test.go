package config

import (
	"strings"
	"testing"

	"github.com/knadh/koanf/v2"
)

func TestConfigRejectsUnsafeAuthenticationSettings(t *testing.T) {
	for _, tc := range []struct{ name, env, secret, sms string }{
		{"empty signing key", "dev", "", "dev"},
		{"short signing key", "dev", "known-default", "dev"},
		{"development SMS in production", "prod", strings.Repeat("k", 32), "dev"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k := koanf.New(".")
			k.Set("app.env", tc.env)
			k.Set("jwt.secret", tc.secret)
			k.Set("sms.provider", tc.sms)
			if err := unmarshal(k); err == nil {
				t.Fatal("unsafe authentication configuration accepted")
			}
		})
	}
}
