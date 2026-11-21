package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// SearchItems tries one abuse per query: quota overruns, undeclared capabilities, private addresses, reads of
// another extension's data, and log floods. Each row reports what the host answered.
func SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	check, ok := abuses[query]
	if !ok {
		return nil, nil
	}
	return []api.Item{{ID: query, Title: check(ctx)}}, nil
}

var abuses = map[string]func(api.Context) string{
	"quota":       keyFlood,
	"big":         bigValue,
	"traversal":   traversal,
	"read-shared": readShared,
	"undeclared":  undeclaredSecret,
	"private":     privateFetch,
	"settings":    undeclaredSettings,
	"flood":       logFlood,
}

func keyFlood(ctx api.Context) string {
	written := 0
	var err error
	for i := 0; i < 1100; i++ {
		if err = ctx.Storage().Set(fmt.Sprintf("k%d", i), i); err != nil {
			break
		}
		written++
	}
	return fmt.Sprintf("quota written=%d quota=%t", written, errors.Is(err, api.ErrQuotaExceeded))
}

func bigValue(ctx api.Context) string {
	err := ctx.Storage().Set("big", strings.Repeat("x", 300<<10))
	return fmt.Sprintf("big quota=%t", errors.Is(err, api.ErrQuotaExceeded))
}

// traversal names a path where a key would be. A key is a key: the host never opens a file from it.
func traversal(ctx api.Context) string {
	found, err := ctx.Storage().Get("../acme.services/storage.json", nil)
	return fmt.Sprintf("traversal found=%t err=%t", found, err != nil)
}

func readShared(ctx api.Context) string {
	found, err := ctx.Storage().Get("shared-key", nil)
	return fmt.Sprintf("read-shared found=%t err=%t", found, err != nil)
}

func undeclaredSecret(ctx api.Context) string {
	_, _, err := ctx.Secrets().Get("TOKEN")
	return fmt.Sprintf("undeclared permission=%t", errors.Is(err, api.ErrPermission))
}

func privateFetch(ctx api.Context) string {
	_, err := ctx.Network().Fetch(api.FetchRequest{URL: "http://127.0.0.1:9/"})
	return fmt.Sprintf("private permission=%t", errors.Is(err, api.ErrPermission))
}

func undeclaredSettings(ctx api.Context) string {
	err := ctx.Settings().Set("units", "imperial")
	return fmt.Sprintf("settings permission=%t", errors.Is(err, api.ErrPermission))
}

func logFlood(ctx api.Context) string {
	for i := 0; i < 400; i++ {
		ctx.Logs().Log(api.LogInfo, "spam", nil)
	}
	entries, _ := ctx.Logs().Recent(api.LogQuery{Query: "spam", Limit: 500})
	return fmt.Sprintf("flood kept=%d", len(entries))
}
