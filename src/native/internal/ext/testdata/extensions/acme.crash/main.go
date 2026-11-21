package main

import (
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// SearchItems starts a goroutine that panics. A goroutine panic is not recoverable by the call that started it,
// so it ends the child process.
func SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	go func() {
		panic("crash fixture: goroutine panic")
	}()
	time.Sleep(time.Second)
	return nil, nil
}
