package push

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestExpoSendUsesOpaqueDataAndKeepsTicketDistinctFromDelivery(t *testing.T) {
	for _, test := range []struct {
		name         string
		presentation ports.PushPresentation
		priority     string
	}{
		{name: "actionable ordinary", presentation: ports.PushActionable, priority: "default"},
		{name: "informational quiet", presentation: ports.PushInformational, priority: "normal"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				if request.Method != http.MethodPost || request.URL.Path != "/--/api/v2/push/send" ||
					request.Header.Get("Content-Type") != "application/json" ||
					request.Header.Get("Accept") != "application/json" {
					t.Fatalf("request = %s %s headers=%v", request.Method, request.URL.Path, request.Header)
				}
				var body map[string]any
				if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				want := map[string]any{
					"to":       "ExponentPushToken[device-secret]",
					"title":    "Invitation",
					"body":     "A Path invitation is ready.",
					"priority": test.priority,
					"data": map[string]any{
						"version":        "1",
						"notificationId": "notification-opaque-1",
					},
				}
				if !equalJSON(body, want) {
					t.Fatalf("payload = %#v, want %#v", body, want)
				}
				for _, forbidden := range []string{"sound", "interruptionLevel"} {
					if _, exists := body[forbidden]; exists {
						t.Fatalf("quiet/ordinary payload included %s", forbidden)
					}
				}
				_, _ = io.WriteString(response, `{"data":{"status":"ok","id":"receipt-1"}}`)
			}))
			defer server.Close()

			adapter, err := NewExpo(&http.Client{Timeout: time.Second}, server.URL)
			if err != nil {
				t.Fatal(err)
			}
			ticket, err := adapter.Send(context.Background(), ports.PushMessage{
				DeviceToken: "ExponentPushToken[device-secret]",
				Title:       "Invitation", Body: "A Path invitation is ready.",
				NotificationID: "notification-opaque-1", Presentation: test.presentation,
			})
			if err != nil {
				t.Fatal(err)
			}
			if ticket.ReceiptID != "receipt-1" || ticket.State != ports.PushAccepted || ticket.Failure != nil {
				t.Fatalf("ticket = %+v, want accepted receipt awaiting delivery", ticket)
			}
		})
	}
}

func TestExpoSendAllowsCanonicalSocialNotificationID(t *testing.T) {
	const notificationID = "comment:0123456789abcdef0123456789abcdef"
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		var body expoMessage
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Data.NotificationID != notificationID {
			t.Fatalf("notification ID=%q", body.Data.NotificationID)
		}
		_, _ = io.WriteString(response, `{"data":{"status":"ok","id":"receipt-1"}}`)
	}))
	defer server.Close()
	adapter, err := NewExpo(&http.Client{Timeout: time.Second}, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	message := validPushMessage()
	message.NotificationID = notificationID
	if _, err := adapter.Send(context.Background(), message); err != nil {
		t.Fatalf("canonical social notification ID rejected: %v", err)
	}
}

func TestNotificationIDValidatorAdmitsOnlyCanonicalSocialColonIDs(t *testing.T) {
	hex := "0123456789abcdef0123456789abcdef"
	nudgeUUID := "14781adc-0a28-4c0a-9499-69110d7a236b"
	for _, value := range []string{"ordinary-opaque", "reaction:" + hex, "comment:" + hex, "comment-heart:" + hex, "nudge:" + nudgeUUID} {
		if !validNotificationID(value) {
			t.Fatalf("canonical notification ID rejected: %q", value)
		}
	}
	for _, value := range []string{
		"comment:", "comment:" + hex[:31], "comment:" + hex + "0",
		"comment:0123456789ABCDEF0123456789ABCDEF", "unknown:" + hex,
		"comment::" + hex, "comment:0123456789abcdef/123456789abcdef",
		"nudge:" + hex, "nudge:14781ADC-0a28-4c0a-9499-69110d7a236b",
		"nudge:14781adc-0a28-4c0a-9499-69110d7a236z",
	} {
		if validNotificationID(value) {
			t.Fatalf("noncanonical notification ID accepted: %q", value)
		}
	}
}

func TestExpoSendClassifiesTicketFailures(t *testing.T) {
	tests := []struct {
		code        string
		disposition ports.PushFailureDisposition
		disable     bool
	}{
		{code: "MessageRateExceeded", disposition: ports.PushRetryable},
		{code: "DeviceNotRegistered", disposition: ports.PushPermanent, disable: true},
		{code: "MessageTooBig", disposition: ports.PushPermanent},
		{code: "InvalidCredentials", disposition: ports.PushPermanent},
	}
	for _, test := range tests {
		t.Run(test.code, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
				_ = json.NewEncoder(response).Encode(map[string]any{"data": map[string]any{
					"status": "error", "message": "provider detail",
					"details": map[string]any{"error": test.code},
				}})
			}))
			defer server.Close()
			adapter, err := NewExpo(&http.Client{Timeout: time.Second}, server.URL)
			if err != nil {
				t.Fatal(err)
			}
			ticket, err := adapter.Send(context.Background(), validPushMessage())
			if err != nil {
				t.Fatal(err)
			}
			if ticket.State != ports.PushFailed || ticket.Failure == nil ||
				ticket.Failure.Code != test.code ||
				ticket.Failure.Disposition != test.disposition ||
				ticket.Failure.DisableDevice != test.disable {
				t.Fatalf("ticket = %+v", ticket)
			}
		})
	}
}

func TestExpoSendClassifiesTransportAndMalformedFailuresWithoutPayloadDisclosure(t *testing.T) {
	for _, test := range []struct {
		name        string
		status      int
		body        string
		disposition ports.PushFailureDisposition
	}{
		{name: "rate limited", status: http.StatusTooManyRequests, body: `{"errors":[{"code":"TOO_MANY_REQUESTS"}]}`, disposition: ports.PushRetryable},
		{name: "server unavailable", status: http.StatusServiceUnavailable, body: `unavailable`, disposition: ports.PushRetryable},
		{name: "bad request", status: http.StatusBadRequest, body: `{"errors":[{"code":"PUSH_TOO_MANY_NOTIFICATIONS"}]}`, disposition: ports.PushPermanent},
		{name: "malformed success", status: http.StatusOK, body: `{"data":{"status":"ok"}}`, disposition: ports.PushPermanent},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
				response.WriteHeader(test.status)
				_, _ = io.WriteString(response, test.body)
			}))
			defer server.Close()
			adapter, err := NewExpo(&http.Client{Timeout: time.Second}, server.URL)
			if err != nil {
				t.Fatal(err)
			}
			_, err = adapter.Send(context.Background(), validPushMessage())
			assertProviderError(t, err, test.disposition)
			for _, secret := range []string{"device-secret", "notification-opaque-1", "private body"} {
				if strings.Contains(err.Error(), secret) {
					t.Fatalf("error disclosed payload data: %v", err)
				}
			}
		})
	}

	adapter, err := NewExpo(&http.Client{
		Timeout: time.Second,
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("network unavailable")
		}),
	}, "https://exp.host")
	if err != nil {
		t.Fatal(err)
	}
	_, err = adapter.Send(context.Background(), validPushMessage())
	assertProviderError(t, err, ports.PushRetryable)
}

func TestExpoReceiptsClassifyDeliveryAndMissingReceiptPending(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/--/api/v2/push/getReceipts" {
			t.Fatalf("request = %s %s", request.Method, request.URL.Path)
		}
		var body struct {
			IDs []string `json:"ids"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		wantIDs := []string{"delivered", "rate", "disabled", "too-big", "missing"}
		if strings.Join(body.IDs, "|") != strings.Join(wantIDs, "|") {
			t.Fatalf("receipt IDs = %v", body.IDs)
		}
		_, _ = io.WriteString(response, `{"data":{
			"delivered":{"status":"ok"},
			"rate":{"status":"error","details":{"error":"MessageRateExceeded"}},
			"disabled":{"status":"error","details":{"error":"DeviceNotRegistered"}},
			"too-big":{"status":"error","details":{"error":"MessageTooBig"}}
		}}`)
	}))
	defer server.Close()
	adapter, err := NewExpo(&http.Client{Timeout: time.Second}, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	receipts, err := adapter.Receipts(context.Background(), []ports.PushReceiptID{
		"delivered", "rate", "disabled", "too-big", "missing",
	})
	if err != nil {
		t.Fatal(err)
	}
	if receipts["delivered"].State != ports.PushDelivered ||
		receipts["missing"].State != ports.PushPending {
		t.Fatalf("receipt states = %+v", receipts)
	}
	if failure := receipts["rate"].Failure; failure == nil || failure.Disposition != ports.PushRetryable {
		t.Fatalf("rate receipt = %+v", receipts["rate"])
	}
	if failure := receipts["disabled"].Failure; failure == nil ||
		failure.Disposition != ports.PushPermanent || !failure.DisableDevice {
		t.Fatalf("disabled receipt = %+v", receipts["disabled"])
	}
	if failure := receipts["too-big"].Failure; failure == nil ||
		failure.Disposition != ports.PushPermanent || failure.DisableDevice {
		t.Fatalf("too-big receipt = %+v", receipts["too-big"])
	}
}

func TestExpoRequiresBoundedClientAndValidProviderInputs(t *testing.T) {
	if _, err := NewExpo(nil, "https://exp.host"); err == nil {
		t.Fatal("nil HTTP client accepted")
	}
	if _, err := NewExpo(&http.Client{}, "https://exp.host"); err == nil {
		t.Fatal("unbounded HTTP client accepted")
	}
	adapter, err := NewExpo(&http.Client{Timeout: time.Second}, "https://exp.host")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Send(context.Background(), ports.PushMessage{}); err == nil {
		t.Fatal("empty push message accepted")
	}
	if _, err := adapter.Receipts(context.Background(), nil); err == nil {
		t.Fatal("empty receipt request accepted")
	}
	if _, err := adapter.Receipts(context.Background(), []ports.PushReceiptID{"receipt:provider"}); err == nil {
		t.Fatal("colon-containing provider receipt accepted")
	}
}

func validPushMessage() ports.PushMessage {
	return ports.PushMessage{
		DeviceToken: "ExponentPushToken[device-secret]",
		Title:       "private title", Body: "private body",
		NotificationID: "notification-opaque-1",
		Presentation:   ports.PushActionable,
	}
}

func assertProviderError(t *testing.T, err error, disposition ports.PushFailureDisposition) {
	t.Helper()
	var providerError *ports.PushProviderError
	if !errors.As(err, &providerError) || providerError.Failure.Disposition != disposition {
		t.Fatalf("error = %v, want %s provider error", err, disposition)
	}
}

func equalJSON(left, right any) bool {
	leftJSON, _ := json.Marshal(left)
	rightJSON, _ := json.Marshal(right)
	return string(leftJSON) == string(rightJSON)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
