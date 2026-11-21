package swifthelper

import (
	"reflect"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
)

func TestSetFrameArgsMatchTheHelperContract(t *testing.T) {
	full := SetFrameArgs(platform.WindowWrite{PID: 42, Key: "42:7", Frame: platform.WindowFrame{X: 8, Y: 8.5, Width: 490, Height: 784}})
	if want := []string{"set-frame", "42", "42:7", "8", "8.5", "490", "784"}; !reflect.DeepEqual(full, want) {
		t.Fatalf("SetFrameArgs = %v, want %v", full, want)
	}
	position := SetFrameArgs(platform.WindowWrite{PID: 42, Key: "42:7", Frame: platform.WindowFrame{X: 0, Y: 0}, PositionOnly: true})
	if want := []string{"set-frame", "42", "42:7", "0", "0", PositionOnlyFlag}; !reflect.DeepEqual(position, want) {
		t.Fatalf("position-only SetFrameArgs = %v, want %v", position, want)
	}
}

func TestOtherSubcommandArgsMatchTheHelperContract(t *testing.T) {
	if got := SetFullScreenArgs(42, "42:7", true); !reflect.DeepEqual(got, []string{"set-fullscreen", "42", "42:7", "true"}) {
		t.Fatalf("SetFullScreenArgs = %v", got)
	}
	if got := RaiseArgs(42, "42:7"); !reflect.DeepEqual(got, []string{"raise", "42", "42:7"}) {
		t.Fatalf("RaiseArgs = %v", got)
	}
	if got := ListArgs("focused", 42); !reflect.DeepEqual(got, []string{"focused", "42"}) {
		t.Fatalf("ListArgs = %v", got)
	}
}
