package config

import "testing"

func TestPushProviderBoundaryRequiresDedicatedSecretAndSecureEndpoint(t *testing.T) {
	for name, change := range map[string]func(*Config){
		"missing token encryption key": func(config *Config) { config.PushTokenKey = "" },
		"short token encryption key":   func(config *Config) { config.PushTokenKey = "short" },
		"plaintext provider":           func(config *Config) { config.PushProviderEndpoint = "http://push.example" },
		"provider credentials":         func(config *Config) { config.PushProviderEndpoint = "https://user:secret@push.example" },
		"provider query":               func(config *Config) { config.PushProviderEndpoint = "https://push.example?secret=value" },
	} {
		t.Run(name, func(t *testing.T) {
			config := validSecureConfig()
			change(&config)
			if err := config.validate(); err == nil {
				t.Fatal("unsafe push configuration accepted")
			}
		})
	}
}

func TestPushProviderPlaintextRequiresExplicitLocalOverride(t *testing.T) {
	config := validSecureConfig()
	config.PushProviderEndpoint = "http://localhost:9000"
	config.PushProviderInsecure = true
	if err := config.validate(); err != nil {
		t.Fatal(err)
	}
}
