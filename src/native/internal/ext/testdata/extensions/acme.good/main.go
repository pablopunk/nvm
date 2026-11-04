package main

import "strings"

var words = []string{"alpha", "beta", "gamma"}

// Commands, RootItems, SearchItems and RunAction are the entry points the host calls. Results are plain values.
func Commands(ctx any) ([]map[string]any, error) {
	return []map[string]any{{"id": "hello", "title": "Say hello"}}, nil
}

func RootItems(ctx any) ([]map[string]any, error) {
	return []map[string]any{{"id": "root", "title": "Good root"}}, nil
}

func SearchItems(ctx any, query string) ([]map[string]any, error) {
	var items []map[string]any
	for _, word := range words {
		if strings.Contains(word, query) {
			items = append(items, map[string]any{"id": word, "title": word})
		}
	}
	return items, nil
}

func RunAction(ctx any, in map[string]any) (map[string]any, error) {
	ref, _ := in["ref"].(map[string]any)
	return map[string]any{"ran": ref["id"]}, nil
}
