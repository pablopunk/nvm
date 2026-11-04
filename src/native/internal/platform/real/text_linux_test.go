//go:build linux

package real

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

const tesseractTSV = "level\tpage_num\tblock_num\tpar_num\tline_num\tword_num\tleft\ttop\twidth\theight\tconf\ttext\n" +
	"5\t1\t1\t1\t1\t1\t10\t20\t40\t15\t90\tInvoice\n" +
	"5\t1\t1\t1\t1\t2\t55\t20\t30\t15\t80\t42\n"

func TestTesseractRunsTheStdoutTSVWithTheRequestedLanguages(t *testing.T) {
	helper := fake.NewHelper()
	helper.Script(tesseractBinary, fake.HelperScript{Stdout: []byte(tesseractTSV)})
	got, err := newTextRecognizer(helper).Recognize(context.Background(), "/photos/invoice.png", []string{"eng", "fra"})
	if err != nil {
		t.Fatal(err)
	}
	calls := helper.Calls()
	want := []string{"/photos/invoice.png", "stdout", "-l", "eng+fra", "tsv"}
	if len(calls) != 1 || calls[0].Binary != tesseractBinary || !reflect.DeepEqual(calls[0].Args, want) {
		t.Fatalf("calls = %+v, want one run with %v", calls, want)
	}
	if got.Text != "Invoice 42" || got.Language != "eng" {
		t.Fatalf("result = %+v", got)
	}
}

func TestTesseractWithoutLanguagesLetsTheEngineChoose(t *testing.T) {
	helper := fake.NewHelper()
	helper.Script(tesseractBinary, fake.HelperScript{Stdout: []byte(tesseractTSV)})
	if _, err := newTextRecognizer(helper).Recognize(context.Background(), "/a.png", nil); err != nil {
		t.Fatal(err)
	}
	if args := helper.Calls()[0].Args; !reflect.DeepEqual(args, []string{"/a.png", "stdout", "tsv"}) {
		t.Fatalf("args = %v", args)
	}
}

func TestTesseractRefusesALanguageThatCouldBeAnOption(t *testing.T) {
	helper := fake.NewHelper()
	if _, err := newTextRecognizer(helper).Recognize(context.Background(), "/a.png", []string{"--config"}); err == nil {
		t.Fatal("a language that looks like an option ran the engine")
	}
	if len(helper.Calls()) != 0 {
		t.Fatal("the engine ran for a refused language")
	}
}

func TestTesseractFailureReportsTheEnginesMessage(t *testing.T) {
	helper := fake.NewHelper()
	helper.Script(tesseractBinary, fake.HelperScript{ExitCode: 1, Stderr: []byte("Error opening data file\n")})
	_, err := newTextRecognizer(helper).Recognize(context.Background(), "/a.png", nil)
	if err == nil || err.Error() != "Error opening data file" {
		t.Fatalf("err = %v, want the engine's message", err)
	}
}

func TestTesseractTimeoutIsReturnedAsTheContextError(t *testing.T) {
	helper := fake.NewHelper()
	helper.Script(tesseractBinary, fake.HelperScript{Timeout: true})
	_, err := newTextRecognizer(helper).Recognize(context.Background(), "/a.png", nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want the deadline", err)
	}
}

func TestLinuxScreenCaptureIsUnsupported(t *testing.T) {
	err := newScreenCapturer(fake.NewHelper()).Capture(context.Background(), "/tmp/x.png", nil)
	if !errors.Is(err, platform.ErrUnsupported) {
		t.Fatalf("err = %v, want ErrUnsupported", err)
	}
}
