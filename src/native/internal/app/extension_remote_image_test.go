package app

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/png"
	"net/http"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestRemoteImageFetch(t *testing.T) {
	var imageData bytes.Buffer
	if err := png.Encode(&imageData, image.NewNRGBA(image.Rect(0, 0, 16, 9))); err != nil {
		t.Fatal(err)
	}
	client := fake.NewHTTP()
	client.Script(fake.Response{Status: 200, Body: imageData.Bytes()})
	data, ok := remoteImageBytes(client, "https://images.example/test.png")
	if !ok || !bytes.Equal(data, imageData.Bytes()) {
		t.Fatal("valid remote image rejected")
	}
	client.Script(fake.Response{Status: 200, Body: []byte("not an image")})
	if _, ok := remoteImageBytes(client, "https://images.example/invalid"); ok {
		t.Fatal("invalid image accepted")
	}
}

func TestRemoteImageRejectsRedirectAndLimits(t *testing.T) {
	client := fake.NewHTTP()
	client.Script(fake.Response{Status: 302, Header: http.Header{"Location": []string{"file:///etc/passwd"}}})
	if _, ok := remoteImageBytes(client, "https://images.example/start"); ok {
		t.Fatal("file redirect accepted")
	}
	if len(client.Requests()) != 1 {
		t.Fatal("invalid redirect was contacted")
	}
	client.Script(fake.Response{Status: 200, Body: make([]byte, maxImageDataURLBytes+1)})
	if _, ok := remoteImageBytes(client, "https://images.example/large"); ok {
		t.Fatal("oversized response accepted")
	}
	header := make([]byte, 33)
	copy(header, "\x89PNG\r\n\x1a\n")
	binary.BigEndian.PutUint32(header[8:12], 13)
	copy(header[12:16], "IHDR")
	binary.BigEndian.PutUint32(header[16:20], 100000)
	binary.BigEndian.PutUint32(header[20:24], 100000)
	header[24], header[25] = 8, 6
	binary.BigEndian.PutUint32(header[29:33], crc32.ChecksumIEEE(header[12:29]))
	client.Script(fake.Response{Status: 200, Body: header})
	if _, ok := remoteImageBytes(client, "https://images.example/pixels"); ok {
		t.Fatal("oversized dimensions accepted")
	}
}

func TestRemoteImageCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := fake.NewHTTP()
	if _, ok := remoteImageBytesContext(ctx, client, "https://images.example/test"); ok {
		t.Fatal("cancelled fetch succeeded")
	}
}

type waitingImageHTTP struct{ started chan struct{} }

func (h waitingImageHTTP) Do(req *http.Request) (*http.Response, error) {
	close(h.started)
	<-req.Context().Done()
	return nil, req.Context().Err()
}

func TestRemoteImageCancellationInterruptsRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := waitingImageHTTP{started: make(chan struct{})}
	done := make(chan bool, 1)
	go func() { _, ok := remoteImageBytesContext(ctx, client, "https://images.example/test"); done <- ok }()
	<-client.started
	cancel()
	if <-done {
		t.Fatal("cancelled request succeeded")
	}
}

type deadlineImageHTTP struct {
	deadline time.Time
	present  bool
}

func (h *deadlineImageHTTP) Do(req *http.Request) (*http.Response, error) {
	h.deadline, h.present = req.Context().Deadline()
	return nil, errors.New("fixture transport")
}
func TestRemoteImageRequestHasBoundedDeadline(t *testing.T) {
	client := &deadlineImageHTTP{}
	before := time.Now()
	remoteImageBytes(client, "https://images.example/test")
	if !client.present || client.deadline.Before(before) || client.deadline.After(before.Add(11*time.Second)) {
		t.Fatalf("deadline %v present %v", client.deadline, client.present)
	}
}

func TestRemoteImageAcceptsUppercaseScheme(t *testing.T) {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewNRGBA(image.Rect(0, 0, 16, 9))); err != nil {
		t.Fatal(err)
	}
	client := fake.NewHTTP()
	client.Script(fake.Response{Status: 200, Body: encoded.Bytes()})
	if loadExtensionBitmapContext(context.Background(), nil, "HTTPS://images.example/test.png", client) == nil {
		t.Fatal("uppercase HTTPS image rejected")
	}
	if len(client.Requests()) != 1 {
		t.Fatal("image was not fetched through public transport")
	}
}
