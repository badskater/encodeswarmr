package auth

import "context"

// contextKey is the unexported type used for context values set by this package.
type contextKey struct{}

// rateLimitContextKey carries the per-API-key rate limit (req/min) for the
// current request. 0 means "use the global default".
type rateLimitContextKey struct{}

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

// WithAPIKeyRateLimit stores the per-key rate limit in the context.
func WithAPIKeyRateLimit(ctx context.Context, rateLimit int) context.Context {
	return context.WithValue(ctx, rateLimitContextKey{}, rateLimit)
}

// APIKeyRateLimitFromContext retrieves the per-key rate limit stored by the
// auth middleware. Returns (0, false) when not set.
func APIKeyRateLimitFromContext(ctx context.Context) (int, bool) {
	v, ok := ctx.Value(rateLimitContextKey{}).(int)
	return v, ok && v > 0
}

// FromContext retrieves the claims stored by the session middleware.
// Returns (nil, false) if the request has not been authenticated.
func FromContext(ctx context.Context) (*Claims, bool) {
	c, ok := ctx.Value(contextKey{}).(*Claims)
	return c, ok
}
