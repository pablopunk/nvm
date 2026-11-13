package learning

import (
	"bytes"
	"errors"
	"io/fs"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/platform/testpath"
	"github.com/pablopunk/nvm/native/internal/store"
)

func newRules(t *testing.T) (*Rules, *fake.FS) {
	t.Helper()
	files := fake.NewFS()
	st, err := store.Open(files, fake.NewClock(), testpath.Abs("learning"), store.WithKeepBackups(0))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	doc := store.Doc(st, "rules.json", 1, func() Document { return Document{} })
	rules, err := OpenRules(doc, bytes.Repeat([]byte("k"), 32))
	if err != nil {
		t.Fatal(err)
	}
	return rules, files
}
func TestRulesRejectStaleReviewAndDeletedRecreation(t *testing.T) {
	rules, _ := newRules(t)
	input := Rule{ID: "one", Kind: "preference", Text: "Prefer concise labels"}
	if err := rules.ApplyAdds(0, []Rule{input}); err != nil {
		t.Fatal(err)
	}
	before, _ := rules.Snapshot()
	rule := before.Rules[0]
	if err := rules.Delete(before.Revision, rule.ID, rule.Revision); err != nil {
		t.Fatal(err)
	}
	if err := rules.ApplyAdds(before.Revision, []Rule{{ID: "two", Kind: "preference", Text: "Prefer compact labels"}}); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale review: %v", err)
	}
	after, _ := rules.Snapshot()
	if err := rules.ApplyAdds(after.Revision, []Rule{{ID: "two", Kind: "preference", Text: "  PREFER concise labels  "}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("deleted rule: %v", err)
	}
}
func TestRulesFailurePreservesCommittedSnapshot(t *testing.T) {
	rules, files := newRules(t)
	if err := rules.ApplyAdds(0, []Rule{{ID: "one", Kind: "workflow", Text: "Keep edits small"}}); err != nil {
		t.Fatal(err)
	}
	before, _ := rules.Snapshot()
	files.FailNext(fake.OpRename, fs.ErrPermission)
	if err := rules.Delete(before.Revision, before.Rules[0].ID, before.Rules[0].Revision); err == nil {
		t.Fatal("failed write succeeded")
	}
	after, _ := rules.Snapshot()
	if len(after.Rules) != 1 || after.Revision != before.Revision {
		t.Fatalf("lost rule: %+v", after)
	}
}
