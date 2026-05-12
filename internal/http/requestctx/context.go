package requestctx

import "context"

type userIDKey struct {
}

func WithUserID(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, userIDKey{}, userID)
}

func UserID(ctx context.Context) (int64, bool) {
	value := ctx.Value(userIDKey{})
	if value == nil {
		return 0, false
	}
	userID, ok := value.(int64)
	return userID, ok
}
