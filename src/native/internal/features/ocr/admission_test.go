package ocr

import (
	"context"
	"errors"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/platform/testpath"
	"io/fs"
	"testing"
)

func TestOCRAvailabilityUsesActualPortsWithoutProbingReadiness(t *testing.T) {
	caps := platform.NewCapabilities(platform.CapabilityTextRecognize, platform.CapabilityScreenCapture)
	var absent *Service
	if got := absent.Availability(caps); got.Image || got.Screen {
		t.Fatalf("nil service: %+v", got)
	}
	text := fake.NewTextRecognizer()
	capture := fake.NewScreenCapturer()
	files := fake.NewFS()
	for _, c := range []struct {
		deps          Deps
		image, screen bool
	}{
		{Deps{}, false, false}, {Deps{Text: text}, true, false}, {Deps{Text: text, Capture: capture}, true, false}, {Deps{Text: text, Capture: capture, FS: files}, true, true},
	} {
		got := New(c.deps).Availability(caps)
		if got.Image != c.image || got.Screen != c.screen {
			t.Fatalf("support=%+v want %v %v", got, c.image, c.screen)
		}
	}
	if got := New(Deps{Text: text, Capture: capture, FS: files}).Availability(platform.NewCapabilities(platform.CapabilityTextRecognize)); !got.Image || got.Screen {
		t.Fatalf("capability intersection: %+v", got)
	}
	if len(text.Calls()) != 0 || len(capture.Captures()) != 0 {
		t.Fatal("support probe performed native work")
	}
}

func TestOCRAdmissionRefusesCancelledAndMissingPorts(t *testing.T) {
	svc, text, capture, _ := deskWith(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := svc.Image(ctx, testpath.Abs("photos", "receipt.png"), Options{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := svc.ImagePNG(ctx, []byte("png"), Options{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := svc.Screen(ctx, Options{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if len(text.Calls()) != 0 || len(capture.Captures()) != 0 {
		t.Fatal("cancelled call reached native effect")
	}
	empty := New(Deps{})
	if _, err := empty.Image(context.Background(), testpath.Abs("photos", "receipt.png"), Options{}); !errors.Is(err, platform.ErrUnsupported) {
		t.Fatal(err)
	}
	if _, err := empty.ImagePNG(context.Background(), nil, Options{}); !errors.Is(err, platform.ErrUnsupported) {
		t.Fatal(err)
	}
	if _, err := empty.Screen(context.Background(), Options{}); !errors.Is(err, platform.ErrUnsupported) {
		t.Fatal(err)
	}
}

type cancellingCapture struct{ cancel context.CancelFunc }

func (c cancellingCapture) Capture(context.Context, string, *platform.Rect) error {
	c.cancel()
	return nil
}

type cancellingOCRWrite struct {
	platform.FS
	cancel context.CancelFunc
	path   string
}

func (f *cancellingOCRWrite) WriteFile(path string, data []byte, mode fs.FileMode) error {
	f.path = path
	err := f.FS.WriteFile(path, data, mode)
	f.cancel()
	return err
}

func TestOCRAdmissionRechecksAfterCaptureOrImageStaging(t *testing.T) {
	svc, text, _, files := deskWith(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.deps.Capture = cancellingCapture{cancel: cancel}
	if _, err := svc.Screen(ctx, Options{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if len(text.Calls()) != 0 {
		t.Fatal("recognition followed cancelled capture")
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	writer := &cancellingOCRWrite{FS: files, cancel: cancel}
	svc.deps.FS = writer
	if _, err := svc.ImagePNG(ctx, []byte("png"), Options{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if len(text.Calls()) != 0 {
		t.Fatal("recognition followed cancelled image staging")
	}
	if _, err := files.Stat(writer.path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("staged image not removed: %v", err)
	}
}

type cancellingOCRText struct{ cancel context.CancelFunc }

func (r cancellingOCRText) Recognize(context.Context, string, []string) (platform.TextResult, error) {
	r.cancel()
	return platform.TextResult{Text: "late success"}, nil
}

func TestOCRAdmissionSuppressesSuccessAfterCallerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc := New(Deps{Text: cancellingOCRText{cancel: cancel}})
	result, err := svc.Image(ctx, testpath.Abs("photos", "receipt.png"), Options{})
	if !errors.Is(err, context.Canceled) || result.Text != "" {
		t.Fatalf("late result=%+v err=%v", result, err)
	}
}

type partialOCRWrite struct {
	platform.FS
	path     string
	writeErr error
}

func (f *partialOCRWrite) WriteFile(path string, data []byte, mode fs.FileMode) error {
	f.path = path
	if err := f.FS.WriteFile(path, data[:1], mode); err != nil {
		return err
	}
	return f.writeErr
}

func TestOCRAdmissionRemovesPartialImageAfterFailedStaging(t *testing.T) {
	svc, text, _, files := deskWith(t)
	writeErr := errors.New("disk full after partial write")
	writer := &partialOCRWrite{FS: files, writeErr: writeErr}
	svc.deps.FS = writer
	result, err := svc.ImagePNG(context.Background(), []byte("private image"), Options{})
	if !errors.Is(err, writeErr) || result.Text != "" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if writer.path == "" {
		t.Fatal("partial-write boundary was not exercised")
	}
	if _, err := files.Stat(writer.path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("partial image remains: %v", err)
	}
	if len(text.Calls()) != 0 {
		t.Fatal("recognizer read a partially staged image")
	}
}
