package child_test

import (
	"context"
	"errors"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/child"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/ext/services"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/platform/real"
	"testing"
)

type attachmentSelection struct {
	calls int
	text  string
	found bool
	paths []string
	err   error
}

func (s *attachmentSelection) Text() (string, bool, error) { s.calls++; return s.text, s.found, s.err }
func (s *attachmentSelection) Files(context.Context) ([]string, error) {
	s.calls++
	return s.paths, s.err
}
func (s *attachmentSelection) Replace(string) (bool, error) { panic("unexpected replacement") }
func (s *attachmentSelection) Undo() error                  { panic("unexpected undo") }

func TestAttachmentSelectionUsesProductionHostContract(t *testing.T) {
	for _, scenario := range []string{"success", "empty", "denied", "unsupported", "undeclared", "cancelled", "closed"} {
		t.Run(scenario, func(t *testing.T) {
			selection := &attachmentSelection{text: "  selected text\n", found: true, paths: []string{"/tmp/one.txt", "/tmp/two.png"}}
			capabilities := []api.Capability{api.CapabilityDesktopSelection, api.CapabilityDesktopFiles}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch scenario {
			case "empty":
				selection.found = false
				selection.text = ""
				selection.paths = nil
			case "denied":
				selection.err = platform.ErrDenied
			case "unsupported":
				selection.err = platform.ErrUnsupported
			case "undeclared":
				capabilities = nil
			case "cancelled":
				cancel()
			}
			clock := fake.NewClock()
			host := services.New(services.Deps{Files: real.NewPorts().FS, Clock: clock, DataRoot: t.TempDir(), Selection: selection}, api.Manifest{APIVersion: "1.0", Identity: api.Identity{ID: "acme.attachments"}, Capabilities: capabilities})
			defer host.Close()
			if scenario == "closed" {
				_ = host.Close()
			}
			direct := child.NewDirectContext(api.Identity{ID: "acme.attachments"}, clock, func(call protocol.ServiceCall) protocol.ServiceReply { return host.Serve(ctx, call) })
			builder := direct.For(ctx, api.LaunchContext{}).AI().Attachments()
			text, textErr := builder.SelectedText("")
			files, fileErr := builder.SelectedFiles("metadata")
			switch scenario {
			case "success":
				if textErr != nil || fileErr != nil || text.Kind != "text" || text.Title != "Selected Text" || text.Text != selection.text || len(files) != 2 || files[0].Path != "/tmp/one.txt" || files[1].Path != "/tmp/two.png" || files[0].As != "metadata" || files[0].Kind != "file" {
					t.Fatalf("text=%+v files=%+v errors=%v/%v", text, files, textErr, fileErr)
				}
			case "empty":
				if !errors.Is(textErr, api.ErrNotAvailable) || fileErr != nil || len(files) != 0 {
					t.Fatalf("empty: %v %v %+v", textErr, fileErr, files)
				}
			default:
				want := api.ErrNotAvailable
				if scenario == "denied" || scenario == "undeclared" {
					want = api.ErrPermission
				}
				if scenario == "cancelled" {
					want = context.Canceled
				}
				if !errors.Is(textErr, want) || !errors.Is(fileErr, want) {
					t.Fatalf("errors=%v/%v want=%v", textErr, fileErr, want)
				}
			}
			if (scenario == "undeclared" || scenario == "cancelled" || scenario == "closed") && selection.calls != 0 {
				t.Fatalf("refused reads=%d", selection.calls)
			}
		})
	}
}
