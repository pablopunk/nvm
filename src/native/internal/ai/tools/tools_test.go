package tools

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ai/agent"
	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/platform/testpath"
)

var home = testpath.Abs("home", "u")

// rig is the conversation's tools over fake ports, with a home folder that holds a few files.
type rig struct {
	t     *testing.T
	fs    *fake.FS
	shell *fake.Shell
	clip  *fake.Clipboard
	set   Set
}

func newRig(t *testing.T) *rig {
	t.Helper()
	r := &rig{t: t, fs: fake.NewFS(), shell: fake.NewShell(), clip: fake.NewClipboard()}
	r.fs.Seed(filepath.Join(home, "docs", "notes.txt"), []byte("first line\nsecond line\n"))
	r.fs.Seed(filepath.Join(home, "docs", "photo.png"), []byte{0x89, 0x50, 0x00, 0x4e})
	r.fs.Seed(filepath.Join(home, ".ssh", "id_ed25519"), []byte("PRIVATE KEY"))
	r.fs.Seed(filepath.Join(home, "project", ".env"), []byte("TOKEN=secret"))
	r.fs.Seed(filepath.Join(home, "project", "readme.md"), []byte("# project"))
	r.fs.Seed(testpath.Abs("etc", "hosts"), []byte("127.0.0.1 localhost"))
	r.shell.SetApps([]platform.App{
		{Name: "Calculator", ID: "calc", Path: "/Applications/Calculator.app"},
		{Name: "Calendar", ID: "cal", Path: "/Applications/Calendar.app"},
		{Name: "Safari", ID: "safari", Path: "/Applications/Safari.app"},
	}, nil)
	set, err := Standard(Deps{
		Home: home, FS: r.fs, Shell: r.shell, Clipboard: r.clip,
		Canonicalize: func(path string) (string, error) {
			if r.fs == nil {
				return path, nil
			}
			if _, err := r.fs.Stat(path); err != nil {
				return "", err
			}
			return path, nil
		},
	})
	if err != nil {
		t.Fatalf("Standard: %v", err)
	}
	r.set = set
	return r
}

// run calls a tool by name, after the policy, the way the loop does, and returns its result.
func (r *rig) run(name, arguments string) (agent.Result, error) {
	r.t.Helper()
	tool, ok := r.set.Tools.Lookup(name)
	if !ok {
		r.t.Fatalf("no tool %q", name)
	}
	if decision := r.set.Policy.Decide(backend.ToolCall{ID: "c", Function: backend.ToolFunction{Name: name, Arguments: arguments}}, tool); !decision.Allow {
		return agent.Result{}, &refusal{decision.Reason}
	}
	return tool.Run(context.Background(), json.RawMessage(arguments))
}

// refusal is what a run returns for a call the policy refused.
type refusal struct{ reason string }

func (r *refusal) Error() string { return "refused: " + r.reason }

// TestStandardRegistersEverySchemaAndWiresIt checks the contract the model sees: unique names, valid schemas, and a
// tools array that encodes.
func TestStandardRegistersEverySchemaAndWiresIt(t *testing.T) {
	r := newRig(t)
	wire, err := r.set.Tools.Wire()
	if err != nil {
		t.Fatalf("Wire: %v", err)
	}
	var decoded []struct {
		Function struct {
			Name       string          `json:"name"`
			Parameters json.RawMessage `json:"parameters"`
		} `json:"function"`
	}
	if err := json.Unmarshal(wire, &decoded); err != nil {
		t.Fatalf("wire is not JSON: %v", err)
	}
	names := make([]string, 0, len(decoded))
	for _, entry := range decoded {
		if !json.Valid(entry.Function.Parameters) {
			t.Fatalf("tool %s has an invalid schema", entry.Function.Name)
		}
		names = append(names, entry.Function.Name)
	}
	want := "search_files read_file list_folder list_apps open_app open_url read_clipboard copy_text trash_file"
	if got := strings.Join(names, " "); got != want {
		t.Fatalf("tools = %q, want %q", got, want)
	}
}

// TestDestructiveAndPrivateToolsAskFirst checks that only the tools that change or expose something are confirmed.
func TestDestructiveAndPrivateToolsAskFirst(t *testing.T) {
	r := newRig(t)
	for _, name := range []string{"trash_file", "read_clipboard"} {
		tool, _ := r.set.Tools.Lookup(name)
		if !tool.Confirm {
			t.Fatalf("%s does not ask first", name)
		}
	}
	for _, name := range []string{"search_files", "read_file", "list_folder", "list_apps", "open_app", "open_url", "copy_text"} {
		tool, _ := r.set.Tools.Lookup(name)
		if tool.Confirm {
			t.Fatalf("%s asks first but should not", name)
		}
	}
}

// TestReadFileTable checks the file reader: text, a cut file, and the refusals.
func TestReadFileTable(t *testing.T) {
	r := newRig(t)
	result, err := r.run("read_file", `{"path":"~/docs/notes.txt"}`)
	if err != nil || result.Text != "first line\nsecond line\n" || result.Outcome != "Read notes.txt (1 KB)" {
		t.Fatalf("read notes = %+v, %v", result, err)
	}
	if _, err := r.run("read_file", `{"path":"~/docs/photo.png"}`); err == nil || !strings.Contains(err.Error(), "not text") {
		t.Fatalf("binary file read: %v", err)
	}
	if _, err := r.run("read_file", `{"path":"~/docs"}`); err == nil || !strings.Contains(err.Error(), "list_folder") {
		t.Fatalf("folder read: %v", err)
	}
	if err := refusedError(r, "read_file", `{"path":"~/.ssh/id_ed25519"}`); err == "" {
		t.Fatal("a credential file was read")
	}
	if err := refusedError(r, "read_file", `{"path":"/etc/hosts"}`); err == "" {
		t.Fatal("a file outside home was read")
	}
	if err := refusedError(r, "read_file", `{"path":"~/project/.env"}`); err == "" {
		t.Fatal("a dotenv secret was read")
	}
}

// TestReadFileCutsLongFiles checks the byte bound.
func TestReadFileCutsLongFiles(t *testing.T) {
	r := newRig(t)
	r.fs.Seed(filepath.Join(home, "big.txt"), []byte(strings.Repeat("x", maxReadBytes+10)))
	result, err := r.run("read_file", `{"path":"~/big.txt"}`)
	if err != nil || !strings.Contains(result.Text, "[file cut: showed 16384 of 16394 bytes]") {
		t.Fatalf("big file = %v, %v", result.Outcome, err)
	}
}

// TestListFolderHidesCredentials checks that the listing never names a credential entry.
func TestListFolderHidesCredentials(t *testing.T) {
	r := newRig(t)
	result, err := r.run("list_folder", `{}`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result.Text, ".ssh") || !strings.Contains(result.Text, "docs/") {
		t.Fatalf("home listing = %q", result.Text)
	}
	result, err = r.run("list_folder", `{"path":"~/project"}`)
	if err != nil || strings.Contains(result.Text, ".env") || !strings.Contains(result.Text, "readme.md") {
		t.Fatalf("project listing = %q, %v", result.Text, err)
	}
	if err := refusedError(r, "list_folder", `{"path":"~/.ssh"}`); err == "" {
		t.Fatal("the .ssh folder was listed")
	}
}

// TestSearchFilesReturnsOnlyAllowedPaths checks the search: a match in docs is found, a key in .ssh is not.
func TestSearchFilesReturnsOnlyAllowedPaths(t *testing.T) {
	r := newRig(t)
	result, err := r.run("search_files", `{"query":"id_ed"}`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result.Text, ".ssh") || result.Outcome != "No matches" {
		t.Fatalf("search for a credential = %q (%s)", result.Text, result.Outcome)
	}
	result, err = r.run("search_files", `{"query":"notes"}`)
	if err != nil || !strings.Contains(result.Text, "notes.txt") || !strings.HasPrefix(result.Outcome, "Found ") {
		t.Fatalf("search for notes = %q (%s), %v", result.Text, result.Outcome, err)
	}
}

// TestOpenAppMatchesExactlyOrOnce checks the name rules: exact, one partial match, and an ambiguous or missing name.
func TestOpenAppMatchesExactlyOrOnce(t *testing.T) {
	r := newRig(t)
	if _, err := r.run("open_app", `{"name":"safari"}`); err != nil {
		t.Fatalf("exact name: %v", err)
	}
	if _, err := r.run("open_app", `{"name":"calc"}`); err != nil {
		t.Fatalf("one partial match: %v", err)
	}
	if _, err := r.run("open_app", `{"name":"cal"}`); err == nil || !strings.Contains(err.Error(), "more than one") {
		t.Fatalf("ambiguous name: %v", err)
	}
	if _, err := r.run("open_app", `{"name":"Photoshop"}`); err == nil {
		t.Fatal("a missing app was opened")
	}
	launched := 0
	for _, call := range r.shell.Calls() {
		if call.Op == "LaunchApp" {
			launched++
		}
	}
	if launched != 2 {
		t.Fatalf("LaunchApp calls = %d, want 2", launched)
	}
}

// TestOpenURLAllowsOnlyWebAddresses checks the scheme rule.
func TestOpenURLAllowsOnlyWebAddresses(t *testing.T) {
	r := newRig(t)
	if _, err := r.run("open_url", `{"url":"https://example.com/a"}`); err != nil {
		t.Fatalf("https: %v", err)
	}
	for _, raw := range []string{"javascript:alert(1)", "file:///etc/hosts", "https://", "not a url"} {
		arguments, _ := json.Marshal(map[string]string{"url": raw})
		if _, err := r.run("open_url", string(arguments)); err == nil {
			t.Fatalf("open_url(%q) was allowed", raw)
		}
	}
}

// TestClipboardTools checks the copy and the read, including the empty clipboard.
func TestClipboardTools(t *testing.T) {
	r := newRig(t)
	if result, err := r.run("read_clipboard", `{}`); err != nil || result.Outcome != "Clipboard has no text" {
		t.Fatalf("empty read = %+v, %v", result, err)
	}
	if _, err := r.run("copy_text", `{"text":"hello"}`); err != nil {
		t.Fatal(err)
	}
	result, err := r.run("read_clipboard", `{}`)
	if err != nil || result.Text != "hello" || result.Outcome != "Read 5 characters" {
		t.Fatalf("read after copy = %+v, %v", result, err)
	}
}

// TestTrashFileGoesThroughTheShellAndRefusesHome checks that trashing never deletes and never takes the home folder.
func TestTrashFileGoesThroughTheShellAndRefusesHome(t *testing.T) {
	r := newRig(t)
	if _, err := r.run("trash_file", `{"path":"~/docs/notes.txt"}`); err != nil {
		t.Fatalf("trash a file: %v", err)
	}
	trashed := false
	for _, call := range r.shell.Calls() {
		trashed = trashed || (call.Op == "Trash" && call.Arg == filepath.Join(home, "docs", "notes.txt"))
	}
	if !trashed {
		t.Fatalf("the shell was not asked to trash the file: %+v", r.shell.Calls())
	}
	if _, err := r.run("trash_file", `{"path":"~"}`); err == nil {
		t.Fatal("the home folder was trashed")
	}
	if err := refusedError(r, "trash_file", `{"path":"~/.ssh/id_ed25519"}`); err == "" {
		t.Fatal("a credential was trashed")
	}
}

// TestPathPolicyRefusesWhatTheGuardRefuses checks the policy table for path arguments.
func TestPathPolicyRefusesWhatTheGuardRefuses(t *testing.T) {
	r := newRig(t)
	read, _ := r.set.Tools.Lookup("read_file")
	tests := []struct {
		name      string
		arguments string
		allow     bool
	}{
		{"allowed path", `{"path":"~/docs/notes.txt"}`, true},
		{"credential path", `{"path":"~/.ssh/id_ed25519"}`, false},
		{"outside home", `{"path":"/etc/hosts"}`, false},
		{"path is a number", `{"path":3}`, false},
		{"arguments are not JSON", `{"path":`, false},
		{"path argument missing (checked by the tool)", `{}`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			call := backend.ToolCall{Function: backend.ToolFunction{Name: "read_file", Arguments: tt.arguments}}
			if got := r.set.Policy.Decide(call, read).Allow; got != tt.allow {
				t.Fatalf("policy allow = %v, want %v", got, tt.allow)
			}
		})
	}
}

// refusedError runs a tool call that should be refused, and returns the refusal text, or "" when it ran.
func refusedError(r *rig, name, arguments string) string {
	_, err := r.run(name, arguments)
	if err == nil {
		return ""
	}
	return err.Error()
}
