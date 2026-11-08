package services

import (
	"context"
	"errors"
)

type actionAdmissionKey struct{}

// WithActionAdmission carries a host-owned revocation check to an effect boundary.
func WithActionAdmission(ctx context.Context, valid func() bool) context.Context {
	return context.WithValue(ctx, actionAdmissionKey{}, valid)
}

func admitAction(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if valid, ok := ctx.Value(actionAdmissionKey{}).(func() bool); ok && !valid() {
		return errors.New("this extension action has expired; reopen its view")
	}
	return nil
}
