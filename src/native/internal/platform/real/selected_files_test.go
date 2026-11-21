package real

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"reflect"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestSelectedFilesContract(t *testing.T) {
	framedPaths := []string{"/tmp/real\nnewline", "/tmp/quoted\"name", "/tmp/back\\slash", "/tmp/folder/", "/"}
	framedJSON, err := json.Marshal(selectedFilesAnswer{Status: "ok", Paths: framedPaths})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name    string
		script  fake.HelperScript
		want    []string
		wantErr error
		fails   bool
	}{
		{name: "POSIX paths across test hosts", script: fake.HelperScript{Stdout: framedJSON}, want: []string{"/tmp/real\nnewline", "/tmp/quoted\"name", "/tmp/back\\slash", "/tmp/folder", "/"}},
		{name: "finder paths preserve framing", script: fake.HelperScript{Stdout: []byte(`{"status":"ok","paths":["/tmp/a\nb","/tmp/quoted\"name"]}`)}, want: []string{"/tmp/a\nb", "/tmp/quoted\"name"}},
		{name: "empty Finder", script: fake.HelperScript{Stdout: []byte(`{"status":"ok","paths":[]}`)}, want: []string{}},
		{name: "other app", script: fake.HelperScript{Stdout: []byte(`{"status":"other"}`)}, want: []string{}},
		{name: "missing source", script: fake.HelperScript{Stdout: []byte(`{"status":"missing"}`)}, wantErr: platform.ErrNotFound},
		{name: "denied", script: fake.HelperScript{ExitCode: 1, Stderr: []byte("Not authorized (-1743)")}, wantErr: platform.ErrDenied},
		{name: "missing helper", script: fake.HelperScript{Err: exec.ErrNotFound}, wantErr: platform.ErrUnsupported},
		{name: "timeout", script: fake.HelperScript{Timeout: true}, wantErr: context.DeadlineExceeded},
		{name: "relative path", script: fake.HelperScript{Stdout: []byte(`{"status":"ok","paths":["relative"]}`)}, fails: true},
		{name: "unclean path", script: fake.HelperScript{Stdout: []byte(`{"status":"ok","paths":["/tmp/../a"]}`)}, fails: true},
		{name: "trailing JSON", script: fake.HelperScript{Stdout: []byte(`{"status":"ok","paths":[]} {}`)}, fails: true},
		{name: "truncated", script: fake.HelperScript{Stdout: make([]byte, platform.MaxHelperOutputBytes+1)}, fails: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			helper := fake.NewHelper()
			helper.Script(osascriptBinary, tc.script)
			got, err := (selectedFilesReader{helper: helper}).Paths(context.Background(), platform.FocusTarget{PID: 42})
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
			if tc.fails && err == nil {
				t.Fatal("expected error")
			}
			if tc.wantErr == nil && !tc.fails && (err != nil || !reflect.DeepEqual(got, tc.want)) {
				t.Fatalf("Paths = %q, %v; want %q", got, err, tc.want)
			}
			calls := helper.Calls()
			if len(calls) != 1 || calls[0].Binary != osascriptBinary || calls[0].Args[len(calls[0].Args)-1] != "42" {
				t.Fatalf("helper calls = %#v", calls)
			}
		})
	}
}

func TestSelectedFilesRejectsInvalidTargetAndCancellation(t *testing.T) {
	helper := fake.NewHelper()
	reader := selectedFilesReader{helper: helper}
	if _, err := reader.Paths(context.Background(), platform.FocusTarget{}); !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("invalid target: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := reader.Paths(ctx, platform.FocusTarget{PID: 42}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled: %v", err)
	}
	if len(helper.Calls()) != 0 {
		t.Fatal("helper ran for invalid or canceled request")
	}
}

func TestSelectedFilesNilHelperIsUnsupported(t *testing.T) {
	_, err := (selectedFilesReader{}).Paths(context.Background(), platform.FocusTarget{PID: 1})
	if !errors.Is(err, platform.ErrUnsupported) {
		t.Fatalf("nil helper: %v", err)
	}
}
