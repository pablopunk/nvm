package backend

import "context"

// pathTokensCurrent is the route that revokes the bearer token the request carries.
const pathTokensCurrent = "/api/tokens/current"

// RevokeCurrentToken revokes the bearer token this client sends. A 401 means the backend no longer knows the
// token, which is the state a revocation aims for, so it returns nil. The call is not retried: a revocation that
// reached the backend is not repeated, and one that did not is left to the caller to report.
func (c *Client) RevokeCurrentToken(ctx context.Context) error {
	err := c.callJSON(ctx, requestSpec{
		method:  "DELETE",
		path:    pathTokensCurrent,
		auth:    authRequired,
		timeout: meTimeout,
	}, nil)
	if IsKind(err, KindUnauthorized) {
		return nil
	}
	return err
}
