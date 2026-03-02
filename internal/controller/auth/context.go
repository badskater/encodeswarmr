package auth

import "context"

// contextKey is the unexported type used for context values set by this package.
type contextKey struct{}

// Claims holds the authenticated user's identity extracted from a validated session.
type Claims struct {
	UserID   string
	Username string
	Role     string
}

// withClaims returns a copy of ctx carrying the given claims.
func withClaims(ctx context.Context, c *Claims) context.Context {
	return context.WithValue(ctx, contextKey{}, c)
}

// FromContext retrieves the claims stored by the session middleware.
// Returns (nil, false) if the request has not been authenticated.
func FromContext(ctx context.Context) (*Claims, bool) {
	c, ok := ctx.Value(contextKey{}).(*Claims)
	return c, ok
}
