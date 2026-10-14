package config

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"
)

func TestLiveActivityAPNsConfigurationRejectsIncompleteOrInvalidSigning(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	valid := LiveActivityConfiguration{Enabled: true, KeyID: "KEY1234567", TeamID: "TEAM123456", PrivateKey: string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded}))}
	config := validSecureConfig()
	config.LiveActivity = valid
	if err := config.validate(); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*LiveActivityConfiguration){
		"missing key":            func(c *LiveActivityConfiguration) { c.PrivateKey = "" },
		"malformed key":          func(c *LiveActivityConfiguration) { c.PrivateKey = "secret-malformed-key" },
		"missing key identifier": func(c *LiveActivityConfiguration) { c.KeyID = "" },
		"missing team":           func(c *LiveActivityConfiguration) { c.TeamID = "" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := valid
			change(&candidate)
			config.LiveActivity = candidate
			err := config.validate()
			if err == nil {
				t.Fatal("unsafe signing configuration accepted")
			}
			if strings.Contains(err.Error(), "secret-malformed-key") {
				t.Fatal("configuration error disclosed secret")
			}
		})
	}
}
