package main

// This source deliberately does not compile: the child must report it as a load failure, not crash.
func SearchItems(ctx any, query string) ([]map[string]any, error) {
	return undefinedHelper(query), nil
}
