package main

// Package initialisation loops forever, so the child never reports ready.
func init() {
	for {
	}
}

func SearchItems(ctx any, query string) ([]map[string]any, error) {
	return nil, nil
}
