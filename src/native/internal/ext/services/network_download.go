package services

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform"
)

const maxDownloadBytes int64 = 64 << 20
const downloadTimeout = 90 * time.Second

// download streams only after the URL, declarations, quotas and destination have been admitted.
func (h *Handler) download(ctx context.Context, request api.DownloadRequest) (api.DownloadResponse, error) {
	var empty api.DownloadResponse
	if !filepath.IsAbs(request.Path) {
		return empty, invalid("a file path must be absolute")
	}
	target, err := parseFetchURL(request.URL)
	if err != nil {
		return empty, err
	}
	headers, err := planFetchHeaders(request.Headers)
	if err != nil {
		return empty, err
	}
	maxBytes, err := boundedBytes(request.MaxBytes, maxDownloadBytes, maxDownloadBytes)
	if err != nil {
		return empty, err
	}
	if request.TimeoutMs < 0 {
		return empty, invalid("a timeout cannot be negative")
	}
	timeout := downloadTimeout
	if request.TimeoutMs > 0 && request.TimeoutMs < downloadTimeout.Milliseconds() {
		timeout = time.Duration(request.TimeoutMs) * time.Millisecond
	}
	types := make(map[string]bool, len(request.MediaTypes))
	if len(request.MediaTypes) > 16 {
		return empty, invalid("too many media types")
	}
	for _, raw := range request.MediaTypes {
		kind, params, err := mime.ParseMediaType(raw)
		if err != nil || len(params) != 0 {
			return empty, invalid("invalid media type")
		}
		types[kind] = true
	}
	stream, ok := h.deps.Files.(platform.StreamFiles)
	if !ok || h.deps.Network == nil {
		return empty, unavailable("network downloads are not available in this build")
	}
	path := filepath.Clean(request.Path)
	if err := h.checkWritableTarget(path, request.Overwrite); err != nil {
		return empty, err
	}
	if !h.fetches.allow() {
		return empty, quota(errors.New("too many network requests; wait a minute and try again"))
	}
	if !h.actions.allow() {
		return empty, rateExceeded("file writes")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	stop := context.AfterFunc(protocol.LifetimeOf(ctx), cancel)
	defer stop()
	plan := fetchPlan{method: "GET", target: target, headers: headers, timeout: timeout, maxBytes: maxBytes}
	exchange := plan.exchange()
	exchange.Check = func(target *url.URL) error {
		if err := checkFetchTarget(target); err != nil {
			return err
		}
		return h.admitDesktopEffect(ctx)
	}
	response, finalURL, err := platform.OpenExchange(ctx, h.deps.Network, exchange, request.FollowRedirects)
	if err != nil {
		return empty, transportFailure(ctx, err, plan)
	}
	defer response.Body.Close()
	if response.StatusCode != platform.StatusOK {
		return empty, unavailable("download returned HTTP %d", response.StatusCode)
	}
	if response.ContentLength > maxBytes {
		return empty, transportFailure(ctx, platform.ErrResponseTooLarge, plan)
	}
	progress := h.newDownloadProgress(ctx, request, response.ContentLength)
	finished := false
	defer func() {
		if !finished {
			progress.abandon()
		}
	}()
	body := progress.wrap(response.Body)
	source := body
	if len(types) > 0 {
		declared, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
		if err != nil || !types[declared] {
			return empty, invalid("download has an unexpected content type")
		}
		prefix := make([]byte, min(int64(512), maxBytes+1))
		n, err := io.ReadFull(body, prefix)
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
			return empty, transportFailure(ctx, err, plan)
		}
		if int64(n) > maxBytes {
			return empty, transportFailure(ctx, platform.ErrResponseTooLarge, plan)
		}
		prefix = prefix[:n]
		sniffed := platform.DetectContentType(prefix)
		sniffed, _, _ = mime.ParseMediaType(sniffed)
		if sniffed != declared || !types[sniffed] {
			return empty, invalid("download content does not match its media type")
		}
		source = io.MultiReader(bytes.NewReader(prefix), body)
	}
	result, err := stream.WriteStream(ctx, path, source, maxBytes, request.Overwrite, func() error { return h.admitDesktopEffect(ctx) })
	// A published file is the download: a failure to remove the staging copy afterwards is retried by the platform and
	// must not report the finished download as failed, or a retry without overwrite would refuse the file it made.
	if err != nil && !result.Published {
		if errors.Is(err, platform.ErrFileTooLarge) {
			err = platform.ErrResponseTooLarge
		}
		return empty, transportFailure(ctx, err, plan)
	}
	if !result.Published {
		return empty, unavailable("download was not published")
	}
	modified := time.Time{}
	if info, err := h.deps.Files.Stat(path); err == nil {
		modified = info.ModTime()
	}
	flattened := map[string]string{}
	for name, values := range response.Header {
		flattened[strings.ToLower(name)] = strings.Join(values, ", ")
	}
	finished = true
	return api.DownloadResponse{Status: response.StatusCode, FinalURL: finalURL.String(), Headers: flattened, Bytes: result.Bytes, File: fileRefOf(path, h.deps.Home, result.Bytes, modified, time.Time{})}, nil
}
