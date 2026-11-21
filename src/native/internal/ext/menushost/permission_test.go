package menushost

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform"
)

type deniedSource struct{ phase string }

func (s deniedSource) Capture(context.Context, int) (platform.MenuSource, error) {
	if s.phase == "capture" {
		return platform.MenuSource{}, &platform.WindowError{Code: platform.WindowErrorDenied}
	}
	return platform.MenuSource{PID: 42, Identity: "identity"}, nil
}

func (s deniedSource) Snapshot(context.Context, platform.MenuSource) ([]platform.MenuEntry, error) {
	if s.phase == "read" {
		return nil, &platform.WindowError{Code: platform.WindowErrorDenied}
	}
	return []platform.MenuEntry{{Reference: "open", Enabled: true}}, nil
}

func (s deniedSource) Invoke(context.Context, platform.MenuSource, string) error {
	if s.phase == "invoke" {
		return &platform.WindowError{Code: platform.WindowErrorDenied}
	}
	return nil
}

func TestNativePermissionErrorsRetainAPIClassification(t *testing.T) {
	for _, phase := range []string{"capture", "read", "invoke"} {
		t.Run(phase, func(t *testing.T) {
			changes := make(chan struct{}, 4)
			a := New(context.Background(), deniedSource{phase}, time.Minute, func() { changes <- struct{}{} })
			defer a.Close()
			a.Capture(42)
			<-changes
			<-changes
			snapshot, err := a.Snapshot(context.Background())
			if phase == "invoke" {
				if err != nil || len(snapshot.Items) != 1 {
					t.Fatalf("snapshot=%+v err=%v", snapshot, err)
				}
				err = a.Invoke(context.Background(), snapshot.Items[0].Handle)
			}
			if !errors.Is(err, api.ErrPermission) {
				t.Fatalf("native %s error lost API permission: %v", phase, err)
			}
		})
	}
}
