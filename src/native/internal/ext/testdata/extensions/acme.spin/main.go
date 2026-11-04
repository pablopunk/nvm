package main

// SearchItems never returns. The host must kill the child at its hard deadline.
func SearchItems(ctx any, query string) ([]map[string]any, error) {
	for {
	}
}

func RootItems(ctx any) ([]map[string]any, error) {
	return []map[string]any{{"id": "root", "title": "Root still works after a restart"}}, nil
}
