package spike

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// The vectors below port ai-credential-path-guard.test.ts case by case. Each TS test name is kept in its Go name, and
// the TS symlink tests run against the in-memory resolver here and against the real file system in
// osresolver_test.go.

func resolvedIsCredential(t *testing.T, fixture guardFixture, raw, cwd string) bool {
	t.Helper()
	guard := NewGuard(GuardConfig{HomeDir: fixture.home}, fixture.fs)
	return IsCredentialPath(guard.Resolve(raw, cwd), CredentialPolicy{HomeDir: fixture.home})
}

func TestCredentialReadsAreDeniedInEverySpelling(t *testing.T) {
	fixture := newGuardFixture()
	keyPath := fixture.home + "/.ssh/id_ed25519"
	cases := []string{
		"~/.ssh/id_ed25519",
		"@~/.ssh/id_ed25519",
		"file://" + keyPath,
		keyPath,
		"../.ssh/id_ed25519",
	}
	for _, raw := range cases {
		if !resolvedIsCredential(t, fixture, raw, fixture.documents) {
			t.Errorf("%q should be a credential path", raw)
		}
	}
}

func TestSymlinksThatResolveIntoSSHAreDenied(t *testing.T) {
	fixture := newGuardFixture()
	fixture.fs.addLink(fixture.elsewhere+"/keys", fixture.home+"/.ssh")
	if !resolvedIsCredential(t, fixture, "keys/id_ed25519", fixture.elsewhere) {
		t.Fatal("a symlink into ~/.ssh must resolve to a credential path")
	}
}

func TestCredentialDirectoryNamesMatchCaseInsensitively(t *testing.T) {
	fixture := newGuardFixture()
	upper := fixture.home + "/.SSH/id_ed25519"
	if !IsCredentialPath(upper, CredentialPolicy{HomeDir: fixture.home}) {
		t.Fatalf("%q should match ~/.ssh case-insensitively", upper)
	}
}

func TestEnvFilesAreDeniedWherever(t *testing.T) {
	fixture := newGuardFixture()
	for _, name := range []string{".env", ".env.local", ".env.production"} {
		if !resolvedIsCredential(t, fixture, name, fixture.documents) {
			t.Errorf("%s should be denied", name)
		}
	}
}

func TestASymlinkedCredentialDirectoryIsStillDenied(t *testing.T) {
	fixture := newGuardFixture()
	fixture.fs.addDir("/fx/private-keys").addFile("/fx/private-keys/id_ed25519")
	fixture.fs.links[fixture.home+"/.ssh"] = "/fx/private-keys"
	guard := NewGuard(GuardConfig{HomeDir: fixture.home}, fixture.fs)
	policy := NewPathPolicy(guard, fixture.documents)

	decision := policy.Decide(ToolCall{Name: "read", Arguments: `{"path":"~/.ssh/id_ed25519"}`}, Tool{})

	if decision.Allow || decision.Reason != CredentialBlockReason {
		t.Fatalf("decision = %+v, want the credential block", decision)
	}
}

func TestASymlinkedEnvFileIsDeniedByRequestedName(t *testing.T) {
	fixture := newGuardFixture()
	fixture.fs.addFile("/fx/service-secrets")
	fixture.fs.addLink(fixture.documents+"/.env", "/fx/service-secrets")
	guard := NewGuard(GuardConfig{HomeDir: fixture.home}, fixture.fs)

	if decision := guard.Check(".env", fixture.documents); decision.Allow {
		t.Fatal("a .env symlink must be denied by the name the model asked for")
	}
}

func TestEnvTemplateFilesStayReadable(t *testing.T) {
	fixture := newGuardFixture()
	guard := NewGuard(GuardConfig{HomeDir: fixture.home}, fixture.fs)
	for _, name := range []string{".env.example", ".env.sample", ".env.template"} {
		if decision := guard.Check(name, fixture.documents); !decision.Allow {
			t.Errorf("%s is a template and must stay readable: %s", name, decision.Reason)
		}
	}
}

func TestOrdinaryNotesStayReadable(t *testing.T) {
	fixture := newGuardFixture()
	guard := NewGuard(GuardConfig{HomeDir: fixture.home}, fixture.fs)
	for _, raw := range []string{"~/Documents/notes.md", "notes.md"} {
		if decision := guard.Check(raw, fixture.documents); !decision.Allow {
			t.Errorf("%q must stay readable: %s", raw, decision.Reason)
		}
	}
}

func TestBrowserProfilesAndNevermindUserDataAreDeniedExceptTheAIWorkspace(t *testing.T) {
	fixture := newGuardFixture()
	userData := fixture.home + "/Library/Application Support/Nevermind"
	policy := CredentialPolicy{
		HomeDir:     fixture.home,
		ExtraDenied: []string{userData + "/pi-agent", userData},
		Allowed:     []string{userData + "/ai-workspace"},
	}
	chromeCookies := fixture.home + "/Library/Application Support/Google/Chrome/Default/Cookies"

	if !IsCredentialPath(chromeCookies, policy) {
		t.Error("browser cookies must be denied")
	}
	if !IsCredentialPath(userData+"/pi-agent/auth.json", policy) {
		t.Error("the agent directory must be denied")
	}
	if !IsCredentialPath(userData+"/byo-key.json", policy) {
		t.Error("byo-key.json must be denied")
	}
	if IsCredentialPath(userData+"/ai-workspace/notes.md", policy) {
		t.Error("the AI workspace must stay readable")
	}
}

func TestNevermindPrivatePathsCoverTheAgentDirectoryAndSecretFiles(t *testing.T) {
	got := NevermindPrivatePaths("/data", "/data/pi-agent")
	want := []string{
		"/data/pi-agent",
		"/data/nevermind-auth.json",
		"/data/nevermind-auth-by-origin.json",
		"/data/byo-key.json",
		"/data/extension-secrets",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NevermindPrivatePaths = %q, want %q", got, want)
	}
}

func TestToolCallGuardBlocksCredentialReadsAndLetsNormalReadsThrough(t *testing.T) {
	fixture := newGuardFixture()
	policy := NewPathPolicy(NewGuard(GuardConfig{HomeDir: fixture.home}, fixture.fs), fixture.documents)

	blocked := policy.Decide(ToolCall{Name: "read", Arguments: `{"path":"~/.ssh/id_ed25519"}`}, Tool{})
	if blocked.Allow || blocked.Reason != CredentialBlockReason {
		t.Fatalf("credential read decision = %+v", blocked)
	}
	if allowed := policy.Decide(ToolCall{Name: "read", Arguments: `{"path":"notes.md"}`}, Tool{}); !allowed.Allow {
		t.Fatalf("normal read refused: %s", allowed.Reason)
	}
}

func TestToolCallGuardBlocksSearchToolsOnlyWhenTheRootIsACredentialLocation(t *testing.T) {
	fixture := newGuardFixture()
	policy := NewPathPolicy(NewGuard(GuardConfig{HomeDir: fixture.home}, fixture.fs), fixture.documents)
	sshRoot := fixture.home + "/.ssh"

	for _, name := range []string{"grep", "find", "ls"} {
		args := `{"pattern":"x","path":"` + sshRoot + `"}`
		if decision := policy.Decide(ToolCall{Name: name, Arguments: args}, Tool{}); decision.Allow {
			t.Errorf("%s over ~/.ssh must be refused", name)
		}
	}
	home := `{"pattern":"x","path":"` + fixture.home + `"}`
	if decision := policy.Decide(ToolCall{Name: "grep", Arguments: home}, Tool{}); !decision.Allow {
		t.Fatalf("grep over home must run, refused: %s", decision.Reason)
	}
}

func TestToolResultDropsSearchLinesThatPointIntoCredentialLocations(t *testing.T) {
	fixture := newGuardFixture()
	policy := NewPathPolicy(NewGuard(GuardConfig{HomeDir: fixture.home}, fixture.fs), fixture.home)

	grepText := strings.Join([]string{
		".ssh/id_ed25519:1: PRIVATE KEY",
		"Documents/notes.md:1: hello",
		"Documents/notes.md-2- context",
		".env:1: TOKEN=abc",
		"[100 matches limit reached]",
	}, "\n")
	grepCall := ToolCall{Name: "grep", Arguments: `{"pattern":"x","path":"` + fixture.home + `"}`}
	wantGrep := "Documents/notes.md:1: hello\nDocuments/notes.md-2- context\n[100 matches limit reached]"
	if got := policy.FilterResult(grepCall, Tool{}, grepText); got != wantGrep {
		t.Fatalf("grep output\n got: %q\nwant: %q", got, wantGrep)
	}

	findCall := ToolCall{Name: "find", Arguments: `{"path":"` + fixture.home + `"}`}
	if got := policy.FilterResult(findCall, Tool{}, ".ssh/\n.ssh/id_ed25519\nDocuments/"); got != "Documents/" {
		t.Fatalf("find output = %q, want Documents/", got)
	}
	lsCall := ToolCall{Name: "ls", Arguments: `{"path":"` + fixture.home + `"}`}
	if got := policy.FilterResult(lsCall, Tool{}, ".ssh/\nDocuments/"); got != "Documents/" {
		t.Fatalf("ls output = %q, want Documents/", got)
	}
	clean := "Documents/notes.md:1: hello"
	if got := policy.FilterResult(grepCall, Tool{}, clean); got != clean {
		t.Fatalf("clean output changed to %q", got)
	}
}

// The TS suite asserts that garbage events fail open (allowed). The spike refuses them instead: a path it cannot
// read is not a path it may touch. This vector is the deliberate divergence.
func TestMalformedToolArgumentsAreRefusedNotAllowed(t *testing.T) {
	fixture := newGuardFixture()
	policy := NewPathPolicy(NewGuard(GuardConfig{HomeDir: fixture.home}, fixture.fs), fixture.documents)
	garbage := []string{
		`{"path":`,
		`{"path":{"nested":true}}`,
		`{"path":42}`,
		`"read"`,
	}
	for _, arguments := range garbage {
		if decision := policy.Decide(ToolCall{Name: "read", Arguments: arguments}, Tool{}); decision.Allow {
			t.Errorf("arguments %s must be refused", arguments)
		}
	}
	if got := policy.FilterResult(ToolCall{Name: "grep", Arguments: `{"path":42}`}, Tool{}, ".ssh/x:1: y"); got != withheldSearchOutput {
		t.Fatalf("unreadable search arguments must withhold output, got %q", got)
	}
}

func TestGrepCandidatesFollowTheLineNumberBoundary(t *testing.T) {
	got := grepLinePathCandidates("a:b:12: text")
	want := []string{"a:b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("candidates = %q, want %q", got, want)
	}
	if got := grepLinePathCandidates("name:x: y"); len(got) != 0 {
		t.Fatalf("a non-numeric boundary must not split the line, got %q", got)
	}
}

func TestRootsConfineResolvedPaths(t *testing.T) {
	fixture := newGuardFixture()
	guard := NewGuard(GuardConfig{HomeDir: fixture.home, Roots: []string{fixture.documents}}, fixture.fs)

	if decision := guard.Check("notes.md", fixture.documents); !decision.Allow {
		t.Fatalf("a file inside the root must be allowed: %s", decision.Reason)
	}
	if decision := guard.Check("../notes.md", fixture.documents); decision.Reason != OutsideRootReason {
		t.Fatalf("traversal out of the root = %+v, want the outside-root refusal", decision)
	}
	if decision := guard.Check("/fx/elsewhere/x", fixture.documents); decision.Reason != OutsideRootReason {
		t.Fatalf("an absolute path outside the root = %+v", decision)
	}
	if decision := guard.Check("../.ssh/id_ed25519", fixture.documents); decision.Reason != CredentialBlockReason {
		t.Fatalf("credentials are refused before the root check: %+v", decision)
	}
}

func TestSymlinkInsideTheRootThatPointsOutsideIsRefused(t *testing.T) {
	fixture := newGuardFixture()
	fixture.fs.addFile(fixture.elsewhere + "/secret.txt")
	fixture.fs.addLink(fixture.documents+"/out", fixture.elsewhere)
	guard := NewGuard(GuardConfig{HomeDir: fixture.home, Roots: []string{fixture.documents}}, fixture.fs)

	if decision := guard.Check("out/secret.txt", fixture.documents); decision.Reason != OutsideRootReason {
		t.Fatalf("a link out of the root = %+v, want the outside-root refusal", decision)
	}
}

func TestWithoutRootsAnyNonCredentialPathIsAllowed(t *testing.T) {
	fixture := newGuardFixture()
	guard := NewGuard(GuardConfig{HomeDir: fixture.home}, fixture.fs)
	if decision := guard.Check("/fx/elsewhere/x", fixture.documents); !decision.Allow {
		t.Fatalf("with no roots the 1.x behaviour allows any non-credential path: %s", decision.Reason)
	}
	if got := filepath.Clean(guard.Resolve("~/Documents/notes.md", fixture.documents)); got != fixture.documents+"/notes.md" {
		t.Fatalf("resolved = %q", got)
	}
}
