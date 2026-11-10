package main

import (
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// SearchItems runs one ctx.AI check per query. The check's outcome is the title of its row.
func SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	check, ok := checks[query]
	if !ok {
		return nil, nil
	}
	return []api.Item{{ID: query, Title: check(ctx)}}, nil
}

var checks = map[string]func(api.Context) string{
	"ask":       askCheck,
	"stream":    streamCheck,
	"ratelimit": rateLimitCheck,
}

func askCheck(ctx api.Context) string {
	text, err := ctx.AI().Ask("say hi", api.AIOptions{Model: api.ModelFast, System: "Be brief."})
	if err != nil {
		return "ask failed: " + err.Error()
	}
	return "ask: " + text
}

func streamCheck(ctx api.Context) string {
	stream, err := ctx.AI().Stream("stream it", api.AIOptions{})
	if err != nil {
		return "stream failed: " + err.Error()
	}
	var types []string
	for event := range stream.Events() {
		types = append(types, event.Type)
	}
	text, err := stream.Result()
	if err != nil {
		return "stream result failed: " + err.Error()
	}
	return fmt.Sprintf("stream: %s events=%s", text, strings.Join(types, ","))
}

func rateLimitCheck(ctx api.Context) string {
	for call := 1; call <= 31; call++ {
		if _, err := ctx.AI().Ask("again", api.AIOptions{}); err != nil {
			return fmt.Sprintf("ratelimit: call %d failed: %v", call, err)
		}
	}
	return "ratelimit: all 31 calls passed"
}
