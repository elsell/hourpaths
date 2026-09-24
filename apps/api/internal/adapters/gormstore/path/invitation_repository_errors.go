package pathstore

import (
	"errors"

	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
)

func classifyUnavailableInvitation(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrInvitationUnavailable
	}
	return err
}

func classifyInvitationPersistenceError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ports.ErrNotFound
	}
	return err
}
