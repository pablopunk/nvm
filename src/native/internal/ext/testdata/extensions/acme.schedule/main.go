package main

import (
	"strconv"
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// acme.schedule is the dev fixture of the one-shot trigger, driven through the search box. "arm <ms>" schedules the
// fire action under the key "job" that many milliseconds ahead, "pending" reports whether a run is pending, and "cancel"
// drops it. "fired" shows how many times the fire action ran, which it records in the extension's storage.

const jobKey = "job"

func SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	switch {
	case strings.HasPrefix(query, "arm "):
		return nil, arm(ctx, query)
	case query == "pending":
		return pendingRow(ctx)
	case query == "cancel":
		return cancelRow(ctx)
	case query == "fired":
		return firedRow(ctx)
	}
	return nil, nil
}

func arm(ctx api.Context, query string) error {
	ms, err := strconv.Atoi(strings.TrimPrefix(query, "arm "))
	if err != nil {
		return err
	}
	at := ctx.Clock().Now().Add(time.Duration(ms) * time.Millisecond)
	return ctx.Scheduler().Once(jobKey, at, api.ActionRef{ID: "fire"})
}

func pendingRow(ctx api.Context) ([]api.Item, error) {
	_, pending, err := ctx.Scheduler().Pending(jobKey)
	if err != nil {
		return nil, err
	}
	return []api.Item{{ID: "pending", Title: strconv.FormatBool(pending)}}, nil
}

func cancelRow(ctx api.Context) ([]api.Item, error) {
	cancelled, err := ctx.Scheduler().Cancel(jobKey)
	if err != nil {
		return nil, err
	}
	return []api.Item{{ID: "cancel", Title: strconv.FormatBool(cancelled)}}, nil
}

func firedRow(ctx api.Context) ([]api.Item, error) {
	var fired int
	if _, err := ctx.Storage().Get("fired", &fired); err != nil {
		return nil, err
	}
	return []api.Item{{ID: "fired", Title: strconv.Itoa(fired)}}, nil
}

func RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	if in.Ref.ID != "fire" {
		return api.Result{}, nil
	}
	var fired int
	if _, err := ctx.Storage().Get("fired", &fired); err != nil {
		return api.Result{}, err
	}
	return api.Result{}, ctx.Storage().Set("fired", fired+1)
}
