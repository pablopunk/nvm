package builder

import (
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// sourcedExtensions is the fake extension manager with one extension whose source can be read.
type sourcedExtensions struct {
	*fakeExtensions
	source string
}

func (e sourcedExtensions) Definition(id string) (api.Manifest, string, error) {
	manifest, _, err := e.fakeExtensions.Definition(id)
	return manifest, e.source, err
}

func TestReadExtensionShowsAnotherExtensionsSource(t *testing.T) {
	extensions := newFakeExtensions()
	extensions.existing["user.other"] = api.Manifest{ID: "user.other", Title: "Other"}
	w := New(newFakeConversation(), sourcedExtensions{fakeExtensions: extensions, source: "package main // other"}, synchronous)

	result, err := w.readExtension(readExtensionArgs{ID: "user.other"})
	if err != nil {
		t.Fatalf("readExtension: %v", err)
	}
	if !strings.Contains(result.Text, "package main // other") || !strings.Contains(result.Text, "read-only") {
		t.Fatalf("result = %q, want the read-only source", result.Text)
	}
	if _, err := w.readExtension(readExtensionArgs{ID: "user.missing"}); err == nil {
		t.Fatal("reading a missing extension succeeded")
	}
}

func TestReadingAnotherExtensionDoesNotAllowWritingIt(t *testing.T) {
	extensions := newFakeExtensions()
	extensions.existing["user.other"] = api.Manifest{ID: "user.other", Title: "Other"}
	w := New(newFakeConversation(), extensions, synchronous)
	if problem := w.ownershipProblem("user.mine", "user.other"); problem == "" {
		t.Fatal("a chat bound to its extension could write another one")
	}
	if problem := w.ownershipProblem("", "user.other"); problem == "" {
		t.Fatal("a new chat could overwrite an installed extension")
	}
}
