package backend

import (
	"context"
	"time"
)

// DeviceEvent is a progress report from SignIn. Pending carries the SignIn so the UI can show the code; SlowDown
// carries the new interval.
type DeviceEvent struct {
	// Phase is the state reached.
	Phase DevicePhase
	// SignIn is set on the first DevicePending event.
	SignIn DeviceSignIn
	// Interval is the poll interval in force, set on DevicePending and DeviceSlowDown.
	Interval time.Duration
}

// SignIn runs a complete device sign-in: it starts the flow, calls open with the verification URL, and polls
// on the clock until the code is approved, denied, expired or ctx ends. Transient network failures do not end
// the flow; they are retried on the next poll until the code expires. open must return promptly; the caller
// opens the browser there. notify may be nil.
//
// On approval it returns the token and account for the caller to store in the keychain. Denied and expired
// flows return ErrSignInDenied and ErrSignInExpired.
func (c *Client) SignIn(ctx context.Context, open func(verificationURL string), notify func(DeviceEvent)) (Approval, error) {
	sign, err := c.StartDeviceSignIn(ctx)
	if err != nil {
		return Approval{}, err
	}
	emitDeviceEvent(notify, DeviceEvent{Phase: DevicePending, SignIn: sign, Interval: sign.Interval})
	if open != nil {
		open(sign.VerificationURL)
	}
	interval := sign.Interval
	for {
		if err := c.pause(ctx, interval); err != nil {
			return Approval{}, err
		}
		if !c.clock.Now().Before(sign.ExpiresAt) {
			emitDeviceEvent(notify, DeviceEvent{Phase: DeviceExpired})
			return Approval{}, ErrSignInExpired
		}
		poll, err := c.PollDeviceSignIn(ctx, sign.Code)
		if err != nil {
			if isNetworkKind(kindOf(err)) {
				continue
			}
			return Approval{}, err
		}
		switch poll.Phase {
		case DeviceSlowDown:
			interval += slowDownStep
			emitDeviceEvent(notify, DeviceEvent{Phase: DeviceSlowDown, Interval: interval})
		case DeviceApproved:
			emitDeviceEvent(notify, DeviceEvent{Phase: DeviceApproved})
			return Approval{Token: poll.Token, User: poll.User}, nil
		case DeviceDenied:
			emitDeviceEvent(notify, DeviceEvent{Phase: DeviceDenied})
			return Approval{}, ErrSignInDenied
		case DeviceExpired:
			emitDeviceEvent(notify, DeviceEvent{Phase: DeviceExpired})
			return Approval{}, ErrSignInExpired
		}
	}
}

// emitDeviceEvent delivers an event when the caller asked for progress reports.
func emitDeviceEvent(notify func(DeviceEvent), event DeviceEvent) {
	if notify != nil {
		notify(event)
	}
}
