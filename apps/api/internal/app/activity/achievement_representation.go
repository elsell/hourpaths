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

type longTimerRepresentationKey struct{}

func WithLongTimerNotificationRepresentation(ctx context.Context, enabled bool) context.Context {
	return context.WithValue(ctx, longTimerRepresentationKey{}, enabled)
}
func longTimerNotificationRepresentation(ctx context.Context) bool {
	enabled, _ := ctx.Value(longTimerRepresentationKey{}).(bool)
	return enabled
}

type goalDeadlineRepresentationKey struct{}

func WithGoalDeadlineNotificationRepresentation(ctx context.Context, enabled bool) context.Context {
	return context.WithValue(ctx, goalDeadlineRepresentationKey{}, enabled)
}
func goalDeadlineNotificationRepresentation(ctx context.Context) bool {
	enabled, _ := ctx.Value(goalDeadlineRepresentationKey{}).(bool)
	return enabled
}
