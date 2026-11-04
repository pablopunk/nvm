package main

import "time"

// SearchItems answers after a delay longer than the test's hard budget. It declares no RootItems.
func SearchItems(ctx any, query string) ([]map[string]any, error) {
	time.Sleep(300 * time.Millisecond)
	return []map[string]any{{"id": "late", "title": "Too late"}}, nil
}
