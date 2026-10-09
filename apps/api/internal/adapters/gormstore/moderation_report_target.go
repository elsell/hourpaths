package gormstore

import (
	"encoding/json"
	"time"

	pathstore "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/path"
	app "github.com/elsell/hour-paths/apps/api/internal/app/moderation"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/moderation"
	pathdomain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
)

type reportSnapshot struct {
	Access   app.TargetAccess
	Evidence []byte
	JPEG     []byte
}

// Evidence uses an explicit allowlist of visible content, never a full database
// row, private practice notes, tokens, or unrelated profile fields.
type reportEvidence struct {
	Title           string    `json:"title,omitempty"`
	Text            string    `json:"text,omitempty"`
	Username        string    `json:"username,omitempty"`
	ContentKind     string    `json:"contentKind,omitempty"`
	DurationSeconds int64     `json:"durationSeconds,omitempty"`
	TargetSeconds   int64     `json:"targetSeconds,omitempty"`
	Version         int64     `json:"version,omitempty"`
	OccurredAt      time.Time `json:"occurredAt"`
}

func resolveReportSnapshot(tx *gorm.DB, viewer string, target domain.Target, at time.Time) (reportSnapshot, error) {
	var active int64
	if err := tx.Model(&userModel{}).Where("id=? AND status='active'", viewer).Count(&active).Error; err != nil {
		return reportSnapshot{}, err
	}
	if active != 1 {
		return reportSnapshot{}, ports.ErrNotFound
	}
	result := reportSnapshot{Access: app.TargetAccess{Target: target}}
	var evidence reportEvidence
	switch target.Kind {
	case domain.Profile:
		var row struct {
			ID, Username, DisplayName, Description string
			UpdatedAt                              time.Time
		}
		err := tx.Table("user_models u").Select("u.id,u.username,u.display_name,COALESCE(u.description,'') AS description,u.updated_at").Where("u.id=? AND u.status='active'", target.ID).
			Where(`NOT EXISTS(SELECT 1 FROM block_models b WHERE (b.blocker_user_id=? AND b.blocked_user_id=u.id) OR (b.blocker_user_id=u.id AND b.blocked_user_id=?))`, viewer, viewer).Take(&row).Error
		if err != nil {
			return result, err
		}
		result.Access.SubjectUserID = row.ID
		evidence = reportEvidence{Title: row.DisplayName, Text: row.Description, Username: row.Username, OccurredAt: row.UpdatedAt}
		var picture struct{ JPEG []byte }
		if err = tx.Table("user_profile_picture_models").Select("jpeg").Where("user_id=?", row.ID).Find(&picture).Error; err != nil {
			return result, err
		}
		result.JPEG = picture.JPEG
	case domain.Path:
		path, err := pathstore.New(tx).Get(tx.Statement.Context, viewer, pathdomain.ID(target.ID))
		if err != nil {
			return result, err
		}
		result.Access.SubjectUserID = path.OwnerUserID
		result.Access.PathID = string(path.ID)
		evidence = reportEvidence{Title: path.Name, OccurredAt: path.UpdatedAt}
	case domain.FeedEvent:
		item, err := NewSocialFeedRepository(tx).GetPracticeCandidate(tx.Statement.Context, viewer, target.ID, at)
		if err != nil {
			return result, err
		}
		result.Access.SubjectUserID = item.ParticipantID
		result.Access.PathID = item.PathID
		evidence = reportEvidence{Title: item.PathName, Username: item.Username, ContentKind: string(item.Type), DurationSeconds: item.DurationSeconds, OccurredAt: item.PublishedAt}
		if item.Achievement != nil {
			evidence.TargetSeconds = item.Achievement.TargetSeconds
		}
	case domain.Comment:
		var source struct{ SocialFeedEventID string }
		if err := tx.Table("social_practice_comment_models").Select("social_feed_event_id").Where("id=?", target.ID).Take(&source).Error; err != nil {
			return result, err
		}
		item, err := resolvePracticeComment(tx, viewer, source.SocialFeedEventID, target.ID)
		if err != nil {
			return result, err
		}
		result.Access.SubjectUserID = item.Comment.AuthorID
		result.Access.PathID = item.Target.PathID
		evidence = reportEvidence{Text: item.Comment.Text, Version: item.Comment.Version, OccurredAt: item.Comment.UpdatedAt}
	case domain.Nudge:
		var row socialNudgeModel
		err := tx.Table("social_nudge_models n").Select("n.*").Joins("JOIN user_models sender ON sender.id=n.sender_user_id AND sender.status='active'").
			Joins("JOIN notification_models notice ON notice.nudge_id=n.id AND notice.kind='nudge_received'").
			Where("notice.id=? AND notice.recipient_user_id=? AND n.recipient_user_id=?", target.ID, viewer, viewer).
			Where(`NOT EXISTS(SELECT 1 FROM block_models b WHERE (b.blocker_user_id=n.sender_user_id AND b.blocked_user_id=n.recipient_user_id) OR (b.blocker_user_id=n.recipient_user_id AND b.blocked_user_id=n.sender_user_id))`).Take(&row).Error
		if err != nil {
			return result, err
		}
		if _, err = pathstore.New(tx).Get(tx.Statement.Context, viewer, pathdomain.ID(row.PathID)); err != nil {
			return result, err
		}
		result.Access.SubjectUserID = row.SenderUserID
		result.Access.PathID = row.PathID
		evidence = reportEvidence{Text: row.Preset, ContentKind: row.ContentKind, OccurredAt: row.SentAt}
	default:
		return result, ports.ErrInvalidArgument
	}
	if result.Access.SubjectUserID == "" {
		return result, ports.ErrNotFound
	}
	var err error
	result.Evidence, err = json.Marshal(evidence)
	return result, err
}
