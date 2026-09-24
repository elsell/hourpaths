package pathstore

import (
	"context"
	"errors"
	"strings"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	DB            *gorm.DB
	WarningPolicy application.InvitationWarningPolicy
}

type model struct {
	ID          string `gorm:"primaryKey"`
	OwnerUserID string
	Name        string
	Visibility  string

	IntervalGoalTargetSeconds *int64
	IntervalGoalRecurrence    *string
	IntervalGoalStartMinute   *int16
	IntervalGoalStartHour     *int16
	IntervalGoalStartWeekday  *int16
	IntervalGoalStartDay      *int16
	IntervalGoalStartMonth    *int16
	OverallTargetSeconds      *int64
	ArchivedAt                *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (model) TableName() string { return "path_models" }

type membershipModel struct {
	PathID   string `gorm:"primaryKey"`
	UserID   string `gorm:"primaryKey"`
	Role     string
	JoinedAt time.Time `gorm:"autoCreateTime"`
}

func (membershipModel) TableName() string { return "path_membership_models" }

type auditModel struct {
	ID, OwnerUserID, ActorUserID, TargetType, TargetID, CorrelationID string
	Action                                                            audit.Action
	Outcome                                                           audit.Outcome
	OccurredAt                                                        time.Time
}

type idempotencyModel struct {
	PrincipalID, Operation, Key string
	RequestHash                 []byte
	ResourceID                  string
	CreatedAt                   time.Time
}

func (idempotencyModel) TableName() string { return "idempotency_models" }

type authorizationOutboxModel struct {
	ID, ResourceType, ResourceID, Relation, SubjectType, SubjectID string
	OwnerUserID, ActorUserID                                       string
	Operation                                                      ports.AuthorizationOperation
	LockedBy                                                       string
	CreatedAt                                                      time.Time
}

func (authorizationOutboxModel) TableName() string { return "authorization_outbox_models" }

func (auditModel) TableName() string { return "audit_event_models" }

func New(db *gorm.DB) *Repository {
	return &Repository{DB: db, WarningPolicy: application.VisibilityInvitationWarningPolicy{}}
}

var errInvalidPersistedPath = errors.New("persisted Path is invalid")

func (r *Repository) Create(ctx context.Context, entity domain.Entity, change ports.AuthorizationChange, idempotency ports.Idempotency, event audit.Event) (domain.Entity, bool, error) {
	if r == nil || r.DB == nil || !validEvent(event, audit.ResourceCreated, entity) || event.ActorUserID != entity.OwnerUserID ||
		!validEntityForPersistence(entity) || entity.CreatedAt.IsZero() || entity.UpdatedAt.IsZero() || idempotency.PrincipalID != entity.OwnerUserID || idempotency.Operation == "" || idempotency.Key == "" || len(idempotency.RequestHash) != 32 ||
		change.ID == "" || change.Relation != "creator" || change.ResourceType != "path" || change.ResourceID != string(entity.ID) || change.SubjectType != "user" || change.SubjectID != entity.OwnerUserID || change.OwnerUserID != entity.OwnerUserID || change.ActorUserID != event.ActorUserID || change.Operation != ports.AuthorizationTouch || change.Lease <= 0 || change.LockedBy == "" {
		return domain.Entity{}, false, ports.ErrInvalidArgument
	}
	result := domain.Entity{}
	replayed := false
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		reservation := idempotencyModel{PrincipalID: idempotency.PrincipalID, Operation: idempotency.Operation, Key: idempotency.Key, RequestHash: append([]byte(nil), idempotency.RequestHash...), ResourceID: string(entity.ID), CreatedAt: entity.CreatedAt}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&reservation)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 0 {
			var existing idempotencyModel
			if err := tx.Where("principal_id = ? AND operation = ? AND key = ?", idempotency.PrincipalID, idempotency.Operation, idempotency.Key).First(&existing).Error; err != nil {
				return err
			}
			if string(existing.RequestHash) != string(idempotency.RequestHash) {
				return ports.ErrIdempotencyConflict
			}
			var row model
			if err := tx.Where("owner_user_id = ? AND id = ?", entity.OwnerUserID, existing.ResourceID).First(&row).Error; err != nil {
				return err
			}
			decoded, decodeErr := toEntity(row)
			if decodeErr != nil {
				return decodeErr
			}
			result = decoded
			replayed = true
			return nil
		}
		if err := tx.Create(fromEntity(entity)).Error; err != nil {
			return err
		}
		if err := tx.Create(&membershipModel{PathID: string(entity.ID), UserID: entity.OwnerUserID, Role: "participant"}).Error; err != nil {
			return err
		}
		outbox := authorizationOutboxModel{ID: change.ID, ResourceType: change.ResourceType, ResourceID: change.ResourceID, Relation: change.Relation, SubjectType: change.SubjectType, SubjectID: change.SubjectID, OwnerUserID: change.OwnerUserID, ActorUserID: change.ActorUserID, Operation: change.Operation, LockedBy: change.LockedBy, CreatedAt: entity.CreatedAt}
		if err := tx.Create(&outbox).Error; err != nil {
			return err
		}
		if err := tx.Model(&outbox).Update("locked_until", gorm.Expr("CURRENT_TIMESTAMP + (? * INTERVAL '1 millisecond')", change.Lease.Milliseconds())).Error; err != nil {
			return err
		}
		if visibilityOutbox, present := visibilityAuthorizationOutbox(entity, change); present {
			created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&visibilityOutbox)
			if created.Error != nil {
				return created.Error
			}
			if created.RowsAffected == 0 {
				var existing authorizationOutboxModel
				if err := tx.Where("id = ?", visibilityOutbox.ID).First(&existing).Error; err != nil {
					return err
				}
				if !sameAuthorizationRelationship(existing, visibilityOutbox) {
					return errors.New("public Path authorization outbox conflicts with cutover bridge")
				}
			}
		}
		if err := tx.Create(fromAudit(event)).Error; err != nil {
			return err
		}
		result = entity
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = ports.ErrNotFound
	}
	return result, replayed, err
}

func sameAuthorizationRelationship(left, right authorizationOutboxModel) bool {
	return left.ID == right.ID && left.ResourceType == right.ResourceType && left.ResourceID == right.ResourceID &&
		left.Relation == right.Relation && left.SubjectType == right.SubjectType && left.SubjectID == right.SubjectID &&
		left.OwnerUserID == right.OwnerUserID && left.ActorUserID == right.ActorUserID && left.Operation == right.Operation
}

func visibilityAuthorizationOutbox(entity domain.Entity, creator ports.AuthorizationChange) (authorizationOutboxModel, bool) {
	relation, subjectID := "followers_owner", entity.OwnerUserID
	if entity.Visibility == "public" {
		relation, subjectID = "public_viewer", "*"
	} else if entity.Visibility != "followers" {
		return authorizationOutboxModel{}, false
	}
	return authorizationOutboxModel{
		ID:           creator.ID + "-" + strings.ReplaceAll(relation, "_", "-"),
		ResourceType: "path",
		ResourceID:   string(entity.ID),
		Relation:     relation,
		SubjectType:  "user",
		SubjectID:    subjectID,
		OwnerUserID:  entity.OwnerUserID,
		ActorUserID:  creator.ActorUserID,
		Operation:    ports.AuthorizationTouch,
		LockedBy:     creator.LockedBy,
		CreatedAt:    entity.CreatedAt,
	}, true
}

func (r *Repository) List(ctx context.Context, owner string, page application.PageRequest) (application.Page, error) {
	if strings.TrimSpace(owner) == "" || r == nil || r.DB == nil || page.Limit < 1 || page.Limit > 100 || page.Snapshot.IsZero() ||
		(page.AfterID == "") != page.AfterCreated.IsZero() || (!page.AfterCreated.IsZero() && page.AfterCreated.After(page.Snapshot)) {
		return application.Page{}, ports.ErrInvalidArgument
	}
	query := r.DB.WithContext(ctx).
		Table("path_models").
		Select("DISTINCT path_models.*").
		Joins("LEFT JOIN path_membership_models ON path_membership_models.path_id = path_models.id AND path_membership_models.user_id = ?", owner).
		Where("(path_models.owner_user_id = ? OR path_membership_models.user_id = ?) AND path_models.created_at <= ?", owner, owner, page.Snapshot)
	if page.Archived {
		query = query.Where("path_models.archived_at IS NOT NULL")
	} else {
		query = query.Where("path_models.archived_at IS NULL")
	}
	if page.AfterID != "" {
		query = query.Where("path_models.created_at > ? OR (path_models.created_at = ? AND path_models.id > ?)", page.AfterCreated, page.AfterCreated, page.AfterID)
	}
	var rows []model
	if err := query.Order("path_models.created_at ASC, path_models.id ASC").Limit(page.Limit + 1).Find(&rows).Error; err != nil {
		return application.Page{}, err
	}
	hasMore := len(rows) > page.Limit
	if hasMore {
		rows = rows[:page.Limit]
	}
	items := make([]domain.Entity, len(rows))
	for i := range rows {
		item, err := toEntity(rows[i])
		if err != nil {
			return application.Page{}, err
		}
		items[i] = item
	}
	return application.Page{Items: items, HasMore: hasMore}, nil
}

func (r *Repository) Get(ctx context.Context, owner string, id domain.ID) (domain.Entity, error) {
	if strings.TrimSpace(owner) == "" || id == "" || r == nil || r.DB == nil {
		return domain.Entity{}, ports.ErrInvalidArgument
	}
	var row model
	err := r.DB.WithContext(ctx).
		Table("path_models").
		Select("path_models.*").
		Joins("LEFT JOIN path_membership_models ON path_membership_models.path_id = path_models.id AND path_membership_models.user_id = ?", owner).
		Where(`path_models.id = ? AND (
path_models.owner_user_id = ? OR path_membership_models.user_id = ? OR (
  NOT EXISTS (SELECT 1 FROM block_models path_block
    WHERE (path_block.blocker_user_id = ? AND path_block.blocked_user_id = path_models.owner_user_id)
       OR (path_block.blocker_user_id = path_models.owner_user_id AND path_block.blocked_user_id = ?))
  AND (path_models.visibility = 'public' OR (
    path_models.visibility = 'followers' AND EXISTS (
      SELECT 1 FROM follow_models path_follow
      WHERE path_follow.follower_user_id = ? AND path_follow.following_user_id = path_models.owner_user_id
    )
  ))
))`, id, owner, owner, owner, owner, owner).
		First(&row).Error
	if err == gorm.ErrRecordNotFound {
		return domain.Entity{}, ports.ErrNotFound
	}
	if err != nil {
		return domain.Entity{}, err
	}
	return toEntity(row)
}

func (r *Repository) Update(ctx context.Context, owner string, entity domain.Entity, event audit.Event) error {
	if entity.OwnerUserID != owner || !validEvent(event, audit.ResourceUpdated, entity) {
		return ports.ErrInvalidArgument
	}
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model{}).Where("owner_user_id = ? AND id = ?", owner, entity.ID).Updates(map[string]any{"name": entity.Name, "visibility": entity.Visibility, "updated_at": entity.UpdatedAt})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ports.ErrNotFound
		}
		return tx.Create(fromAudit(event)).Error
	})
}

func validEvent(event audit.Event, action audit.Action, entity domain.Entity) bool {
	return event.Valid() && event.Action == action && event.Outcome == audit.Succeeded && event.TargetType == "path" && event.TargetID == string(entity.ID) && event.OwnerUserID == entity.OwnerUserID
}

func validEntityForPersistence(entity domain.Entity) bool {
	validated, err := domain.New(entity.ID, entity.OwnerUserID, entity.Attributes)
	return err == nil && validated.ID == entity.ID && validated.OwnerUserID == entity.OwnerUserID && validated.Attributes == entity.Attributes
}

func fromEntity(entity domain.Entity) *model {
	row := &model{ID: string(entity.ID), OwnerUserID: entity.OwnerUserID, Name: entity.Name, Visibility: entity.Visibility, CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt}
	if !entity.ArchivedAt.IsZero() {
		archivedAt := entity.ArchivedAt.UTC()
		row.ArchivedAt = &archivedAt
	}
	if entity.IntervalGoal.Present {
		row.IntervalGoalTargetSeconds = pointer(entity.IntervalGoal.TargetSeconds)
		row.IntervalGoalRecurrence = pointer(string(entity.IntervalGoal.Recurrence))
		switch entity.IntervalGoal.Recurrence {
		case domain.RecurrenceHourly:
			row.IntervalGoalStartMinute = pointer(int16(entity.IntervalGoal.Alignment.Minute))
		case domain.RecurrenceDaily:
			row.IntervalGoalStartHour = pointer(int16(entity.IntervalGoal.Alignment.Hour))
		case domain.RecurrenceWeekly:
			row.IntervalGoalStartWeekday = pointer(int16(entity.IntervalGoal.Alignment.ISOWeekday))
		case domain.RecurrenceMonthly:
			row.IntervalGoalStartDay = pointer(int16(entity.IntervalGoal.Alignment.Day))
		case domain.RecurrenceYearly:
			row.IntervalGoalStartDay = pointer(int16(entity.IntervalGoal.Alignment.Day))
			row.IntervalGoalStartMonth = pointer(int16(entity.IntervalGoal.Alignment.Month))
		}
	}
	if entity.OverallTarget.Present {
		row.OverallTargetSeconds = pointer(entity.OverallTarget.TargetSeconds)
	}
	return row
}

func toEntity(row model) (domain.Entity, error) {
	intervalGoal, err := intervalGoalFromModel(row)
	if err != nil {
		return domain.Entity{}, err
	}
	overallTarget := domain.OverallTarget{}
	if row.OverallTargetSeconds != nil {
		overallTarget = domain.OverallTarget{Present: true, TargetSeconds: *row.OverallTargetSeconds}
	}
	entity, err := domain.New(domain.ID(row.ID), row.OwnerUserID, domain.Attributes{
		Name: row.Name, Visibility: row.Visibility, IntervalGoal: intervalGoal, OverallTarget: overallTarget,
	})
	if err != nil {
		return domain.Entity{}, errInvalidPersistedPath
	}
	entity.CreatedAt = row.CreatedAt.UTC()
	entity.UpdatedAt = row.UpdatedAt.UTC()
	if row.ArchivedAt != nil {
		entity.ArchivedAt = row.ArchivedAt.UTC()
		if entity.ArchivedAt.Before(entity.CreatedAt) || entity.UpdatedAt.Before(entity.ArchivedAt) {
			return domain.Entity{}, errInvalidPersistedPath
		}
	}
	return entity, nil
}

func intervalGoalFromModel(row model) (domain.IntervalGoal, error) {
	absent := row.IntervalGoalTargetSeconds == nil && row.IntervalGoalRecurrence == nil &&
		row.IntervalGoalStartMinute == nil && row.IntervalGoalStartHour == nil &&
		row.IntervalGoalStartWeekday == nil && row.IntervalGoalStartDay == nil && row.IntervalGoalStartMonth == nil
	if absent {
		return domain.IntervalGoal{}, nil
	}
	if row.IntervalGoalTargetSeconds == nil || row.IntervalGoalRecurrence == nil {
		return domain.IntervalGoal{}, errInvalidPersistedPath
	}
	goal := domain.IntervalGoal{Present: true, TargetSeconds: *row.IntervalGoalTargetSeconds, Recurrence: domain.Recurrence(*row.IntervalGoalRecurrence)}
	switch goal.Recurrence {
	case domain.RecurrenceHourly:
		if row.IntervalGoalStartMinute == nil || row.IntervalGoalStartHour != nil || row.IntervalGoalStartWeekday != nil || row.IntervalGoalStartDay != nil || row.IntervalGoalStartMonth != nil {
			return domain.IntervalGoal{}, errInvalidPersistedPath
		}
		goal.Alignment.Minute = int(*row.IntervalGoalStartMinute)
	case domain.RecurrenceDaily:
		if row.IntervalGoalStartMinute != nil || row.IntervalGoalStartHour == nil || row.IntervalGoalStartWeekday != nil || row.IntervalGoalStartDay != nil || row.IntervalGoalStartMonth != nil {
			return domain.IntervalGoal{}, errInvalidPersistedPath
		}
		goal.Alignment.Hour = int(*row.IntervalGoalStartHour)
	case domain.RecurrenceWeekly:
		if row.IntervalGoalStartMinute != nil || row.IntervalGoalStartHour != nil || row.IntervalGoalStartWeekday == nil || row.IntervalGoalStartDay != nil || row.IntervalGoalStartMonth != nil {
			return domain.IntervalGoal{}, errInvalidPersistedPath
		}
		goal.Alignment.ISOWeekday = int(*row.IntervalGoalStartWeekday)
	case domain.RecurrenceMonthly:
		if row.IntervalGoalStartMinute != nil || row.IntervalGoalStartHour != nil || row.IntervalGoalStartWeekday != nil || row.IntervalGoalStartDay == nil || row.IntervalGoalStartMonth != nil {
			return domain.IntervalGoal{}, errInvalidPersistedPath
		}
		goal.Alignment.Day = int(*row.IntervalGoalStartDay)
	case domain.RecurrenceYearly:
		if row.IntervalGoalStartMinute != nil || row.IntervalGoalStartHour != nil || row.IntervalGoalStartWeekday != nil || row.IntervalGoalStartDay == nil || row.IntervalGoalStartMonth == nil {
			return domain.IntervalGoal{}, errInvalidPersistedPath
		}
		goal.Alignment.Day = int(*row.IntervalGoalStartDay)
		goal.Alignment.Month = int(*row.IntervalGoalStartMonth)
	default:
		return domain.IntervalGoal{}, errInvalidPersistedPath
	}
	return goal, nil
}

func pointer[T any](value T) *T {
	return &value
}

func fromAudit(event audit.Event) *auditModel {
	return &auditModel{ID: event.ID, OwnerUserID: event.OwnerUserID, ActorUserID: event.ActorUserID, Action: event.Action, TargetType: event.TargetType, TargetID: event.TargetID, Outcome: event.Outcome, CorrelationID: event.CorrelationID, OccurredAt: event.OccurredAt}
}
