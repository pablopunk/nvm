package main

import "strings"

// SearchItems returns a result far larger than one protocol line may carry.
func SearchItems(ctx any, query string) ([]map[string]any, error) {
	padding := strings.Repeat("x", 2<<20)
	return []map[string]any{{"id": "huge", "title": padding}}, nil
}
