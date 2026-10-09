package app

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/profilepicture"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"time"
)

type OwnPicture struct {
	Owner, AssetID, URL string
	Revision            int64
}
type PictureWriteCommand struct {
	Owner, AssetID, URL string
	ExpectedRevision    int64
	JPEG                []byte
	ChangedAt           time.Time
	Idempotency         ports.Idempotency
	Audit               audit.Event
}
type PictureRepository interface {
	GetOwnPicture(context.Context, string) (OwnPicture, error)
	WriteOwnPicture(context.Context, PictureWriteCommand) (OwnPicture, error)
	// Opaque current-image capability. It never returns account/profile fields.
	ReadPublicPicture(context.Context, string) ([]byte, error)
}

type PictureUpdate struct {
	Image            []byte
	Crop             *profilepicture.Crop
	Remove           bool
	ExpectedRevision int64
}

func (a App) OwnProfilePicture(ctx context.Context, authorization string) (OwnPicture, error) {
	user, err := a.currentUserForAuditedOperation(ctx, authorization)
	if err != nil {
		return OwnPicture{}, err
	}
	if a.Pictures == nil || a.Audits == nil {
		return OwnPicture{}, ports.ErrUnavailable
	}
	result, err := a.Pictures.GetOwnPicture(ctx, user.ID)
	if err != nil {
		return OwnPicture{}, err
	}
	if result.Owner != user.ID || result.Revision < 1 {
		return OwnPicture{}, ports.ErrUnavailable
	}
	if err = a.Audits.AppendAuditEvent(ctx, a.auditEvent(ctx, user.ID, user.ID, audit.UserViewed, "user", user.ID, audit.Succeeded)); err != nil {
		return OwnPicture{}, err
	}
	return result, nil
}

type pictureAdmissionKey struct{}
type pictureAdmission struct {
	Owner          string
	CredentialHash [32]byte
}

// AdmitProfilePictureUpload authenticates and limits before HTTP reads upload bytes.
func (a App) AdmitProfilePictureUpload(ctx context.Context, authorization string) (context.Context, error) {
	user, err := a.authenticateCurrentUser(ctx, authorization)
	if err != nil {
		return nil, err
	}
	if err = a.pictureProcessingAdmission(ctx, authorization, user.ID); err != nil {
		return nil, err
	}
	return context.WithValue(ctx, pictureAdmissionKey{}, pictureAdmission{Owner: user.ID, CredentialHash: sha256.Sum256([]byte(authorization))}), nil
}
func (a App) pictureProcessingAdmission(ctx context.Context, authorization, owner string) error {
	if admission, ok := ctx.Value(pictureAdmissionKey{}).(pictureAdmission); ok && admission.Owner == owner && admission.CredentialHash == sha256.Sum256([]byte(authorization)) {
		return nil
	}

	if a.PictureRateLimiter == nil {
		return ports.ErrUnavailable
	}
	if !a.PictureRateLimiter.Allow(owner, a.Clock.Now().UTC()) {
		return ErrRateLimited
	}
	return nil
}
func (a App) PreviewProfilePicture(ctx context.Context, authorization string, data []byte) (profilepicture.Prepared, error) {
	user, err := a.currentUserForAuditedOperation(ctx, authorization)
	if err != nil {
		return profilepicture.Prepared{}, err
	}
	if a.PictureProcessor == nil || a.Audits == nil {
		return profilepicture.Prepared{}, ports.ErrUnavailable
	}
	if len(data) == 0 || len(data) > profilepicture.MaxUploadBytes {
		return profilepicture.Prepared{}, ports.ErrInvalidArgument
	}
	if err = a.pictureProcessingAdmission(ctx, authorization, user.ID); err != nil {
		return profilepicture.Prepared{}, err
	}
	result, err := a.PictureProcessor.Process(ctx, data, nil)
	if err != nil {
		return profilepicture.Prepared{}, err
	}
	current, err := a.authenticateCurrentUser(ctx, authorization)
	if err != nil {
		return profilepicture.Prepared{}, err
	}
	if current.ID != user.ID {
		return profilepicture.Prepared{}, ErrUnauthenticated
	}
	if result.Width < 1 || result.Height < 1 || result.Width > profilepicture.PreviewEdge || result.Height > profilepicture.PreviewEdge || len(result.JPEG) == 0 || len(result.JPEG) > 1048576 {
		return profilepicture.Prepared{}, ports.ErrUnavailable
	}
	if err = a.Audits.AppendAuditEvent(ctx, a.auditEvent(ctx, user.ID, user.ID, audit.UserViewed, "user", user.ID, audit.Succeeded)); err != nil {
		return profilepicture.Prepared{}, err
	}
	return result, nil
}
func (a App) UpdateProfilePicture(ctx context.Context, authorization, key string, input PictureUpdate) (OwnPicture, error) {
	user, err := a.currentUserForAuditedOperation(ctx, authorization)
	if err != nil {
		return OwnPicture{}, err
	}
	if a.Pictures == nil || a.PictureProcessor == nil || a.NewPictureID == nil || a.PictureLocation == nil {
		return OwnPicture{}, ports.ErrUnavailable
	}
	if input.ExpectedRevision < 1 || !validTimeZoneMutationKey(key) || len(input.Image) > profilepicture.MaxUploadBytes {
		return OwnPicture{}, ports.ErrInvalidArgument
	}
	if input.Remove {
		if len(input.Image) != 0 || input.Crop != nil {
			return OwnPicture{}, ports.ErrInvalidArgument
		}
	} else if len(input.Image) == 0 || input.Crop == nil {
		return OwnPicture{}, ports.ErrInvalidArgument
	}
	if err = a.pictureProcessingAdmission(ctx, authorization, user.ID); err != nil {
		return OwnPicture{}, err
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return OwnPicture{}, ports.ErrInvalidArgument
	}
	hash := sha256.Sum256(encoded)
	var jpeg []byte
	var assetID, location string
	if !input.Remove {
		prepared, err := a.PictureProcessor.Process(ctx, input.Image, input.Crop)
		if err != nil {
			return OwnPicture{}, err
		}
		if prepared.Width != profilepicture.PictureEdge || prepared.Height != profilepicture.PictureEdge || len(prepared.JPEG) < 4 || len(prepared.JPEG) > 1048576 {
			return OwnPicture{}, ports.ErrUnavailable
		}
		jpeg = prepared.JPEG
		assetID = a.NewPictureID()
		location = a.PictureLocation(assetID)
		if assetID == "" || location == "" {
			return OwnPicture{}, ports.ErrUnavailable
		}
	}
	// A session revoked during potentially expensive decoding cannot write afterward.
	current, err := a.authenticateCurrentUser(ctx, authorization)
	if err != nil {
		return OwnPicture{}, err
	}
	if current.ID != user.ID {
		return OwnPicture{}, ErrUnauthenticated
	}
	changedAt := a.Clock.Now().UTC().Truncate(time.Microsecond)
	event := a.auditEvent(ctx, user.ID, user.ID, audit.ResourceUpdated, "user", user.ID, audit.Succeeded)
	event.OccurredAt = changedAt
	result, err := a.Pictures.WriteOwnPicture(ctx, PictureWriteCommand{Owner: user.ID, ExpectedRevision: input.ExpectedRevision, AssetID: assetID, URL: location, JPEG: jpeg, ChangedAt: changedAt, Audit: event, Idempotency: ports.Idempotency{PrincipalID: user.ID, Operation: "account.profile.picture.update", Key: key, RequestHash: hash[:]}})
	if err != nil {
		return OwnPicture{}, err
	}
	if result.Owner != user.ID || result.Revision < 1 {
		return OwnPicture{}, ports.ErrUnavailable
	}
	return result, nil
}
func (a App) PublicProfilePicture(ctx context.Context, id string) ([]byte, error) {
	if a.Pictures == nil {
		return nil, ports.ErrUnavailable
	}
	return a.Pictures.ReadPublicPicture(ctx, id)
}
