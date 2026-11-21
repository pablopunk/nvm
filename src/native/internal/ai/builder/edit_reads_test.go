package builder

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestEditReadsRequiresEveryPageAndKeepsSnapshot(t *testing.T) {
	original := strings.Repeat("x", apiPageBytes*2)
	files := map[string][]byte{"main.go": []byte(original), "helper.go": []byte("helper")}
	token := new(int)
	calls := 0
	r := newEditReads(func(string) (map[string][]byte, any, error) { calls++; return files, token, nil })
	first, err := r.read(context.Background(), "user.test", 0)
	if err != nil || !strings.Contains(first.Text, "helper.go") {
		t.Fatalf("first: %v %s", err, first.Text)
	}
	if _, ok := r.revision("user.test"); ok {
		t.Fatal("partial read granted revision")
	}
	if _, err := r.read(context.Background(), "user.test", apiPageBytes*2); err == nil {
		t.Fatal("skipped page accepted")
	}
	files["main.go"] = []byte("changed")
	for !r.entries["user.test"].complete {
		offset := r.entries["user.test"].next
		page, err := r.read(context.Background(), "user.test", offset)
		if err != nil || strings.Contains(page.Text, "changed") {
			t.Fatalf("page: %v %s", err, page.Text)
		}
	}
	if got, ok := r.revision("user.test"); !ok || got != token || calls != 1 {
		t.Fatal("missing stable revision")
	}
	if _, ok := r.revision("user.other"); ok {
		t.Fatal("revision crossed targets")
	}
	if _, ok := newEditReads(r.fetch).revision("user.test"); ok {
		t.Fatal("revision crossed turns")
	}
}

func TestEditReadsRefusesMissingFailedAndCancelledSnapshots(t *testing.T) {
	for _, fetch := range []func(string) (map[string][]byte, any, error){
		func(string) (map[string][]byte, any, error) { return nil, new(int), nil },
		func(string) (map[string][]byte, any, error) { return nil, nil, errors.New("unreadable") },
		func(string) (map[string][]byte, any, error) {
			return map[string][]byte{"main.go": []byte("x")}, nil, nil
		},
	} {
		r := newEditReads(fetch)
		if _, err := r.read(context.Background(), "user.test", 0); err == nil {
			t.Fatal("accepted invalid snapshot")
		}
		if _, ok := r.revision("user.test"); ok {
			t.Fatal("invalid snapshot granted revision")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := newEditReads(func(string) (map[string][]byte, any, error) {
		t.Fatal("called after cancellation")
		return nil, nil, nil
	})
	if _, err := r.read(ctx, "user.test", 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
}
