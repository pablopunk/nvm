package real

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
)

func TestNewPortsWiresEveryPort(t *testing.T) {
	ports := NewPorts()
	if ports.Clock == nil || ports.FS == nil || ports.HTTP == nil || ports.Clipboard == nil ||
		ports.Shell == nil || ports.Screens == nil || ports.Notifier == nil || ports.Keychain == nil ||
		ports.Processes == nil || ports.Helper == nil || ports.Hotkeys == nil || ports.Tray == nil ||
		ports.Autostart == nil {
		t.Fatalf("NewPorts left a port nil: %+v", ports)
	}
	if !ports.Capabilities.Supported(platform.CapabilityFileWatch) {
		t.Fatal("file watching must be supported on every OS")
	}
}

func TestProcessCapabilitiesMatchTheOperatingSystem(t *testing.T) {
	supported := Capabilities().Supported(platform.CapabilityProcessList)
	if supported != (runtime.GOOS == "linux") {
		t.Fatalf("processes.list supported=%v on %s", supported, runtime.GOOS)
	}
}

func TestFileRoundTripUsesTheOperatingSystem(t *testing.T) {
	dir := t.TempDir()
	files := osFS{}
	first := filepath.Join(dir, "first.txt")
	second := filepath.Join(dir, "second.txt")
	if err := files.WriteFile(first, []byte("hello"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := files.Rename(first, second); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	data, err := files.ReadFile(second)
	if err != nil || string(data) != "hello" {
		t.Fatalf("ReadFile = %q, %v", data, err)
	}
	if _, err := files.Stat(first); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Stat of renamed-away file: %v, want ErrNotExist", err)
	}
	if err := files.Remove(second); err != nil {
		t.Fatalf("Remove: %v", err)
	}
}

func TestConcealedClipboardMarkersAreRecognized(t *testing.T) {
	tests := []struct {
		name    string
		formats []string
		want    bool
	}{
		{name: "no formats", formats: nil, want: false},
		{name: "plain text", formats: []string{"text/plain"}, want: false},
		{name: "nspasteboard concealed type", formats: []string{"public.utf8-plain-text", "org.nspasteboard.ConcealedType"}, want: true},
		{name: "windows exclude from history", formats: []string{"ExcludeClipboardContentFromMonitorProcessing"}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isConcealedClipboard(tt.formats); got != tt.want {
				t.Fatalf("isConcealedClipboard(%v) = %v, want %v", tt.formats, got, tt.want)
			}
		})
	}
}
