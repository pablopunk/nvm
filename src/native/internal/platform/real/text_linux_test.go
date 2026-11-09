//go:build linux

package real

import (
	"context"
	"errors"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestLinuxScreenCaptureIsUnsupported(t *testing.T) {
	err := newScreenCapturer(fake.NewHelper()).Capture(context.Background(), "/tmp/x.png", nil)
	if !errors.Is(err, platform.ErrUnsupported) {
		t.Fatalf("err = %v, want ErrUnsupported", err)
	}
}
