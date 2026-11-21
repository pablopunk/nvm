package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const fixtureCommit = "0123456789abcdef0123456789abcdef01234567"

func releaseFlagsFixture(t *testing.T, git bool, env ...string) (string, error) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts", "packaging"), 0755); err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile(filepath.Join("..", "..", "scripts", "packaging", "release-ldflags.sh"))
	if err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(root, "scripts", "packaging", "release-ldflags.sh")
	if err := os.WriteFile(scriptPath, script, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "mygo.json"), []byte(`{"version":"2.0.0"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if git {
		for _, args := range [][]string{{"init"}, {"config", "user.email", "fixture@example.test"}, {"config", "user.name", "Fixture"}, {"add", "mygo.json"}, {"commit", "-m", "fixture"}} {
			cmd := exec.Command("git", args...)
			cmd.Dir = root
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %v: %v\n%s", args, err, out)
			}
		}
	}
	cmd := exec.Command("bash", filepath.ToSlash(scriptPath))
	cmd.Dir = root
	cmd.Env = append([]string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME"), "NEVERMIND_SOURCE_COMMIT=", "GITHUB_SHA=", "RELEASE=false"}, env...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestAnalyticsSourceCommitPrecedence(t *testing.T) {
	for _, tc := range []struct{ name, explicit, github, want string }{
		{"explicit", fixtureCommit, strings.Repeat("b", 40), fixtureCommit},
		{"github", "", strings.Repeat("c", 40), strings.Repeat("c", 40)},
		{"checkout", "", "", "checkout"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := releaseFlagsFixture(t, true, "NEVERMIND_SOURCE_COMMIT="+tc.explicit, "GITHUB_SHA="+tc.github)
			if err != nil {
				t.Fatal(err)
			}
			if tc.want == "checkout" {
				if !strings.Contains(out, "analyticsSourceCommit=") {
					t.Fatalf("checkout SHA missing: %s", out)
				}
			} else if !strings.Contains(out, "analyticsSourceCommit="+tc.want) {
				t.Fatalf("flags %q missing commit %q", out, tc.want)
			}
		})
	}
}

func TestAnalyticsSourceCommitMissingOutsideGitIsOmitted(t *testing.T) {
	out, err := releaseFlagsFixture(t, false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "analyticsSourceCommit=") {
		t.Fatalf("non-Git fixture emitted source commit: %s", out)
	}
}

func TestMalformedExplicitAnalyticsSourceCommitIsRejected(t *testing.T) {
	_, err := releaseFlagsFixture(t, true, "NEVERMIND_SOURCE_COMMIT=BAD", "GITHUB_SHA="+fixtureCommit)
	if err == nil {
		t.Fatal("malformed explicit SHA fell back instead of failing")
	}
}
