package manager

import (
	"slices"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func writeSource(t *testing.T, files *fake.FS, dir, source string) {
	t.Helper()
	if err := files.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := files.WriteFile(dir+"/main.go", []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCommandOnlyExtensionsCanWaitForTheirFirstCall(t *testing.T) {
	files := fake.NewFS()
	writeSource(t, files, "/ext/commands", "package main\nfunc Commands() {}\nfunc RunAction() {}\n")
	methods, dormant := dormantMethods(files, "/ext/commands")
	if !dormant || !slices.Equal(methods, []string{"commands", "run"}) {
		t.Fatalf("dormant %v methods %v", dormant, methods)
	}
	writeSource(t, files, "/ext/search", "package main\nfunc Commands() {}\nfunc SearchItems() {}\n")
	if _, dormant := dormantMethods(files, "/ext/search"); dormant {
		t.Fatal("an extension the palette queries unasked went dormant")
	}
	writeSource(t, files, "/ext/broken", "package main\nfunc (\n")
	if _, dormant := dormantMethods(files, "/ext/broken"); dormant {
		t.Fatal("source that does not parse went dormant")
	}
}

func TestCommandSnapshotsSurviveARestartAndFollowTheSource(t *testing.T) {
	files := fake.NewFS()
	if err := files.MkdirAll("/data", 0o755); err != nil {
		t.Fatal(err)
	}
	commands := []api.Command{{ID: "start", Title: "Start"}}
	openCommandSnapshots(files, "/data").put("acme.timer", "hash-1", commands)
	reopened := openCommandSnapshots(files, "/data")
	got, ok := reopened.get("acme.timer", "hash-1")
	if !ok || len(got) != 1 || got[0].ID != "start" {
		t.Fatalf("got %v, %v", got, ok)
	}
	if _, ok := reopened.get("acme.timer", "hash-2"); ok {
		t.Fatal("commands of changed source were reused")
	}
}
