package main

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// SearchItems runs one context service per query, so each check reports its outcome as a row title.
func SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	check, ok := checks[query]
	if !ok {
		return nil, nil
	}
	return []api.Item{{ID: query, Title: check(ctx)}}, nil
}

// LoadItems is the data loader the host calls for a view source. The loader named "fail" fails on purpose.
func LoadItems(ctx api.Context, loader string) ([]api.Item, error) {
	if loader == "fail" {
		return nil, fmt.Errorf("feed %s is down", loader)
	}
	return []api.Item{{ID: loader, Title: "loaded " + loader}}, nil
}

var checks = map[string]func(api.Context) string{
	"storage":  storageCheck,
	"memo":     memoCheck,
	"secrets":  secretsCheck,
	"settings": settingsCheck,
	"logs":     logsCheck,
	"text":     textCheck,
	"network":  networkCheck,
	"cache":    cacheCheck,
	"shared":   sharedCheck,
}

func storageCheck(ctx api.Context) string {
	if err := ctx.Storage().Set("city", map[string]string{"name": "Lisbon"}); err != nil {
		return "storage set failed: " + err.Error()
	}
	var city map[string]string
	found, err := ctx.Storage().Get("city", &city)
	if err != nil {
		return "storage get failed: " + err.Error()
	}
	return fmt.Sprintf("storage found=%t city=%s", found, city["name"])
}

var memoLoads int

func memoCheck(ctx api.Context) string {
	var value int
	if err := ctx.Storage().Memo("answer", time.Minute, &value, func() (any, error) {
		memoLoads++
		return 42, nil
	}); err != nil {
		return "memo failed: " + err.Error()
	}
	var again int
	if err := ctx.Storage().Memo("answer", time.Minute, &again, func() (any, error) {
		memoLoads++
		return 0, nil
	}); err != nil {
		return "memo failed: " + err.Error()
	}
	return fmt.Sprintf("memo value=%d again=%d loads=%d", value, again, memoLoads)
}

func secretsCheck(ctx api.Context) string {
	if err := ctx.Secrets().Set("API_TOKEN", "canary-token-7f3a9c"); err != nil {
		return "secrets set failed: " + err.Error()
	}
	value, found, err := ctx.Secrets().Get("API_TOKEN")
	if err != nil {
		return "secrets get failed: " + err.Error()
	}
	return fmt.Sprintf("secrets found=%t value=%s", found, value)
}

func settingsCheck(ctx api.Context) string {
	var units string
	if err := ctx.Settings().Get("units", &units); err != nil {
		return "settings get failed: " + err.Error()
	}
	if err := ctx.Settings().Set("units", "imperial"); err != nil {
		return "settings set failed: " + err.Error()
	}
	toggled, err := ctx.Settings().Toggle("compact")
	if err != nil {
		return "settings toggle failed: " + err.Error()
	}
	return fmt.Sprintf("settings default=%s definitions=%d compact=%s", units, len(ctx.Settings().Definitions()), toggled)
}

func logsCheck(ctx api.Context) string {
	ctx.Logs().Log(api.LogInfo, "hello from services", map[string]int{"n": 2})
	entries, err := ctx.Logs().Recent(api.LogQuery{Query: "hello"})
	if err != nil {
		return "logs failed: " + err.Error()
	}
	return fmt.Sprintf("logs recent=%d", len(entries))
}

func textCheck(ctx api.Context) string {
	result, err := ctx.Text().Template("hi {name}{cursor}", api.TemplateOptions{Variables: map[string]string{"name": "there"}})
	if err != nil {
		return "text failed: " + err.Error()
	}
	cursor := -1
	if result.Cursor != nil {
		cursor = *result.Cursor
	}
	return fmt.Sprintf("text=%s cursor=%s", result.Text, strconv.Itoa(cursor))
}

func networkCheck(ctx api.Context) string {
	_, err := ctx.Network().Fetch(api.FetchRequest{URL: "http://127.0.0.1:9/"})
	return fmt.Sprintf("network permission=%t", errors.Is(err, api.ErrPermission))
}

func cacheCheck(ctx api.Context) string {
	ctx.Cache().Set("k", 7, time.Minute)
	value, ok := ctx.Cache().Get("k")
	return fmt.Sprintf("cache found=%t value=%v", ok, value)
}

func sharedCheck(ctx api.Context) string {
	if err := ctx.Storage().Set("shared-key", "from services"); err != nil {
		return "shared failed: " + err.Error()
	}
	return "shared written"
}
