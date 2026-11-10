package services

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/platform/real"
)

type downloadTestFiles struct {
	platform.FS
	beforeAdmit func()
}

func (f downloadTestFiles) WriteStream(ctx context.Context, path string, source io.Reader, max int64, overwrite bool, admit func() error) (platform.StreamFileResult, error) {
	data, err := io.ReadAll(io.LimitReader(source, max+1))
	if err != nil {
		return platform.StreamFileResult{}, err
	}
	if int64(len(data)) > max {
		return platform.StreamFileResult{}, platform.ErrResponseTooLarge
	}
	if f.beforeAdmit != nil {
		f.beforeAdmit()
	}
	if err := ctx.Err(); err != nil {
		return platform.StreamFileResult{}, err
	}
	if err := admit(); err != nil {
		return platform.StreamFileResult{}, err
	}
	if !overwrite {
		if err := f.FS.CreateExclusive(path, data, 0600); err != nil {
			return platform.StreamFileResult{}, err
		}
	} else if err := f.FS.WriteFile(path, data, 0600); err != nil {
		return platform.StreamFileResult{}, err
	}
	return platform.StreamFileResult{Bytes: int64(len(data)), Published: true}, nil
}

func TestDownloadStreamsAndEnforcesDeclarations(t *testing.T) {
	port := fake.NewHTTP()
	data := append([]byte{0x89, 'P', 'N', 'G', 13, 10, 26, 10}, bytes.Repeat([]byte{0}, 64)...)
	port.Script(fake.Response{Status: 200, Header: http.Header{"Content-Type": {"image/png"}}, Body: data})
	files := downloadTestFiles{FS: real.NewPorts().FS}
	path := filepath.Join(t.TempDir(), "image.png")
	h := desktopHandler(t, Deps{Files: files, Network: port}, api.CapabilityNetwork, api.CapabilityDesktopFiles)
	var got api.DownloadResponse
	raw := mustOK(t, serve(t, h, protocol.ServiceNetwork, protocol.OpDownload, api.DownloadRequest{URL: "https://example.com/image", Path: path, MediaTypes: []string{"image/png"}}))
	if err := jsonInto(raw, &got); err != nil || got.Bytes != int64(len(data)) || got.File.Path != path {
		t.Fatalf("download: %+v, %v", got, err)
	}
	other := desktopHandler(t, Deps{Files: files, Network: port}, api.CapabilityNetwork)
	requireCode(t, serve(t, other, protocol.ServiceNetwork, protocol.OpDownload, api.DownloadRequest{URL: "https://example.com/", Path: path}), api.CodePermission, "")
	if len(port.Requests()) != 1 {
		t.Fatal("missing declaration sent a request")
	}
}

func TestDownloadRejectsRedirectOversizeAndWrongMedia(t *testing.T) {
	for _, tc := range []struct {
		name     string
		response fake.Response
		max      int64
		types    []string
	}{
		{"redirect", fake.Response{Status: 302, Header: http.Header{"Location": {"https://other.example/"}}}, 100, nil},
		{"oversize", fake.Response{Status: 200, Body: bytes.Repeat([]byte("x"), 20)}, 8, nil},
		{"wrong-image", fake.Response{Status: 200, Header: http.Header{"Content-Type": {"image/png"}}, Body: []byte("not png")}, 100, []string{"image/png"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			port := fake.NewHTTP()
			port.Script(tc.response)
			files := downloadTestFiles{FS: real.NewPorts().FS}
			path := filepath.Join(t.TempDir(), "out")
			h := desktopHandler(t, Deps{Files: files, Network: port}, api.CapabilityNetwork, api.CapabilityDesktopFiles)
			reply := serve(t, h, protocol.ServiceNetwork, protocol.OpDownload, api.DownloadRequest{URL: "https://example.com/", Path: path, MaxBytes: tc.max, MediaTypes: tc.types})
			if reply.Failure == nil {
				t.Fatal("unexpected success")
			}
			if _, err := files.Stat(path); err == nil {
				t.Fatal("failure published file")
			}
			if len(port.Requests()) != 1 {
				t.Fatal("unexpected redirect request")
			}
		})
	}
}

func TestDownloadStopsBeforePublicationOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	port := fake.NewHTTP()
	port.Script(fake.Response{Status: 200, Body: []byte("body")})
	files := downloadTestFiles{FS: real.NewPorts().FS, beforeAdmit: cancel}
	path := filepath.Join(t.TempDir(), "out")
	h := desktopHandler(t, Deps{Files: files, Network: port}, api.CapabilityNetwork, api.CapabilityDesktopFiles)
	_, err := h.download(ctx, api.DownloadRequest{URL: "https://example.com", Path: path})
	if err == nil || !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
	if _, err := files.Stat(path); err == nil {
		t.Fatal("cancelled download published")
	}
}

func TestDownloadRedirectsRespectPolicyAndStripCredentials(t *testing.T) {
	port := fake.NewHTTP()
	port.Script(fake.Response{Status: 302, Header: http.Header{"Location": {"https://other.example/image"}}}, fake.Response{Status: 200, Body: []byte("body")})
	files := downloadTestFiles{FS: real.NewPorts().FS}
	h := desktopHandler(t, Deps{Files: files, Network: port}, api.CapabilityNetwork, api.CapabilityDesktopFiles)
	req := api.DownloadRequest{URL: "https://example.com/image", Path: filepath.Join(t.TempDir(), "out"), FollowRedirects: true, Headers: map[string]string{"Authorization": "secret"}}
	mustOK(t, serve(t, h, protocol.ServiceNetwork, protocol.OpDownload, req))
	requests := port.Requests()
	if len(requests) != 2 || requests[1].Header.Get("Authorization") != "" {
		t.Fatal("redirect leaked authorization")
	}
	port2 := fake.NewHTTP()
	port2.Script(fake.Response{Status: 302, Header: http.Header{"Location": {"file:///etc/passwd"}}})
	h2 := desktopHandler(t, Deps{Files: files, Network: port2}, api.CapabilityNetwork, api.CapabilityDesktopFiles)
	req.Path = filepath.Join(t.TempDir(), "other")
	if reply := serve(t, h2, protocol.ServiceNetwork, protocol.OpDownload, req); reply.Failure == nil || len(port2.Requests()) != 1 {
		t.Fatal("unsafe redirect was followed")
	}
}

func TestDownloadRefusesExistingDestination(t *testing.T) {
	files := downloadTestFiles{FS: real.NewPorts().FS}
	path := filepath.Join(t.TempDir(), "out")
	if err := files.WriteFile(path, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	port := fake.NewHTTP()
	h := desktopHandler(t, Deps{Files: files, Network: port}, api.CapabilityNetwork, api.CapabilityDesktopFiles)
	if reply := serve(t, h, protocol.ServiceNetwork, protocol.OpDownload, api.DownloadRequest{URL: "https://example.com", Path: path}); reply.Failure == nil {
		t.Fatal("existing file accepted")
	}
	data, err := files.ReadFile(path)
	if err != nil || string(data) != "keep" || len(port.Requests()) != 0 {
		t.Fatal("existing file changed or request sent")
	}
}
