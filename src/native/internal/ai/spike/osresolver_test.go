package spike

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// The real-file-system versions of the TS symlink tests. They use t.TempDir and skip, as the TS tests do, when the
// platform refuses to create symlinks.

func realFixture(t *testing.T) (root, home, documents string) {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	home = filepath.Join(base, "home")
	documents = filepath.Join(home, "Documents")
	mustMkdir(t, filepath.Join(home, ".ssh"))
	mustMkdir(t, documents)
	mustWrite(t, filepath.Join(home, ".ssh", "id_ed25519"), "PRIVATE KEY")
	mustWrite(t, filepath.Join(documents, "notes.md"), "hello")
	return base, home, documents
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func symlinkOrSkip(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		if errors.Is(err, os.ErrPermission) {
			t.Skipf("symlinks are not permitted here: %v", err)
		}
		t.Fatalf("symlink: %v", err)
	}
}

func TestOSSymlinksThatResolveIntoSSHAreDenied(t *testing.T) {
	root, home, _ := realFixture(t)
	elsewhere := filepath.Join(root, "elsewhere")
	mustMkdir(t, elsewhere)
	symlinkOrSkip(t, filepath.Join(home, ".ssh"), filepath.Join(elsewhere, "keys"))
	guard := NewGuard(GuardConfig{HomeDir: home}, OSResolver{})

	if decision := guard.Check("keys/id_ed25519", elsewhere); decision.Allow {
		t.Fatal("a real symlink into ~/.ssh must be denied")
	}
}

func TestOSASymlinkedCredentialDirectoryIsStillDenied(t *testing.T) {
	root, home, documents := realFixture(t)
	realKeys := filepath.Join(root, "private-keys")
	mustMkdir(t, realKeys)
	mustWrite(t, filepath.Join(realKeys, "id_ed25519"), "PRIVATE KEY")
	if err := os.RemoveAll(filepath.Join(home, ".ssh")); err != nil {
		t.Fatalf("remove .ssh: %v", err)
	}
	symlinkOrSkip(t, realKeys, filepath.Join(home, ".ssh"))
	guard := NewGuard(GuardConfig{HomeDir: home}, OSResolver{})
	policy := NewPathPolicy(guard, documents)

	decision := policy.Decide(ToolCall{Name: "read", Arguments: `{"path":"~/.ssh/id_ed25519"}`}, Tool{})

	if decision.Allow {
		t.Fatal("a real symlinked credential directory must still be denied")
	}
}

func TestOSASymlinkedEnvFileIsDeniedByRequestedName(t *testing.T) {
	root, home, documents := realFixture(t)
	secrets := filepath.Join(root, "service-secrets")
	mustWrite(t, secrets, "TOKEN=abc")
	symlinkOrSkip(t, secrets, filepath.Join(documents, ".env"))
	guard := NewGuard(GuardConfig{HomeDir: home}, OSResolver{})

	if decision := guard.Check(".env", documents); decision.Allow {
		t.Fatal("a real .env symlink must be denied by the requested name")
	}
}

func TestOSOrdinaryNotesStayReadable(t *testing.T) {
	_, home, documents := realFixture(t)
	guard := NewGuard(GuardConfig{HomeDir: home}, OSResolver{})
	if decision := guard.Check("~/Documents/notes.md", documents); !decision.Allow {
		t.Fatalf("notes must stay readable: %s", decision.Reason)
	}
}
