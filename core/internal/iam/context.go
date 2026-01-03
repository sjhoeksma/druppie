package iam

import "context"

// GetUserFromContext retrieves the user from the context if present
func GetUserFromContext(ctx context.Context) (*User, bool) {
	u, ok := ctx.Value(userContextKey).(*User)
	return u, ok
}

// ContextWithUser returns a context with the user set
func ContextWithUser(ctx context.Context, u *User) context.Context {
	return context.WithValue(ctx, userContextKey, u)
}
