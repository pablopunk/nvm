package main

// SearchItems panics on the calling goroutine for one query. The child recovers the panic, reports it, and keeps running.
func SearchItems(ctx any, query string) ([]map[string]any, error) {
	if query == "boom" {
		panic("fixture panic on query boom")
	}
	return []map[string]any{{"id": "ok", "title": "Still alive"}}, nil
}
