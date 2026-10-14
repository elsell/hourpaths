package config

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
)

type LiveActivityConfiguration struct {
	Enabled, Sandbox          bool
	KeyID, TeamID, PrivateKey string
}

func loadLiveActivityConfiguration() LiveActivityConfiguration {
	return LiveActivityConfiguration{Enabled: boolean("HOURPATHS_LIVE_ACTIVITY_PUSH_ENABLED"), Sandbox: boolean("HOURPATHS_APNS_SANDBOX"), KeyID: os.Getenv("HOURPATHS_APNS_KEY_ID"), TeamID: os.Getenv("HOURPATHS_APNS_TEAM_ID"), PrivateKey: os.Getenv("HOURPATHS_APNS_PRIVATE_KEY")}
}
func (c LiveActivityConfiguration) validate() error {
	if !c.Enabled {
		return nil
	}
	invalid := errors.New("Live Activity APNs requires valid key and team identifiers and a P-256 private key")
	for _, id := range []string{c.KeyID, c.TeamID} {
		if len(id) != 10 {
			return invalid
		}
		for _, character := range id {
			if !(character >= 'A' && character <= 'Z' || character >= '0' && character <= '9') {
				return invalid
			}
		}
	}
	block, rest := pem.Decode([]byte(c.PrivateKey))
	if block == nil || block.Type != "PRIVATE KEY" || len(bytes.TrimSpace(rest)) != 0 {
		return invalid
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return invalid
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok || key.Curve != elliptic.P256() {
		return invalid
	}
	return nil
}
