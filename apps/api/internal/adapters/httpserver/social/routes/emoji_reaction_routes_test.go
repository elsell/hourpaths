package routes

import (
	"context"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
)

func (service *controlledService) ListPracticeReactions(_ context.Context, authorization, eventID string, reaction domain.Reaction, cursor string, limit int) ([]domain.PublicProfile, string, error) {
	service.authorization, service.eventID, service.receivedCursor, service.limit, service.reaction = authorization, eventID, cursor, limit, reaction
	return service.profiles, service.cursor, service.err
}

func (service *controlledService) AddPracticeEmojiReaction(_ context.Context, authorization, eventID, emoji, key string) (domain.ReactionSummary, error) {
	service.authorization, service.eventID, service.reactionKey, service.reaction = authorization, eventID, key, domain.Reaction(emoji)
	return service.reactionSummary, service.err
}
func (service *controlledService) RemovePracticeEmojiReaction(_ context.Context, authorization, eventID, emoji, key string) (domain.ReactionSummary, error) {
	service.authorization, service.eventID, service.reactionKey, service.reaction = authorization, eventID, key, domain.Reaction(emoji)
	return service.reactionSummary, service.err
}
