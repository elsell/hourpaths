package social

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

var ErrInvalidProfileQuery = errors.New("profile query must contain at least two non-whitespace characters")

type RelationshipState string

const (
	RelationshipSelf      RelationshipState = "self"
	RelationshipNone      RelationshipState = "none"
	RelationshipRequested RelationshipState = "requested"
	RelationshipFollowing RelationshipState = "following"
)

func (state RelationshipState) Valid() bool {
	return state == RelationshipSelf || state == RelationshipNone ||
		state == RelationshipRequested || state == RelationshipFollowing
}

type PublicProfile struct {
	ID, Username, DisplayName string
	ProfilePictureURL         string
	Description               string
	FollowerCount             int64
	FollowingCount            int64
	Relationship              RelationshipState
}

func (profile PublicProfile) Valid() bool {
	_, usernameErr := NormalizeUsername(profile.Username)
	nameLength := utf8.RuneCountInString(strings.TrimSpace(profile.DisplayName))
	descriptionLength := utf8.RuneCountInString(profile.Description)
	return profile.ID != "" && usernameErr == nil && nameLength >= 1 && nameLength <= 100 &&
		utf8.ValidString(profile.Username) && utf8.ValidString(profile.DisplayName) &&
		utf8.ValidString(profile.ProfilePictureURL) && utf8.ValidString(profile.Description) &&
		descriptionLength <= 500 && (profile.Description == "" || strings.TrimSpace(profile.Description) != "") &&
		profile.FollowerCount >= 0 && profile.FollowingCount >= 0 && profile.Relationship.Valid()
}

type FollowRequest struct {
	ID, RecipientUserID string
	Requester           PublicProfile
	CreatedAt           time.Time
}

func (request FollowRequest) Valid() bool {
	return request.ID != "" && request.RecipientUserID != "" && request.Requester.Valid() &&
		request.Requester.ID != request.RecipientUserID && request.Requester.Relationship != RelationshipSelf &&
		!request.CreatedAt.IsZero() && request.CreatedAt.Location() == time.UTC
}

func NormalizeProfileQuery(value string) (string, error) {
	value = strings.ToLower(norm.NFC.String(strings.TrimSpace(value)))
	if utf8.RuneCountInString(value) < 2 {
		return "", ErrInvalidProfileQuery
	}
	return value, nil
}

func NormalizeUsername(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) < 3 || len(value) > 64 || !utf8.ValidString(value) {
		return "", ErrInvalidProfileQuery
	}
	for index := range len(value) {
		character := value[index]
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '_' && character != '.' {
			return "", ErrInvalidProfileQuery
		}
	}
	return value, nil
}
