package context

import "context"

type contextKey string

const (
	userIDKey contextKey = "user_id"
)

func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

func GetUserID(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(userIDKey).(string)
	return id, ok
}
