package services

import (
	"encoding/json"
	"reflect"
	"regexp"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

func noCalc(string) (string, bool) { return "", false }

func TestExpandTemplateTable(t *testing.T) {
	values := map[string]string{"name": "Ada", "cursor": defaultCursorToken}
	calc := func(expression string) (string, bool) {
		if expression == "2+2" {
			return "4", true
		}
		return "", false
	}
	tests := []struct {
		name        string
		input       string
		want        string
		wantCursor  *int
		wantMissing []string
	}{
		{name: "plain text passes through", input: "hello", want: "hello"},
		{name: "single and double braces", input: "hi {name} and {{ name }}", want: "hi Ada and Ada"},
		{name: "missing names are empty and listed once, sorted", input: "{zed} {amy} {zed}", want: "  ", wantMissing: []string{"amy", "zed"}},
		{name: "argument prefix reads the same values", input: "{argument:name}", want: "Ada"},
		{name: "calculator evaluates offline", input: "sum {calculator: 2+2}", want: "sum 4"},
		{name: "unanswered calculator is empty and not missing", input: "{calculator: nope}", want: ""},
		{name: "cursor is removed and its byte offset kept", input: "ab{cursor}cd", want: "abcd", wantCursor: intPtr(2)},
		{name: "cursor offset counts bytes", input: "é{cursor}", want: "é", wantCursor: intPtr(2)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, cursor, missing := expandTemplate(tt.input, values, defaultCursorToken, calc)
			if got != tt.want {
				t.Fatalf("text = %q; want %q", got, tt.want)
			}
			if !reflect.DeepEqual(cursor, tt.wantCursor) {
				t.Fatalf("cursor = %v; want %v", deref(cursor), deref(tt.wantCursor))
			}
			if !reflect.DeepEqual(missing, tt.wantMissing) {
				t.Fatalf("missing = %v; want %v", missing, tt.wantMissing)
			}
		})
	}
	if _, _, missing := expandTemplate("{x}", values, defaultCursorToken, noCalc); len(missing) != 1 {
		t.Fatalf("unknown name not reported: %v", missing)
	}
}

func TestStandardVariablesFollowTheHostClock(t *testing.T) {
	r := newRig(t)
	h := handlerFor(t, r, manifestFor("acme.one"))
	result := h.expand(protocol.TemplateArgs{Input: "{date} {time} {datetime} {uuid}"})
	now := r.clock.Now().Local()
	want := now.Format("2006-01-02") + " " + now.Format("15:04") + " " + now.Format("2006-01-02 15:04") + " "
	if len(result.Text) < len(want)+36 || result.Text[:len(want)] != want {
		t.Fatalf("text = %q; want the clock's date and time before a UUID", result.Text)
	}
	uuid := regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !uuid.MatchString(result.Text) {
		t.Fatalf("uuid part of %q is not a version 4 UUID", result.Text)
	}
}

func TestClipboardNeedsTheCapabilityAndNoOptOut(t *testing.T) {
	r := newRig(t)
	r.clipboard.WriteText("copied text")
	withCapability := handlerFor(t, r, manifestFor("acme.clip", api.CapabilityClipboardHistory))
	withoutCapability := handlerFor(t, r, manifestFor("acme.plain"))
	if got := withCapability.expand(protocol.TemplateArgs{Input: "{clipboard}"}).Text; got != "copied text" {
		t.Fatalf("clipboard with the capability = %q", got)
	}
	if got := withoutCapability.expand(protocol.TemplateArgs{Input: "{clipboard}"}).Text; got != "" {
		t.Fatalf("clipboard without the capability = %q; want empty", got)
	}
	off := false
	if got := withCapability.expand(protocol.TemplateArgs{Input: "{clipboard}", Options: api.TemplateOptions{IncludeClipboard: &off}}).Text; got != "" {
		t.Fatalf("clipboard with includeClipboard false = %q; want empty", got)
	}
}

func TestTemplateServiceReturnsTheResultAsJSON(t *testing.T) {
	h := handlerFor(t, newRig(t), manifestFor("acme.one"))
	raw := mustOK(t, serve(t, h, protocol.ServiceText, protocol.OpTemplate, protocol.TemplateArgs{
		Input:   "Hi {who}{cursor}",
		Options: api.TemplateOptions{Variables: map[string]string{"who": "there"}},
	}))
	var result api.TemplateResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.Text != "Hi there" || result.Cursor == nil || *result.Cursor != len("Hi there") {
		t.Fatalf("result = %+v; want the text with the cursor at its end", result)
	}
}

func intPtr(n int) *int { return &n }

func deref(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}
