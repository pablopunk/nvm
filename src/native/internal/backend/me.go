package backend

import (
	"context"
	"time"
)

// meTimeout bounds the account read.
const meTimeout = 10 * time.Second

// Account is the signed-in user's plan and balance.
type Account struct {
	// Email is the account email.
	Email string `json:"email"`
	// Plan is the plan name, such as "free" or "pro".
	Plan string `json:"plan"`
	// Balance is the credit balance the backend reports.
	Balance int `json:"balance"`
}

// Me reads the account. It is an idempotent GET. The route answers the account of the caller; a backend that
// rejects the bearer token surfaces as KindUnauthorized.
func (c *Client) Me(ctx context.Context) (Account, error) {
	var account Account
	err := c.callJSON(ctx, requestSpec{
		method:     "GET",
		path:       pathMe,
		auth:       authRequired,
		timeout:    meTimeout,
		idempotent: true,
	}, &account)
	if err != nil {
		return Account{}, err
	}
	return account, nil
}
