package app

import (
	"context"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

// AccountExportProfile is an explicit personal-data projection. It deliberately
// contains no session, provider subject, credential, or administrative metadata.
type AccountExportProfile struct {
	UserID, Email, Username, DisplayName, Description string
	Visibility                                        identity.ProfileVisibility
	TimeZone                                          string
	FirstDayOfWeek                                    int
	CreatedAt, UpdatedAt                              time.Time
}
type AccountExportRepository interface {
	ReadExportProfile(context.Context, string) (AccountExportProfile, error)
}

func (a App) ExportOwnAccountProfile(ctx context.Context, authorization string) (AccountExportProfile, error) {
	if a.Auth == nil || a.Users == nil || a.AccountExport == nil || a.Clock == nil || a.Audits == nil {
		return AccountExportProfile{}, ports.ErrUnavailable
	}
	user, err := a.currentUserForAuditedOperation(ctx, authorization)
	if err != nil {
		return AccountExportProfile{}, err
	}
	if user.Status != identity.StatusActive {
		return AccountExportProfile{}, ErrUnauthenticated
	}
	profile, err := a.AccountExport.ReadExportProfile(ctx, user.ID)
	if err != nil {
		return AccountExportProfile{}, err
	}
	if profile.UserID != user.ID || profile.CreatedAt.IsZero() || profile.UpdatedAt.Before(profile.CreatedAt) || !validWeekStart(profile.FirstDayOfWeek) || identity.ValidateIANATimeZone(identity.IANATimeZone(profile.TimeZone)) != nil {
		return AccountExportProfile{}, ports.ErrUnavailable
	}
	if err = a.Audits.AppendAuditEvent(ctx, a.auditEvent(ctx, user.ID, user.ID, audit.ResourceViewed, "account_export", user.ID, audit.Succeeded)); err != nil {
		return AccountExportProfile{}, err
	}
	return profile, nil
}
