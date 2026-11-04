package child

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// refusingLink answers every host call with an unavailable failure, so the remote services can be walked without a
// host and without a nil link.
func refusingLink() *hostLink {
	return newDirectLink(func(protocol.ServiceCall) protocol.ServiceReply {
		return protocol.ServiceReply{Failure: &api.Failure{Code: api.CodeUnavailable, Message: "no host in this test"}}
	})
}

// everyService lists each accessor value the child hands an extension, so one test walks every method of each.
func everyService() map[string]any {
	link := refusingLink()
	return map[string]any{
		"ai":               unsupportedAI{},
		"attachments":      attachmentBuilder{},
		"dictation":        unsupportedDictation{},
		"aiBuilder":        unsupportedAIBuilder{},
		"ownership":        unsupportedOwnership{},
		"updates":          unsupportedUpdates{},
		"keyboard":         unsupportedKeyboard{},
		"selection":        unsupportedSelection{},
		"apps":             remoteApps{link: link},
		"files":            remoteFiles{link: link},
		"systemClipboard":  remoteSystemClipboard{link: link},
		"windowManagement": remoteWindowManagement{link: link},
		"windows":          childWindows{},
		"navigation":       childNavigation{},
	}
}

// zeroArgs returns zero values for every parameter of method, as an extension passing nothing useful would.
func zeroArgs(method reflect.Method) []reflect.Value {
	args := make([]reflect.Value, 0, method.Type.NumIn()-1)
	for i := 1; i < method.Type.NumIn(); i++ {
		args = append(args, reflect.Zero(method.Type.In(i)))
	}
	return args
}

// TestEveryServiceMethodAnswersWithoutPanicking calls every method of every accessor value. A panic fails the test. A
// method whose last result is an error must return nil or an error that wraps api.ErrNotAvailable, so an extension
// sees a clear refusal, never a silent empty answer that hides it.
func TestEveryServiceMethodAnswersWithoutPanicking(t *testing.T) {
	errorType := reflect.TypeOf((*error)(nil)).Elem()
	services := everyService()
	names := make([]string, 0, len(services))
	for name := range services {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		value := reflect.ValueOf(services[name])
		for i := 0; i < value.NumMethod(); i++ {
			method := value.Type().Method(i)
			t.Run(fmt.Sprintf("%s.%s", name, method.Name), func(t *testing.T) {
				results := callWithoutPanic(t, value.Method(i), zeroArgs(method))
				outs := method.Type.NumOut()
				if outs == 0 || method.Type.Out(outs-1) != errorType {
					return
				}
				err, _ := results[outs-1].Interface().(error)
				if err != nil && !errors.Is(err, api.ErrNotAvailable) {
					t.Fatalf("returned %v; want nil or an error wrapping api.ErrNotAvailable", err)
				}
			})
		}
	}
}

func callWithoutPanic(t *testing.T, method reflect.Value, args []reflect.Value) []reflect.Value {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("panicked: %v", recovered)
		}
	}()
	return method.Call(args)
}

// TestHostContextNeverHandsOutANilService fails when an accessor the child serves would return nil, which is what the
// embedded nil interfaces did before each accessor was wired.
func TestHostContextNeverHandsOutANilService(t *testing.T) {
	ctx := newHostContext(api.Identity{ID: "acme.test"}, nil, refusingLink())
	accessors := map[string]any{
		"AI": ctx.AI(), "AIBuilder": ctx.AIBuilder(), "Ownership": ctx.Ownership(), "Dictation": ctx.Dictation(),
		"Updates": ctx.Updates(), "Windows": ctx.Windows(), "Navigation": ctx.Navigation(), "Indicators": ctx.Indicators(),
		"Desktop": ctx.Desktop(), "Shell": ctx.Desktop().Shell(), "Keyboard": ctx.Desktop().Keyboard(),
		"Selection": ctx.Desktop().Selection(), "Files": ctx.Desktop().Files(), "Apps": ctx.Desktop().Apps(),
		"SystemClipboard": ctx.Desktop().Clipboard(), "WindowManagement": ctx.Desktop().WindowManagement(),
	}
	for name, service := range accessors {
		if service == nil {
			t.Fatalf("%s is nil; want a service", name)
		}
	}
}
