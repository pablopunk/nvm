package main

import "time"

// SearchItems starts a goroutine that panics. A goroutine panic is not recoverable by the call that started it,
// so it ends the child process.
func SearchItems(ctx any, query string) ([]map[string]any, error) {
	go func() {
		panic("crash fixture: goroutine panic")
	}()
	time.Sleep(time.Second)
	return nil, nil
}
