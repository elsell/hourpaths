package httpserver

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/profileimage"
	"github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/profilepicture"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"image"
	"image/png"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type pictureRouteStore struct {
	rows   map[string]app.OwnPicture
	images map[string][]byte
	writes int
}

func (s *pictureRouteStore) GetOwnPicture(_ context.Context, owner string) (app.OwnPicture, error) {
	row, ok := s.rows[owner]
	if !ok {
		return app.OwnPicture{}, ports.ErrNotFound
	}
	return row, nil
}
func (s *pictureRouteStore) WriteOwnPicture(_ context.Context, c app.PictureWriteCommand) (app.OwnPicture, error) {
	row, err := s.GetOwnPicture(context.Background(), c.Owner)
	if err != nil {
		return row, err
	}
	if row.Revision != c.ExpectedRevision {
		return row, ports.ErrConflict
	}
	delete(s.images, row.AssetID)
	row.AssetID = c.AssetID
	row.URL = c.URL
	row.Revision++
	s.rows[c.Owner] = row
	if len(c.JPEG) > 0 {
		s.images[c.AssetID] = c.JPEG
	}
	s.writes++
	return row, nil
}
func (s *pictureRouteStore) ReadPublicPicture(_ context.Context, id string) ([]byte, error) {
	data, ok := s.images[id]
	if !ok {
		return nil, ports.ErrNotFound
	}
	return data, nil
}

type pictureRouteProcessor struct{ after func() }

func (p pictureRouteProcessor) Process(_ context.Context, data []byte, crop *profilepicture.Crop) (profilepicture.Prepared, error) {
	result, err := profileimage.Transform(data, crop)
	if p.after != nil {
		p.after()
	}
	if err != nil {
		return result, ports.ErrInvalidArgument
	}
	return result, nil
}
func TestPictureHTTPAdmissionAndOwnerIsolation(t *testing.T) {
	state := &deletionRouteState{commands: map[string]app.AccountDeletionCommand{}}
	store := &pictureRouteStore{rows: map[string]app.OwnPicture{"owner": {Owner: "owner", Revision: 1}, "other": {Owner: "other", Revision: 1}}, images: map[string][]byte{}}
	application := app.App{Auth: profileRouteAuth{state}, Users: deletionRouteUsers{}, Pictures: store, PictureProcessor: pictureRouteProcessor{}, PictureRateLimiter: docsLimiter{}, NewPictureID: func() string { return "2d4874f6-c207-447b-92e7-07b23a655bea" }, PictureLocation: func(id string) string { return "https://api.example.test/v1/profile-pictures/" + id }, Audits: timeZoneRouteAudits{}, AuditRateLimiter: docsLimiter{}, Clock: docsClock{now: time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)}}
	handler, _ := New(application, nil, Options{})
	var raw bytes.Buffer
	png.Encode(&raw, image.NewRGBA(image.Rect(0, 0, 16, 16)))
	imageBody := base64.StdEncoding.EncodeToString(raw.Bytes())
	preview := `{"image":"` + imageBody + `"}`
	update := `{"image":"` + imageBody + `","crop":{"x":0,"y":0,"size":16},"expectedRevision":1,"remove":false}`
	call := func(method, path, credential, body string, want int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", credential)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "picture-http-key-0001")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		if response.Code != want {
			t.Fatalf("%s %s got %d want %d: %s", method, path, response.Code, want, response.Body.String())
		}
		return response
	}
	for _, credential := range []string{"", "malformed", "Bearer expired", "Bearer provider-jwt", "Bearer provisional"} {
		call("GET", "/v1/me/profile/picture", credential, "", 401)
		call("POST", "/v1/me/profile/picture/preview", credential, preview, 401)
		call("PUT", "/v1/me/profile/picture", credential, update, 401)
	}
	call("POST", "/v1/me/profile/picture/preview", "Bearer dependency-failed", preview, 503)
	call("POST", "/v1/me/profile/picture/preview", "Bearer owner", `{"image":"bm90IGFuIGltYWdl"}`, 400)
	call("POST", "/v1/me/profile/picture/preview", "Bearer owner", preview, 200)
	if store.writes != 0 {
		t.Fatal("preview changed the picture")
	}
	call("PUT", "/v1/me/profile/picture", "Bearer owner", strings.TrimSuffix(update, "}")+`,"owner":"other"}`, 422)
	response := call("PUT", "/v1/me/profile/picture", "Bearer owner", update, 200)
	var envelope struct {
		Data struct {
			UserID, URL string
			Revision    int64
		}
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.UserID != "owner" || store.rows["other"].Revision != 1 {
		t.Fatal("owner boundary lost")
	}
	media := call("GET", "/v1/profile-pictures/2d4874f6-c207-447b-92e7-07b23a655bea", "", "", 200)
	if media.Header().Get("Content-Type") != "image/jpeg" || media.Header().Get("Cache-Control") != "no-store" || media.Header().Get("X-Content-Type-Options") != "nosniff" || !bytes.HasPrefix(media.Body.Bytes(), []byte{0xff, 0xd8}) {
		t.Fatal("unsafe media response")
	}
	call("PUT", "/v1/me/profile/picture", "Bearer owner", `{"image":"","expectedRevision":2,"remove":true}`, 200)
	call("GET", "/v1/profile-pictures/2d4874f6-c207-447b-92e7-07b23a655bea", "", "", 404)
	application.PictureProcessor = pictureRouteProcessor{after: func() { state.commands["owner"] = app.AccountDeletionCommand{} }}
	handler, _ = New(application, nil, Options{})
	call("PUT", "/v1/me/profile/picture", "Bearer owner", strings.Replace(update, `"expectedRevision":1`, `"expectedRevision":3`, 1), 401)
	if store.rows["owner"].Revision != 3 {
		t.Fatal("revoked processing session changed picture")
	}
}

type unreadPictureBody struct{ read bool }

func (b *unreadPictureBody) Read([]byte) (int, error) { b.read = true; return 0, io.EOF }
func TestPictureRejectsUnauthenticatedBeforeReadingUpload(t *testing.T) {
	handler, _ := New(app.App{Auth: timeZoneRouteAuth{err: app.ErrUnauthenticated}}, nil, Options{})
	body := &unreadPictureBody{}
	req := httptest.NewRequest("POST", "/v1/me/profile/picture/preview", body)
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Code != 401 || body.read {
		t.Fatalf("status=%d read=%v", response.Code, body.read)
	}
}
