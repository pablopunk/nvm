package main

// SearchItems allocates far more memory than the child's address-space limit allows.
func SearchItems(ctx any, query string) ([]map[string]any, error) {
	block := make([]byte, 4<<30)
	block[len(block)-1] = 1
	return []map[string]any{{"id": "big", "title": string(block[len(block)-1:])}}, nil
}
