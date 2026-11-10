package real

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/platform/onnxocr"
)

func TestOCRRunsEachReadInAChildOfTheApp(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	helper := fake.NewHelper()
	helper.Script(binary,
		fake.HelperScript{Stdout: []byte(`{"text":"niño","blocks":[]}`)},
		fake.HelperScript{Stdout: []byte(`{"text":"niño","blocks":[]}`)},
	)
	recognizer := onnxText{helper: helper}
	assets := onnxocr.Assets{Library: "/runtime", Detector: "/det", Recognizer: "/rec", Dictionary: "/dict"}
	for range 2 {
		result, err := recognizer.read(context.Background(), assets, "/photos/a file.png")
		if err != nil || result.Text != "niño" {
			t.Fatalf("result=%+v err=%v", result, err)
		}
	}
	calls := helper.Calls()
	if len(calls) != 2 {
		t.Fatalf("child runs=%d, want one per read", len(calls))
	}
	for _, call := range calls {
		if call.Binary != binary || !reflect.DeepEqual(call.Args, assets.Args("/photos/a file.png")) {
			t.Fatalf("child invocation=%+v", call)
		}
	}
}

func TestOCRReturnsChildFailureAndCancellation(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		script fake.HelperScript
		want   error
	}{
		{name: "cancel", script: fake.HelperScript{Timeout: true}, want: context.DeadlineExceeded},
		{name: "invalid output", script: fake.HelperScript{Stdout: []byte("invalid")}, want: errUnreadableTextOutput},
		{name: "child failure", script: fake.HelperScript{ExitCode: 1, Stderr: []byte("model unavailable")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			helper := fake.NewHelper()
			helper.Script(binary, tc.script)
			_, err := (onnxText{helper: helper}).read(context.Background(), onnxocr.Assets{}, "/a.png")
			if err == nil || (tc.want != nil && !errors.Is(err, tc.want)) {
				t.Fatalf("error=%v, want %v", err, tc.want)
			}
			if tc.want == nil && err.Error() != "model unavailable" {
				t.Fatal(err)
			}
		})
	}
}

var _ platform.TextRecognizer = onnxText{}
