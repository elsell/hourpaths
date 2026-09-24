package gormstore

import (
	"context"
	"errors"
	"strconv"

	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
)

const usernameSuggestionBatchSize = 256
const asciiUsernameUppercase = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
const asciiUsernameLowercase = "abcdefghijklmnopqrstuvwxyz"

type usernameReservations func(context.Context, []string) (map[string]struct{}, error)

func (s *Store) SuggestAvailableUsername(ctx context.Context, displayName string) (string, error) {
	base := identity.UsernameSuggestion(displayName)
	if base == "" {
		return "", nil
	}
	if s == nil || s.DB == nil {
		return "", errors.New("username suggestion store is unavailable")
	}

	return suggestAvailableUsername(ctx, base, s.reservedUsernames)
}

func (s *Store) reservedUsernames(ctx context.Context, candidates []string) (map[string]struct{}, error) {
	var rows []struct {
		Username string `gorm:"column:normalized_username"`
	}
	result := s.DB.WithContext(ctx).Model(&userModel{}).
		Select("translate(username, ?, ?) AS normalized_username", asciiUsernameUppercase, asciiUsernameLowercase).
		Where("translate(username, ?, ?) IN ?", asciiUsernameUppercase, asciiUsernameLowercase, candidates).
		Scan(&rows)
	if result.Error != nil {
		return nil, result.Error
	}
	reserved := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		reserved[row.Username] = struct{}{}
	}
	return reserved, nil
}

func suggestAvailableUsername(ctx context.Context, base string, reservations usernameReservations) (string, error) {
	for firstOrdinal := 1; ; firstOrdinal += usernameSuggestionBatchSize {
		candidates := usernameCandidates(base, firstOrdinal, usernameSuggestionBatchSize)
		reserved, err := reservations(ctx, candidates)
		if err != nil {
			return "", err
		}
		for _, candidate := range candidates {
			if _, exists := reserved[candidate]; !exists {
				return candidate, nil
			}
		}
	}
}

func usernameCandidates(base string, firstOrdinal, count int) []string {
	candidates := make([]string, count)
	for index := range candidates {
		ordinal := firstOrdinal + index
		if ordinal == 1 {
			candidates[index] = base
			continue
		}
		suffix := "." + strconv.Itoa(ordinal)
		candidates[index] = base[:min(len(base), 64-len(suffix))] + suffix
	}
	return candidates
}
