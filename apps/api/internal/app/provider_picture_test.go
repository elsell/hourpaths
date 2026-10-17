package app

import (
	"context"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/profilepicture"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"testing"
	"time"
)

type controlledProviderPicture struct{ err error }

func (f controlledProviderPicture) Fetch(context.Context, string) ([]byte, error) {
	return []byte("provider-image"), f.err
}

type controlledPictureProcessor struct{}

func (controlledPictureProcessor) Process(_ context.Context, _ []byte, crop *profilepicture.Crop) (profilepicture.Prepared, error) {
	if crop == nil {
		return profilepicture.Prepared{JPEG: []byte("normalized-preview"), Width: 800, Height: 600}, nil
	}
	if crop.X != 100 || crop.Y != 0 || crop.Size != 600 {
		return profilepicture.Prepared{}, ports.ErrInvalidArgument
	}
	return profilepicture.Prepared{JPEG: []byte("normalized-square"), Width: 512, Height: 512}, nil
}

type controlledImportPictures struct {
	revision int64
	written  *PictureWriteCommand
}

func (p *controlledImportPictures) GetOwnPicture(context.Context, string) (OwnPicture, error) {
	return OwnPicture{Owner: "alice", Revision: p.revision}, nil
}
func (p *controlledImportPictures) ReadPublicPicture(context.Context, string) ([]byte, error) {
	return nil, ports.ErrNotFound
}
func (p *controlledImportPictures) WriteOwnPicture(_ context.Context, command PictureWriteCommand) (OwnPicture, error) {
	if command.Owner != "alice" || command.ExpectedRevision != p.revision {
		return OwnPicture{}, ports.ErrConflict
	}
	p.written = &command
	p.revision++
	return OwnPicture{Owner: "alice", Revision: p.revision, URL: command.URL, AssetID: command.AssetID}, nil
}
func TestProviderImportUsesNormalizedImageAndPreservesRemoval(t *testing.T) {
	for _, tc := range []struct {
		name      string
		revision  int64
		failure   error
		wantWrite bool
	}{
		{"initial import", 1, nil, true}, {"removed or replaced", 2, nil, false}, {"fetch unavailable", 1, ports.ErrUnavailable, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pictures := &controlledImportPictures{revision: tc.revision}
			events := []audit.Event{}
			a := App{Auth: fakeAuth{principal: ports.Principal{UserID: "alice", Scopes: []string{"api:user"}}}, Users: fakeUsers{user: identity.User{ID: "alice", Status: identity.StatusActive}},
				ProviderPictures: controlledProviderPicture{tc.failure}, PictureProcessor: controlledPictureProcessor{}, Pictures: pictures,
				Audits: fakeAudits{events: &events}, AuditRateLimiter: fakeAuditRateLimiter{}, PictureRateLimiter: fakeAuditRateLimiter{}, Clock: fakeClock{now: time.Now()},
				NewPictureID: func() string { return "picture" }, PictureLocation: func(id string) string { return "https://api.example/pictures/" + id },
			}
			a.importProviderPicture(context.Background(), "active-session", "https://provider.example/photo")
			if (pictures.written != nil) != tc.wantWrite {
				t.Fatalf("write=%v", pictures.written != nil)
			}
			if pictures.written != nil && (string(pictures.written.JPEG) != "normalized-square" || pictures.written.Audit.OwnerUserID != "alice") {
				t.Fatal("picture did not use audited normalized write")
			}
		})
	}
}
