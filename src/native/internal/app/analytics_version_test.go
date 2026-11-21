package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalyticsVersionUsesPackagedConfigVersion(t *testing.T) {
	if analyticsAppVersion != nativeClientVersion {
		t.Fatalf("development analytics version = %q, want native client fallback %q", analyticsAppVersion, nativeClientVersion)
	}
	script := filepath.Join("..", "..", "scripts", "packaging", "release-ldflags.sh")
	temp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(temp, "scripts", "packaging"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(temp, "scripts"), 0755); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(script)
	if err != nil {
		t.Fatal(err)
	}
	fixtureScript := filepath.Join(temp, "scripts", "packaging", "release-ldflags.sh")
	if err := os.WriteFile(fixtureScript, contents, 0755); err != nil {
		t.Fatal(err)
	}
	version := "2.0.0-next.0"
	config := `{"version":"` + version + `"}`
	if err := os.WriteFile(filepath.Join(temp, "mygo.json"), []byte(config), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", filepath.ToSlash(fixtureScript))
	output, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	flag := "-X github.com/pablopunk/nvm/native/internal/app.analyticsAppVersion=" + version
	if !strings.Contains(string(output), flag) {
		t.Fatalf("ldflags %q missing %q", output, flag)
	}
	if strings.Contains(flag, nativeClientVersion) {
		t.Fatalf("packaged analytics version unexpectedly uses backend compatibility version %q", nativeClientVersion)
	}
}

func TestStampedPackageVersionFlowsToReleaseFlags(t *testing.T) {
	temp := t.TempDir()
	root := filepath.Join(temp, "native")
	if err := os.MkdirAll(filepath.Join(root, "scripts", "packaging"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"stamp-mygo-config.sh", "release-ldflags.sh"} {
		contents, err := os.ReadFile(filepath.Join("..", "..", "scripts", "packaging", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "scripts", "packaging", name), contents, 0755); err != nil {
			t.Fatal(err)
		}
	}
	config, err := os.ReadFile(filepath.Join("..", "..", "mygo.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "mygo.json"), config, 0644); err != nil {
		t.Fatal(err)
	}
	version := "2.0.0-custom.7"
	stamp := exec.Command("bash", filepath.ToSlash(filepath.Join(root, "scripts", "packaging", "stamp-mygo-config.sh")))
	stamp.Dir = root
	stamp.Env = append(os.Environ(), "VERSION="+version, "RELEASE=false")
	if output, err := stamp.CombinedOutput(); err != nil {
		t.Fatalf("stamp config: %v\n%s", err, output)
	}
	for _, release := range []string{"false", "true"} {
		cmd := exec.Command("bash", filepath.ToSlash(filepath.Join(root, "scripts", "packaging", "release-ldflags.sh")))
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "RELEASE="+release)
		output, err := cmd.Output()
		if err != nil {
			t.Fatalf("release=%s: %v", release, err)
		}
		flags := string(output)
		for _, want := range []string{"isPackagedBuild=true", "analyticsAppVersion=" + version} {
			if !strings.Contains(flags, want) {
				t.Fatalf("release=%s flags %q missing %q", release, flags, want)
			}
		}
		if strings.Contains(flags, "releaseBuild=") != (release == "true") {
			t.Fatalf("release=%s flags %q have incorrect releaseBuild", release, flags)
		}
	}
}

func TestReleaseLDFLagsRejectMalformedVersion(t *testing.T) {
	script := filepath.ToSlash(filepath.Join("..", "..", "scripts", "packaging", "release-ldflags.sh"))
	for _, version := range []string{"2.0.0 next", "2.0.0\t", "2.0.0;touch", "2.0.0\\n-X evil=true", ""} {
		t.Run(version, func(t *testing.T) {
			temp := t.TempDir()
			if err := os.MkdirAll(filepath.Join(temp, "scripts", "packaging"), 0755); err != nil {
				t.Fatal(err)
			}
			contents, err := os.ReadFile(script)
			if err != nil {
				t.Fatal(err)
			}
			fixture := filepath.ToSlash(filepath.Join(temp, "scripts", "packaging", "release-ldflags.sh"))
			if err := os.WriteFile(fixture, contents, 0755); err != nil {
				t.Fatal(err)
			}
			data := `{"version":"` + version + `"}`
			if err := os.WriteFile(filepath.Join(temp, "mygo.json"), []byte(data), 0644); err != nil {
				t.Fatal(err)
			}
			if err := exec.Command("bash", fixture).Run(); err == nil {
				t.Fatal("expected malformed version rejection")
			}
		})
	}
}

func TestAnalyticsReleaseIdentity(t *testing.T) {
	for _, tc := range []struct{ os, want string }{
		{"darwin", "nevermind@2.0.0+0123456789abcdef0123456789abcdef01234567.darwin"},
		{"linux", "nevermind@2.0.0+0123456789abcdef0123456789abcdef01234567.linux"},
		{"windows", "nevermind@2.0.0+0123456789abcdef0123456789abcdef01234567.win32"},
	} {
		got := analyticsReleaseIdentity("2.0.0", "0123456789abcdef0123456789abcdef01234567", tc.os)
		if got != tc.want {
			t.Fatalf("identity(%s) = %q, want %q", tc.os, got, tc.want)
		}
	}
	if got := analyticsReleaseIdentity("2.0.0", "", "darwin"); got != "nevermind@2.0.0" {
		t.Fatalf("fallback = %q", got)
	}
}
