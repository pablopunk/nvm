package swifthelper

import (
	"errors"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
)

func TestOkEnvelopeDecodesItsDisplays(t *testing.T) {
	envelope, err := ParseEnvelope([]byte(`{"ok":true,"code":"ok","data":[{"id":"1","isPrimary":true,"frame":{"x":0,"y":0,"width":1440,"height":900},"workArea":{"x":0,"y":25,"width":1440,"height":875}}]}`))
	if err != nil || envelope.Err() != nil {
		t.Fatalf("ParseEnvelope = %+v, %v", envelope, err)
	}
	displays, err := Decode[[]Display](envelope)
	if err != nil || len(displays) != 1 || displays[0].WorkArea.Y != 25 || !displays[0].IsPrimary {
		t.Fatalf("Decode = %+v, %v", displays, err)
	}
}

func TestFailedEnvelopesMapToPlatformCodes(t *testing.T) {
	cases := map[string]platform.WindowErrorCode{
		`{"ok":false,"code":"denied","message":"no access"}`:     platform.WindowErrorDenied,
		`{"ok":false,"code":"no-window"}`:                        platform.WindowErrorNoWindow,
		`{"ok":false,"code":"target-lost"}`:                      platform.WindowErrorTargetLost,
		`{"ok":false,"code":"unsupported"}`:                      platform.WindowErrorUnsupported,
		`{"ok":false,"code":"unapplied","message":"no change"}`:  platform.WindowErrorUnapplied,
		`{"ok":false,"code":"error","message":"helper crashed"}`: platform.WindowErrorFailed,
	}
	for raw, want := range cases {
		envelope, err := ParseEnvelope([]byte(raw))
		if err != nil {
			t.Fatalf("ParseEnvelope(%s): %v", raw, err)
		}
		var windowErr *platform.WindowError
		if !errors.As(envelope.Err(), &windowErr) || windowErr.Code != want {
			t.Fatalf("Err(%s) = %v, want code %q", raw, envelope.Err(), want)
		}
	}
}

func TestUnreadableAnswersAreRejected(t *testing.T) {
	for _, raw := range []string{"", "not json", `{"ok":false,"code":"exploded"}`} {
		if _, err := ParseEnvelope([]byte(raw)); !errors.Is(err, ErrUnreadable) {
			t.Fatalf("ParseEnvelope(%q) error = %v, want ErrUnreadable", raw, err)
		}
	}
	envelope, _ := ParseEnvelope([]byte(`{"ok":true,"code":"ok"}`))
	if _, err := Decode[[]Display](envelope); !errors.Is(err, ErrUnreadable) {
		t.Fatalf("Decode without data error = %v, want ErrUnreadable", err)
	}
}

func TestListedWindowFlattensWindowFieldsAndKeepsStackOrder(t *testing.T) {
	envelope, _ := ParseEnvelope([]byte(`{"ok":true,"code":"ok","data":[{"bundleId":"com.apple.Notes","windowKey":"77:3","title":"Todo","frame":{"x":10,"y":20,"width":300,"height":200},"displayId":"main","isFullScreen":false,"isMinimized":false,"isMovable":true,"isResizable":true,"pid":77,"appName":"Notes","zIndex":2}]}`))
	listed, err := Decode[[]ListedWindow](envelope)
	if err != nil || len(listed) != 1 {
		t.Fatalf("Decode = %+v, %v", listed, err)
	}
	record := listed[0].PlatformWindow(listed[0].PID, listed[0].AppName, listed[0].ZIndex)
	if record.Key != "77:3" || record.PID != 77 || record.AppName != "Notes" || record.ZIndex != 2 || record.Title != "Todo" || !record.Resizable {
		t.Fatalf("record = %+v", record)
	}
}
