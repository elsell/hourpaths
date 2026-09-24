package push

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

const (
	expoSendPath        = "/--/api/v2/push/send"
	expoReceiptsPath    = "/--/api/v2/push/getReceipts"
	maxProviderResponse = 64 << 10
	maxReceiptBatch     = 1000
)

type Expo struct {
	client      *http.Client
	sendURL     string
	receiptsURL string
}

var _ ports.PushProvider = (*Expo)(nil)

func NewExpo(client *http.Client, endpoint string) (*Expo, error) {
	if client == nil || client.Timeout <= 0 {
		return nil, ports.ErrInvalidArgument
	}
	base, err := url.Parse(endpoint)
	if err != nil || base.Scheme == "" || base.Host == "" || base.User != nil ||
		base.RawQuery != "" || base.Fragment != "" ||
		(base.Scheme != "https" && base.Scheme != "http") {
		return nil, ports.ErrInvalidArgument
	}
	boundedClient := *client
	return &Expo{
		client:      &boundedClient,
		sendURL:     providerURL(base, expoSendPath),
		receiptsURL: providerURL(base, expoReceiptsPath),
	}, nil
}

func (e *Expo) Send(ctx context.Context, message ports.PushMessage) (ports.PushTicket, error) {
	if e == nil || e.client == nil || !validMessage(message) {
		return ports.PushTicket{}, ports.ErrInvalidArgument
	}
	priority := "default"
	if message.Presentation == ports.PushInformational {
		priority = "normal"
	}
	payload := expoMessage{
		To: message.DeviceToken, Title: message.Title, Body: message.Body,
		Priority: priority,
		Data:     expoData{Version: "1", NotificationID: message.NotificationID},
	}
	var envelope expoEnvelope
	if err := e.post(ctx, e.sendURL, payload, &envelope); err != nil {
		return ports.PushTicket{}, err
	}
	if len(envelope.Errors) != 0 {
		return ports.PushTicket{}, requestFailure(envelope.Errors[0].Code)
	}
	ticket, ok := singleTicket(envelope.Data)
	if !ok {
		return ports.PushTicket{}, providerFailure("malformed_provider_response", ports.PushPermanent, false)
	}
	switch ticket.Status {
	case "ok":
		if !validOpaque(string(ticket.ID), 256) {
			return ports.PushTicket{}, providerFailure("malformed_provider_response", ports.PushPermanent, false)
		}
		return ports.PushTicket{ReceiptID: ticket.ID, State: ports.PushAccepted}, nil
	case "error":
		failure := classifiedFailure(ticket.Details.Error)
		return ports.PushTicket{State: ports.PushFailed, Failure: &failure}, nil
	default:
		return ports.PushTicket{}, providerFailure("malformed_provider_response", ports.PushPermanent, false)
	}
}

func (e *Expo) Receipts(ctx context.Context, ids []ports.PushReceiptID) (map[ports.PushReceiptID]ports.PushReceipt, error) {
	if e == nil || e.client == nil || len(ids) == 0 || len(ids) > maxReceiptBatch {
		return nil, ports.ErrInvalidArgument
	}
	encodedIDs := make([]string, len(ids))
	seen := make(map[ports.PushReceiptID]struct{}, len(ids))
	for index, id := range ids {
		if !validOpaque(string(id), 256) {
			return nil, ports.ErrInvalidArgument
		}
		if _, duplicate := seen[id]; duplicate {
			return nil, ports.ErrInvalidArgument
		}
		seen[id] = struct{}{}
		encodedIDs[index] = string(id)
	}
	var envelope expoEnvelope
	if err := e.post(ctx, e.receiptsURL, expoReceiptRequest{IDs: encodedIDs}, &envelope); err != nil {
		return nil, err
	}
	if len(envelope.Errors) != 0 {
		return nil, requestFailure(envelope.Errors[0].Code)
	}
	var providerReceipts map[string]expoResult
	if len(envelope.Data) == 0 || json.Unmarshal(envelope.Data, &providerReceipts) != nil || providerReceipts == nil {
		return nil, providerFailure("malformed_provider_response", ports.PushPermanent, false)
	}
	results := make(map[ports.PushReceiptID]ports.PushReceipt, len(ids))
	for _, id := range ids {
		providerReceipt, exists := providerReceipts[string(id)]
		if !exists {
			results[id] = ports.PushReceipt{ReceiptID: id, State: ports.PushPending}
			continue
		}
		switch providerReceipt.Status {
		case "ok":
			results[id] = ports.PushReceipt{ReceiptID: id, State: ports.PushDelivered}
		case "error":
			failure := classifiedFailure(providerReceipt.Details.Error)
			results[id] = ports.PushReceipt{ReceiptID: id, State: ports.PushFailed, Failure: &failure}
		default:
			return nil, providerFailure("malformed_provider_response", ports.PushPermanent, false)
		}
	}
	return results, nil
}

func (e *Expo) post(ctx context.Context, endpoint string, payload, target any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return providerFailure("malformed_request", ports.PushPermanent, false)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return ports.ErrInvalidArgument
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	response, err := e.client.Do(request)
	if err != nil {
		return providerFailure("network", ports.PushRetryable, false)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		disposition := ports.PushPermanent
		if response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= http.StatusInternalServerError {
			disposition = ports.PushRetryable
		}
		return providerFailure("http_"+strconv.Itoa(response.StatusCode), disposition, false)
	}
	limited := io.LimitReader(response.Body, maxProviderResponse+1)
	encoded, err := io.ReadAll(limited)
	if err != nil {
		return providerFailure("network", ports.PushRetryable, false)
	}
	if len(encoded) == 0 || len(encoded) > maxProviderResponse ||
		json.Unmarshal(encoded, target) != nil {
		return providerFailure("malformed_provider_response", ports.PushPermanent, false)
	}
	return nil
}

type expoMessage struct {
	To       string   `json:"to"`
	Title    string   `json:"title"`
	Body     string   `json:"body"`
	Priority string   `json:"priority"`
	Data     expoData `json:"data"`
}

type expoData struct {
	Version        string `json:"version"`
	NotificationID string `json:"notificationId"`
}

type expoReceiptRequest struct {
	IDs []string `json:"ids"`
}

type expoEnvelope struct {
	Data   json.RawMessage    `json:"data"`
	Errors []expoRequestError `json:"errors"`
}

type expoRequestError struct {
	Code string `json:"code"`
}

type expoResult struct {
	Status  string              `json:"status"`
	ID      ports.PushReceiptID `json:"id"`
	Details expoDetails         `json:"details"`
}

type expoDetails struct {
	Error string `json:"error"`
}

func singleTicket(data json.RawMessage) (expoResult, bool) {
	if len(data) == 0 {
		return expoResult{}, false
	}
	var ticket expoResult
	if data[0] == '{' {
		return ticket, json.Unmarshal(data, &ticket) == nil
	}
	var tickets []expoResult
	if json.Unmarshal(data, &tickets) != nil || len(tickets) != 1 {
		return expoResult{}, false
	}
	return tickets[0], true
}

func classifiedFailure(code string) ports.PushFailure {
	switch code {
	case "MessageRateExceeded", "TOO_MANY_REQUESTS":
		return ports.PushFailure{Code: code, Disposition: ports.PushRetryable}
	case "DeviceNotRegistered":
		return ports.PushFailure{Code: code, Disposition: ports.PushPermanent, DisableDevice: true}
	case "MessageTooBig":
		return ports.PushFailure{Code: code, Disposition: ports.PushPermanent}
	case "":
		return ports.PushFailure{Code: "malformed_provider_response", Disposition: ports.PushPermanent}
	default:
		return ports.PushFailure{Code: code, Disposition: ports.PushPermanent}
	}
}

func requestFailure(code string) error {
	failure := classifiedFailure(code)
	return &ports.PushProviderError{Failure: failure}
}

func providerFailure(code string, disposition ports.PushFailureDisposition, disable bool) error {
	return &ports.PushProviderError{Failure: ports.PushFailure{
		Code: code, Disposition: disposition, DisableDevice: disable,
	}}
}

func providerURL(base *url.URL, path string) string {
	target := *base
	target.Path = strings.TrimRight(base.Path, "/") + path
	target.RawPath = ""
	return target.String()
}

func validMessage(message ports.PushMessage) bool {
	return validText(message.DeviceToken, 2048) &&
		validText(message.Title, 256) &&
		validText(message.Body, 4096) &&
		validNotificationID(message.NotificationID) &&
		(message.Presentation == ports.PushActionable ||
			message.Presentation == ports.PushInformational)
}

func validNotificationID(value string) bool {
	if validOpaque(value, 128) {
		return true
	}
	if suffix, found := strings.CutPrefix(value, "nudge:"); found {
		return validCanonicalUUID(suffix)
	}
	for _, prefix := range []string{"reaction:", "comment:", "comment-heart:"} {
		if suffix, found := strings.CutPrefix(value, prefix); found {
			if len(suffix) != 32 {
				return false
			}
			for index := range len(suffix) {
				character := suffix[index]
				if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
					return false
				}
			}
			return true
		}
	}
	return false
}

func validCanonicalUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for index := range len(value) {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			if value[index] != '-' {
				return false
			}
			continue
		}
		character := value[index]
		if !((character >= '0' && character <= '9') ||
			(character >= 'a' && character <= 'f')) {
			return false
		}
	}
	return true
}

func validText(value string, max int) bool {
	return utf8.ValidString(value) && strings.TrimSpace(value) == value &&
		value != "" && utf8.RuneCountInString(value) <= max
}

func validOpaque(value string, max int) bool {
	if value == "" || len(value) > max {
		return false
	}
	for index := 0; index < len(value); index++ {
		character := value[index]
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			character == '-' || character == '_' || character == '.' {
			continue
		}
		return false
	}
	return true
}
