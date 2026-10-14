package push

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

var errLiveActivityTransport = errors.New("live_activity_provider_unavailable")

// LiveActivityAPNs owns only provider transport and signing. The application
// worker controls registration eligibility, revocation and retry scheduling.
type LiveActivityAPNs struct {
	client        *http.Client
	clock         ports.Clock
	endpoint      string
	keyID, teamID string
	key           *ecdsa.PrivateKey
	mu            sync.Mutex
	token         string
	issuedAt      time.Time
}

var _ ports.LiveActivityProvider = (*LiveActivityAPNs)(nil)

func NewLiveActivityAPNs(client *http.Client, clock ports.Clock, keyID, teamID string, privateKey []byte, sandbox bool) (*LiveActivityAPNs, error) {
	if client == nil || client.Timeout <= 0 || client.Timeout > 30*time.Second || clock == nil || !validAppleIdentifier(keyID) || !validAppleIdentifier(teamID) {
		return nil, ports.ErrInvalidArgument
	}
	block, rest := pem.Decode(privateKey)
	if block == nil || block.Type != "PRIVATE KEY" || len(bytes.TrimSpace(rest)) != 0 {
		return nil, ports.ErrInvalidArgument
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, ports.ErrInvalidArgument
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok || key.Curve != elliptic.P256() {
		return nil, ports.ErrInvalidArgument
	}
	bounded := *client
	// Never forward a provider bearer credential or activity token on redirects.
	bounded.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	endpoint := "https://api.push.apple.com"
	if sandbox {
		endpoint = "https://api.sandbox.push.apple.com"
	}
	return &LiveActivityAPNs{client: &bounded, clock: clock, endpoint: endpoint, keyID: keyID, teamID: teamID, key: key}, nil
}

func validAppleIdentifier(value string) bool {
	if len(value) != 10 {
		return false
	}
	for _, character := range value {
		if !(character >= 'A' && character <= 'Z' || character >= '0' && character <= '9') {
			return false
		}
	}
	return true
}

func (a *LiveActivityAPNs) providerToken(now time.Time) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.token != "" && !now.Before(a.issuedAt) && now.Sub(a.issuedAt) < 45*time.Minute {
		return a.token, nil
	}
	header, _ := json.Marshal(struct {
		Alg   string `json:"alg"`
		KeyID string `json:"kid"`
	}{"ES256", a.keyID})
	claims, _ := json.Marshal(struct {
		Issuer   string `json:"iss"`
		IssuedAt int64  `json:"iat"`
	}{a.teamID, now.Unix()})
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	digest := sha256.Sum256([]byte(unsigned))
	r, s, err := ecdsa.Sign(rand.Reader, a.key, digest[:])
	if err != nil {
		return "", errLiveActivityTransport
	}
	signature := make([]byte, 64)
	r.FillBytes(signature[:32])
	s.FillBytes(signature[32:])
	a.token = unsigned + "." + base64.RawURLEncoding.EncodeToString(signature)
	a.issuedAt = now
	return a.token, nil
}

func (a *LiveActivityAPNs) Refresh(ctx context.Context, refresh ports.LiveActivityRefresh) (ports.LiveActivityDelivery, error) {
	if a == nil || a.clock == nil || a.client == nil {
		return "", ports.ErrInvalidArgument
	}
	now := a.clock.Now().UTC()
	if !validLiveActivityRefresh(refresh, now) {
		return "", ports.ErrInvalidArgument
	}
	credential, err := a.providerToken(now)
	if err != nil {
		return "", err
	}
	// Mirrors the native ContentState exactly; all Path data is resolved locally.
	props, _ := json.Marshal(struct {
		Owner   string `json:"owner"`
		Lease   string `json:"accountLease"`
		Local   bool   `json:"localPresentation"`
		StaleAt int64  `json:"staleAt"`
	}{refresh.OwnerUserID, refresh.AccountLease, true, refresh.StaleAt.UnixMilli()})
	payload, _ := json.Marshal(map[string]any{"aps": map[string]any{
		"timestamp": now.Unix(), "event": "update", "stale-date": refresh.StaleAt.Unix(),
		"content-state": map[string]string{"name": "HourPathsTimers", "props": string(props)},
	}})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, a.endpoint+"/3/device/"+refresh.DeviceToken, bytes.NewReader(payload))
	if err != nil {
		return "", errLiveActivityTransport
	}
	request.Header.Set("Authorization", "bearer "+credential)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("apns-topic", "com.hourpaths.mobile.push-type.liveactivity")
	request.Header.Set("apns-push-type", "liveactivity")
	request.Header.Set("apns-priority", "5")
	request.Header.Set("apns-expiration", strconv.FormatInt(refresh.ExpiresAt.Unix(), 10))
	response, err := a.client.Do(request)
	if err != nil {
		return ports.LiveActivityRetry, errLiveActivityTransport
	}
	defer response.Body.Close()
	if response.ProtoMajor != 2 {
		return ports.LiveActivityRetry, errLiveActivityTransport
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 4097))
	if err != nil || len(body) > 4096 {
		return ports.LiveActivityRetry, errLiveActivityTransport
	}
	if response.StatusCode == http.StatusOK {
		return ports.LiveActivityAccepted, nil
	}
	if response.StatusCode == http.StatusGone {
		return ports.LiveActivityTokenInvalid, nil
	}
	if response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500 {
		return ports.LiveActivityRetry, nil
	}
	var failure struct {
		Reason string `json:"reason"`
	}
	if json.Unmarshal(body, &failure) == nil && response.StatusCode == http.StatusBadRequest && (failure.Reason == "BadDeviceToken" || failure.Reason == "DeviceTokenNotForTopic") {
		return ports.LiveActivityTokenInvalid, nil
	}
	// Provider bodies and request URLs can contain secrets. Return only a bounded,
	// typed outcome; callers must not log credentials, response bodies or tokens.
	return ports.LiveActivityConfigurationFailure, nil
}

func validLiveActivityRefresh(refresh ports.LiveActivityRefresh, now time.Time) bool {
	if now.IsZero() || refresh.OwnerUserID == "" || len(refresh.OwnerUserID) > 128 || refresh.AccountLease == "" || len(refresh.AccountLease) > 128 || strings.TrimSpace(refresh.OwnerUserID) != refresh.OwnerUserID || strings.TrimSpace(refresh.AccountLease) != refresh.AccountLease || len(refresh.DeviceToken) < 2 || len(refresh.DeviceToken) > 2048 || len(refresh.DeviceToken)%2 != 0 {
		return false
	}
	if _, err := hex.DecodeString(refresh.DeviceToken); err != nil {
		return false
	}
	return refresh.ExpiresAt.After(now) && !refresh.ExpiresAt.After(now.Add(8*time.Hour)) && refresh.StaleAt.After(now) && !refresh.StaleAt.After(refresh.ExpiresAt)
}
