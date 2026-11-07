package app

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/png"
	"testing"

	"github.com/pablopunk/nvm/native/internal/features/files/thumbs"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestImageReferencesKeepGridAndPreviewSizesSeparate(t *testing.T) {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewNRGBA(image.Rect(0, 0, 512, 256))); err != nil {
		t.Fatal(err)
	}
	fsys := fake.NewFS()
	fsys.Seed("/image #1.png", encoded.Bytes())
	for _, tc := range []struct {
		ref   string
		width int
	}{
		{thumbs.GridReference("file:///image%20%231.png"), thumbs.GridEdge},
		{"file:///image%20%231.png", 512},
		{"/image #1.png", 512},
	} {
		data, ok := imageBytes(fsys, tc.ref)
		if !ok {
			t.Fatalf("image rejected: %s", tc.ref)
		}
		config, err := png.DecodeConfig(bytes.NewReader(data))
		if err != nil || config.Width != tc.width || config.Height != tc.width/2 {
			t.Fatalf("%s rendered %+v, %v; want %dx%d", tc.ref, config, err, tc.width, tc.width/2)
		}
	}
}

func TestImageDataURLRejectsOversizedPixelHeaderBeforeDecode(t *testing.T) {
	header := make([]byte, 33)
	copy(header, "\x89PNG\r\n\x1a\n")
	binary.BigEndian.PutUint32(header[8:12], 13)
	copy(header[12:16], "IHDR")
	binary.BigEndian.PutUint32(header[16:20], 100_000)
	binary.BigEndian.PutUint32(header[20:24], 100_000)
	header[24], header[25] = 8, 6
	binary.BigEndian.PutUint32(header[29:33], crc32.ChecksumIEEE(header[12:29]))
	ref := "data:image/png;base64," + base64.StdEncoding.EncodeToString(header)
	if _, ok := imageBytes(nil, ref); ok {
		t.Fatal("oversized image header accepted")
	}
}

func TestImageBytesRendersLargeLocalImage(t *testing.T) {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewNRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	data := append(encoded.Bytes(), make([]byte, 9<<20)...)
	fsys := fake.NewFS()
	fsys.Seed("/large image.png", data)
	result, ok := imageBytes(fsys, "file:///large%20image.png")
	if !ok {
		t.Fatal("large local image was rejected")
	}
	config, err := png.DecodeConfig(bytes.NewReader(result))
	if err != nil || config.Width > thumbs.PreviewEdge || config.Height > thumbs.PreviewEdge {
		t.Fatalf("invalid rendered image: %+v, %v", config, err)
	}
}

type blockingImageFS struct {
	platform.FS
	entered chan struct{}
	release chan struct{}
}

func (f *blockingImageFS) ReadFileNoFollow(path string, limit int64) ([]byte, error) {
	f.entered <- struct{}{}
	<-f.release
	return f.FS.(interface {
		ReadFileNoFollow(string, int64) ([]byte, error)
	}).ReadFileNoFollow(path, limit)
}

func TestExtensionImageLoadsUseTwoPermits(t *testing.T) {
	fsys := fake.NewFS()
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewNRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	fsys.Seed("/test.png", encoded.Bytes())
	blocked := &blockingImageFS{FS: fsys, entered: make(chan struct{}, 3), release: make(chan struct{}, 3)}
	done := make(chan struct{}, 3)
	for range 3 {
		go func() { loadExtensionBitmap(blocked, "/test.png"); done <- struct{}{} }()
	}
	<-blocked.entered
	<-blocked.entered
	select {
	case <-blocked.entered:
		t.Fatal("third load entered before a permit was released")
	default:
	}
	blocked.release <- struct{}{}
	<-blocked.entered
	blocked.release <- struct{}{}
	blocked.release <- struct{}{}
	for range 3 {
		<-done
	}
}
