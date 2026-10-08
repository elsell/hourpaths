package activity

import "context"

type achievementRepresentationKey struct{}

// WithAchievementNotificationRepresentation carries a client capability, not authorization.
func WithAchievementNotificationRepresentation(ctx context.Context, enabled bool) context.Context {
	return context.WithValue(ctx, achievementRepresentationKey{}, enabled)
}
func achievementNotificationRepresentation(ctx context.Context) bool {
	enabled, _ := ctx.Value(achievementRepresentationKey{}).(bool)
	return enabled
}
