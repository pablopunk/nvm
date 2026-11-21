package extjobs

import (
	"context"
	"errors"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/jobs"
	"testing"
)

type unsupportedHost struct{ *fakeHost }

func (unsupportedHost) OperationsSupported(required []api.Operation) bool { return len(required) == 0 }

func TestOperationPrerequisiteSkipsBackgroundCallback(t *testing.T) {
	host := unsupportedHost{&fakeHost{invocations: make(chan api.Invocation, 1)}}
	adapter := New(context.Background(), Config{Host: func() Host { return host }})
	defer adapter.Close()
	command := api.Command{ID: "preview", Title: "Preview", Requires: []api.Operation{api.OperationFilePreview}}
	err := adapter.runCommand(plan{extensionID: "acme.test", command: command})(context.Background(), jobs.Run{})
	var skipped jobs.Skipped
	if !errors.As(err, &skipped) {
		t.Fatalf("not a prerequisite skip: %v", err)
	}
	if len(host.invocations) != 0 {
		t.Fatal("unsupported background callback executed")
	}
}
