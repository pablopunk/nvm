package main

import (
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// SearchItems runs one ctx.Dictation check per query. The check's outcome is the title of its row.
func SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	check, ok := checks[query]
	if !ok {
		return nil, nil
	}
	return []api.Item{{ID: query, Title: check(ctx)}}, nil
}

var checks = map[string]func(api.Context) string{
	"status":     statusCheck,
	"meter":      meterCheck,
	"available":  availableCheck,
	"devices":    devicesCheck,
	"dictate":    dictateCheck,
	"cancel":     cancelCheck,
	"recordings": recordingsCheck,
	"retry":      retryCheck,
	"delete":     deleteCheck,
}

func statusCheck(ctx api.Context) string {
	status, err := ctx.Dictation().Status()
	if err != nil {
		return "status failed: " + err.Error()
	}
	return "status: " + string(status)
}

func availableCheck(ctx api.Context) string {
	available, err := ctx.Dictation().APIAvailable()
	if err != nil {
		return "available failed: " + err.Error()
	}
	return fmt.Sprintf("available: %t", available)
}

func devicesCheck(ctx api.Context) string {
	devices, err := ctx.Dictation().Devices()
	if err != nil {
		return "devices failed: " + err.Error()
	}
	var titles []string
	for _, device := range devices {
		titles = append(titles, device.Title)
	}
	return "devices: " + strings.Join(titles, ",")
}

// dictateCheck starts a dictation with the default microphone and muted output, then stops it for its transcript.
func dictateCheck(ctx api.Context) string {
	if err := ctx.Dictation().Start(api.DictationStartOptions{DeviceID: "default", MuteSystemAudioWhileRecord: true}); err != nil {
		return "start failed: " + err.Error()
	}
	transcript, err := ctx.Dictation().Stop()
	if err != nil {
		return "stop failed: " + err.Error()
	}
	return "transcript: " + transcript
}

func cancelCheck(ctx api.Context) string {
	if err := ctx.Dictation().Cancel(); err != nil {
		return "cancel failed: " + err.Error()
	}
	return "cancel: done"
}

func recordingsCheck(ctx api.Context) string {
	saved, err := ctx.Dictation().Recordings()
	if err != nil {
		return "recordings failed: " + err.Error()
	}
	var ids []string
	for _, recording := range saved {
		ids = append(ids, fmt.Sprintf("%s/%d", recording.ID, recording.SegmentCount))
	}
	return "recordings: " + strings.Join(ids, ",")
}

func retryCheck(ctx api.Context) string {
	transcript, err := ctx.Dictation().Retry("dictation-saved")
	if err != nil {
		return "retry failed: " + err.Error()
	}
	return "retry: " + transcript
}

func deleteCheck(ctx api.Context) string {
	if err := ctx.Dictation().DeleteRecording("dictation-saved"); err != nil {
		return "delete failed: " + err.Error()
	}
	return "delete: done"
}

// meterCheck exercises the same declarative binding as the built-in, under a different owner.
func meterCheck(ctx api.Context) string {
	const id = "microphone"
	if err := ctx.Indicators().Show(api.IndicatorInput{ID: id, Title: "Fixture recording", Status: api.IndicatorRecording}); err != nil {
		return err.Error()
	}
	defer ctx.Indicators().Hide(id)
	if err := ctx.Dictation().Start(api.DictationStartOptions{IndicatorID: id}); err != nil {
		return err.Error()
	}
	return "meter: " + cancelCheck(ctx)
}
