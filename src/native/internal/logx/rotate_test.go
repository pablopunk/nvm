package logx

import (
	"fmt"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// TestRotationKeepsOnlyTheConfiguredNumberOfFiles logs enough error lines (each flushes at once) to rotate
// several times with a small limit, then checks the file set, the size bound and the newest line.
func TestRotationKeepsOnlyTheConfiguredNumberOfFiles(t *testing.T) {
	logger, files, _ := newTestLogger(t, Options{MaxBytes: 400, MaxFiles: 3})
	for i := range 30 {
		logger.Error(fmt.Sprintf("line %02d", i), nil)
	}
	if err := logger.Flush(); err != nil {
		t.Fatal(err)
	}

	present := map[string]bool{}
	for _, name := range files.Paths() {
		if strings.HasPrefix(name, logDir+"/nvm.log") {
			present[name] = true
		}
	}
	for _, want := range []string{logDir + "/nvm.log", logDir + "/nvm.log.1", logDir + "/nvm.log.2"} {
		if !present[want] {
			t.Fatalf("missing %s; present: %v", want, present)
		}
	}
	if present[logDir+"/nvm.log.3"] {
		t.Fatal("kept more than MaxFiles files")
	}
	for name := range present {
		data, err := files.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if int64(len(data)) > 400 {
			t.Fatalf("%s is %d bytes, above MaxBytes", name, len(data))
		}
	}
	if !strings.Contains(string(activeFile(t, files)), "line 29") {
		t.Fatal("the active file does not hold the newest line")
	}
}

func TestSingleFileModeReplacesTheActiveFile(t *testing.T) {
	logger, files, _ := newTestLogger(t, Options{MaxBytes: 150, MaxFiles: 1})
	for i := range 6 {
		logger.Error(fmt.Sprintf("replace %d", i), nil)
	}
	if err := logger.Flush(); err != nil {
		t.Fatal(err)
	}
	for _, name := range files.Paths() {
		if strings.HasSuffix(name, ".1") {
			t.Fatalf("MaxFiles 1 kept a rotated copy: %s", name)
		}
	}
	if !strings.Contains(string(activeFile(t, files)), "replace 5") {
		t.Fatal("active file lost the newest entry")
	}
}

func TestReopeningContinuesTheActiveFile(t *testing.T) {
	files := fake.NewFS()
	clock := fake.NewClock()
	first, err := New(files, clock, logDir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	first.Info("before restart", nil)
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second, err := New(files, clock, logDir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	second.Info("after restart", nil)
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
	content := string(activeFile(t, files))
	if !strings.Contains(content, "before restart") || !strings.Contains(content, "after restart") {
		t.Fatalf("reopen did not keep both entries:\n%s", content)
	}
}
