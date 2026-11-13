//go:build linux

package real

import (
	"bytes"
	"os"
	"os/exec"
	"testing"
)

// TestWatchScriptRecordsCopiesWhateverWlPasteExports runs the watch script in a real shell the way wl-paste --watch
// does, with and without the CLIPBOARD_STATE and CLIPBOARD_TYPE variables that only some wl-clipboard releases
// export, and checks every copy still parses.
func TestWatchScriptRecordsCopiesWhateverWlPasteExports(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\nrest-of-image")
	cases := []struct {
		name  string
		env   []string
		input []byte
		check func(t *testing.T, line []byte)
	}{
		{"text without variables", nil, []byte("hello there"), wantText("hello there")},
		{"text with variables", []string{"CLIPBOARD_STATE=data", "CLIPBOARD_TYPE=text/plain;charset=utf-8"}, []byte("hi"), wantText("hi")},
		{"image without variables", nil, png, wantPNG(png)},
		{"sensitive copy", []string{"CLIPBOARD_STATE=sensitive"}, []byte("secret"), wantConcealed},
		{"emptied clipboard", []string{"CLIPBOARD_STATE=nil"}, nil, wantNothing},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command("sh", "-c", waylandWatchScript)
			cmd.Env = append(withoutClipboardVariables(os.Environ()), tc.env...)
			cmd.Stdin = bytes.NewReader(tc.input)
			out, err := cmd.Output()
			if err != nil {
				t.Fatal(err)
			}
			tc.check(t, bytes.TrimSuffix(out, []byte("\n")))
		})
	}
}

func withoutClipboardVariables(env []string) []string {
	var out []string
	for _, kv := range env {
		if !bytes.HasPrefix([]byte(kv), []byte("CLIPBOARD_")) {
			out = append(out, kv)
		}
	}
	return out
}

func wantText(text string) func(*testing.T, []byte) {
	return func(t *testing.T, line []byte) {
		t.Helper()
		got, ok := parseWaylandCopy(line)
		if !ok || got.Text != text {
			t.Fatalf("line %q parsed as %+v, %v; want text %q", line, got, ok, text)
		}
	}
}

func wantPNG(png []byte) func(*testing.T, []byte) {
	return func(t *testing.T, line []byte) {
		t.Helper()
		got, ok := parseWaylandCopy(line)
		if !ok || !bytes.Equal(got.PNG, png) {
			t.Fatalf("line %q parsed as %+v, %v; want the PNG", line, got, ok)
		}
	}
}

func wantConcealed(t *testing.T, line []byte) {
	t.Helper()
	if got, ok := parseWaylandCopy(line); !ok || !got.Concealed || got.Text != "" {
		t.Fatalf("line %q parsed as %+v, %v; want a concealed copy", line, got, ok)
	}
}

func wantNothing(t *testing.T, line []byte) {
	t.Helper()
	if got, ok := parseWaylandCopy(line); ok {
		t.Fatalf("line %q parsed as %+v; want no copy", line, got)
	}
}
