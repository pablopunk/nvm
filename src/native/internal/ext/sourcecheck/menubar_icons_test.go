package sourcecheck

import (
	"reflect"
	"testing"
)

func TestUnknownMenuBarIconsAreReported(t *testing.T) {
	source := `package main

import "github.com/pablopunk/nvm/native/internal/ext/api"

var bar = api.MenuBar{ID: "a", Icon: api.MenuBarIcon("not-an-icon")}
var row = api.MenuRow{ID: "b", Icon: "also-wrong"}
var good = api.MenuBar{ID: "c", Icon: api.MenuBarIcon("github")}
var fallback = api.MenuBar{ID: "d", Icon: api.MenuIconDefault}
`
	got := UnknownMenuBarIcons(map[string][]byte{"main.go": []byte(source)})
	if want := []string{"also-wrong", "not-an-icon"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unknown = %v, want %v", got, want)
	}
}
