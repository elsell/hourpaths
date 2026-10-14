package push

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type activityClock struct{ now time.Time }

func (c *activityClock) Now() time.Time { return c.now }

func TestLiveActivityRefreshUsesSignedHTTP2AndOnlyLocalPresentationIdentity(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	clock := &activityClock{time.Unix(1800000000, 0)}
	token := strings.Repeat("ab", 32)
	var authorizations []string
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor != 2 || r.Method != "POST" || r.URL.Path != "/3/device/"+token {
			t.Errorf("incorrect APNs request protocol/path")
		}
		if r.Header.Get("apns-topic") != "com.hourpaths.mobile.push-type.liveactivity" || r.Header.Get("apns-push-type") != "liveactivity" || r.Header.Get("apns-priority") != "5" {
			t.Error("incorrect Live Activity headers")
		}
		authorization := strings.TrimPrefix(r.Header.Get("Authorization"), "bearer ")
		authorizations = append(authorizations, authorization)
		parts := strings.Split(authorization, ".")
		if len(parts) != 3 {
			t.Error("invalid JWT")
			w.WriteHeader(400)
			return
		}
		signature, err := base64.RawURLEncoding.DecodeString(parts[2])
		if err != nil || len(signature) != 64 {
			t.Error("invalid ES256 signature")
			w.WriteHeader(400)
			return
		}
		digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
		if !ecdsa.Verify(&key.PublicKey, digest[:], new(big.Int).SetBytes(signature[:32]), new(big.Int).SetBytes(signature[32:])) {
			t.Error("provider signature rejected")
		}
		header, _ := base64.RawURLEncoding.DecodeString(parts[0])
		claims, _ := base64.RawURLEncoding.DecodeString(parts[1])
		if !strings.Contains(string(header), `"kid":"KEY1234567"`) || !strings.Contains(string(claims), `"iss":"TEAM123456"`) {
			t.Error("wrong key identity")
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			return
		}
		var decoded struct {
			APS struct {
				Event     string                       `json:"event"`
				Timestamp int64                        `json:"timestamp"`
				Stale     int64                        `json:"stale-date"`
				Content   struct{ Name, Props string } `json:"content-state"`
			} `json:"aps"`
		}
		if err = json.Unmarshal(body, &decoded); err != nil {
			t.Error(err)
		}
		if decoded.APS.Event != "update" || decoded.APS.Timestamp != clock.Now().Unix() || decoded.APS.Content.Name != "HourPathsTimers" {
			t.Error("wrong update envelope")
		}
		var props map[string]any
		if err = json.Unmarshal([]byte(decoded.APS.Content.Props), &props); err != nil {
			t.Error(err)
		}
		if len(props) != 4 || props["owner"] != "alice" || props["accountLease"] != "lease" || props["localPresentation"] != true || strings.Contains(string(body), token) {
			t.Error("refresh contains more than local presentation identity/deadline")
		}
		w.WriteHeader(http.StatusOK)
	}))
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()
	client := server.Client()
	client.Timeout = time.Second
	adapter, err := NewLiveActivityAPNs(client, clock, "KEY1234567", "TEAM123456", pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded}), false)
	if err != nil {
		t.Fatal(err)
	}
	adapter.endpoint = server.URL
	refresh := ports.LiveActivityRefresh{OwnerUserID: "alice", AccountLease: "lease", DeviceToken: token, ExpiresAt: clock.Now().Add(8 * time.Hour), StaleAt: clock.Now().Add(time.Hour)}
	start := clock.now
	for _, offset := range []time.Duration{0, 10 * time.Minute, 20 * time.Minute, 46 * time.Minute} {
		clock.now = start.Add(offset)
		outcome, err := adapter.Refresh(context.Background(), refresh)
		if err != nil || outcome != ports.LiveActivityAccepted {
			t.Fatalf("refresh: %s %v", outcome, err)
		}
	}
	if authorizations[0] != authorizations[1] || authorizations[1] != authorizations[2] {
		t.Error("provider token refreshed too often")
	}
	if authorizations[2] == authorizations[3] {
		t.Error("provider token was not renewed before expiry")
	}
}

func TestLiveActivityProviderFailureClassificationAndCredentialContainment(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	clock := &activityClock{time.Unix(1800000000, 0)}
	token := strings.Repeat("ab", 32)
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   ports.LiveActivityDelivery
	}{
		{"expired token", 410, `{"reason":"Unregistered"}`, ports.LiveActivityTokenInvalid},
		{"bad token", 400, `{"reason":"BadDeviceToken"}`, ports.LiveActivityTokenInvalid},
		{"wrong topic", 400, `{"reason":"DeviceTokenNotForTopic"}`, ports.LiveActivityTokenInvalid},
		{"rate limit", 429, `{"reason":"TooManyRequests"}`, ports.LiveActivityRetry},
		{"unavailable", 503, `{"reason":"Shutdown"}`, ports.LiveActivityRetry},
		{"bad signing key", 403, `{"reason":"InvalidProviderToken"}`, ports.LiveActivityConfigurationFailure},
		{"redirect", 307, `{}`, ports.LiveActivityConfigurationFailure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if calls > 1 {
					t.Error("followed provider redirect with credentials")
					w.WriteHeader(500)
					return
				}
				if tc.status == 307 {
					w.Header().Set("Location", "/credential-leak")
				}
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			server.EnableHTTP2 = true
			server.StartTLS()
			defer server.Close()
			client := server.Client()
			client.Timeout = time.Second
			adapter, err := NewLiveActivityAPNs(client, clock, "KEY1234567", "TEAM123456", pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded}), false)
			if err != nil {
				t.Fatal(err)
			}
			adapter.endpoint = server.URL
			refresh := ports.LiveActivityRefresh{OwnerUserID: "alice", AccountLease: "lease", DeviceToken: token, ExpiresAt: clock.Now().Add(time.Hour), StaleAt: clock.Now().Add(time.Minute)}
			got, err := adapter.Refresh(context.Background(), refresh)
			if err != nil || got != tc.want {
				t.Fatalf("outcome=%s error=%v", got, err)
			}
			bad := refresh
			bad.DeviceToken = "../credential-leak"
			if _, err := adapter.Refresh(context.Background(), bad); err == nil {
				t.Fatal("invalid token accepted")
			}
			bad = refresh
			bad.ExpiresAt = clock.Now()
			if _, err := adapter.Refresh(context.Background(), bad); err == nil {
				t.Fatal("expired registration accepted")
			}
			server.Close()
			_, err = adapter.Refresh(context.Background(), refresh)
			if err == nil || strings.Contains(err.Error(), token) || strings.Contains(err.Error(), server.URL) {
				t.Fatal("network failure missing or disclosed token/URL")
			}
		})
	}
}
