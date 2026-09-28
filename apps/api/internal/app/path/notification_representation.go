package path

import "context"

type notificationEmojiRepresentationKey struct{}

// WithNotificationEmojiRepresentation negotiates response vocabulary only. It
// does not grant access; normal recipient, visibility, and audit checks apply.
func WithNotificationEmojiRepresentation(ctx context.Context, enabled bool) context.Context {
	return context.WithValue(ctx, notificationEmojiRepresentationKey{}, enabled)
}
func notificationEmojiRepresentation(ctx context.Context) bool {
	enabled, _ := ctx.Value(notificationEmojiRepresentationKey{}).(bool)
	return enabled
}
