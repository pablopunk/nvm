//go:build spike

package spike

import (
	"errors"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/grafana/sobek"
)

// sobekExt is one loaded JavaScript extension in its own sobek runtime.
type sobekExt struct {
	vm     *sobek.Runtime
	search sobek.Callable
}

func loadSobek(source string) (*sobekExt, error) {
	vm := sobek.New()
	if _, err := vm.RunString(source); err != nil {
		return nil, err
	}
	search, ok := sobek.AssertFunction(vm.Get("Search"))
	if !ok {
		return nil, errors.New("Search is not a function")
	}
	return &sobekExt{vm: vm, search: search}, nil
}

// call runs Search and returns the number of rows, exporting every row back to Go.
func (e *sobekExt) call(query string) (int, error) {
	value, err := e.search(sobek.Undefined(), e.vm.ToValue(query))
	if err != nil {
		return 0, err
	}
	rows, ok := value.Export().([]any)
	if !ok {
		return 0, errors.New("Search did not return an array")
	}
	return len(rows), nil
}

// TestMeasureSobekLoadAndCall mirrors TestMeasureYaegiLoadAndCall for the JavaScript runtime.
func TestMeasureSobekLoadAndCall(t *testing.T) {
	cases := []struct {
		name   string
		source string
		query  string
	}{
		{"trivial (3 items)", TrivialJS, "one"},
		{"realistic (~500 lines)", RealisticJS(), "item-0"},
	}
	for _, c := range cases {
		load, err := medianDuration(10, func() error {
			_, err := loadSobek(c.source)
			return err
		})
		requireNoError(t, err)
		ext, err := loadSobek(c.source)
		requireNoError(t, err)
		rows, err := ext.call(c.query)
		requireNoError(t, err)
		for range 200 {
			if _, err := ext.call(c.query); err != nil {
				t.Fatal(err)
			}
		}
		samples := make([]time.Duration, 0, 2000)
		for range 2000 {
			start := time.Now()
			if _, err := ext.call(c.query); err != nil {
				t.Fatal(err)
			}
			samples = append(samples, time.Since(start))
		}
		p50, p95, p99 := summarize(samples)
		measureLine(t, "sobek load=%s lines=%d load_ms=%.1f rows=%d call_p50_us=%.1f call_p95_us=%.1f call_p99_us=%.1f",
			c.name, sourceLines(c.source), float64(load.Microseconds())/1e3, rows, p50, p95, p99)
	}
}

// TestMeasureSobekHeapPerExtension keeps 50 runtimes alive and reports heap growth per runtime.
func TestMeasureSobekHeapPerExtension(t *testing.T) {
	const n = 50
	cases := []struct {
		name   string
		source string
	}{
		{"trivial", TrivialJS},
		{"realistic", RealisticJS()},
	}
	for _, c := range cases {
		before := heapAlloc()
		exts := make([]*sobekExt, 0, n)
		for range n {
			ext, err := loadSobek(c.source)
			requireNoError(t, err)
			exts = append(exts, ext)
		}
		after := heapAlloc()
		measureLine(t, "sobek heap case=%s runtimes=%d heap_per_runtime_mib=%.2f total_mib=%.1f",
			c.name, n, float64(after-before)/float64(n)/1024/1024, float64(after-before)/1024/1024)
		runtime.KeepAlive(exts)
	}
}

// TestSobekInterruptBusyLoops: sobek's Interrupt is asynchronous and honoured inside the
// interpreter loop, so a host timer can stop a runaway script on the same runtime.
func TestSobekInterruptBusyLoops(t *testing.T) {
	cases := map[string]string{
		"while(true) loop": `while (true) {}`,
		"10 s busy loop":   `var s = Date.now(); while (Date.now() - s < 10000) {}`,
	}
	for _, name := range []string{"while(true) loop", "10 s busy loop"} {
		vm := sobek.New()
		timer := time.AfterFunc(200*time.Millisecond, func() { vm.Interrupt("budget exceeded") })
		start := time.Now()
		_, err := vm.RunString(cases[name])
		elapsed := time.Since(start)
		timer.Stop()
		var interrupted *sobek.InterruptedError
		measureLine(t, "interrupt sobek case=%q returned_ms=%.1f interrupted=%v err=%q",
			name, float64(elapsed.Microseconds())/1e3, errors.As(err, &interrupted), firstLine(errString(err)))
	}
}

// TestSobekMemoryBombIsNotCapped allocates 64 MiB of typed arrays inside a runtime.
func TestSobekMemoryBombIsNotCapped(t *testing.T) {
	source := `var keep = []; for (var i = 0; i < 64; i++) keep.push(new Uint8Array(1 << 20)); keep.length;`
	vm := sobek.New()
	before := totalAllocated()
	value, err := vm.RunString(source)
	requireNoError(t, err)
	allocated := totalAllocated() - before
	measureLine(t, "memory sobek bomb=64MiB returned=%v allocated_mib=%.1f cap=none",
		value.Export(), float64(allocated)/1024/1024)
	runtime.KeepAlive(vm)
}

// TestSobekPanicsAndThrows checks what a JavaScript throw and a host-side Go panic look like.
func TestSobekPanicsAndThrows(t *testing.T) {
	vm := sobek.New()
	_, err := vm.RunString(`throw new Error("js failure")`)
	measureLine(t, "panic sobek js-throw surfaced=%v message=%q", err != nil, firstLine(errString(err)))

	vm.Set("hostPanic", func() { panic("host callback panic") })
	var recovered any
	func() {
		defer func() { recovered = recover() }()
		_, _ = vm.RunString(`hostPanic()`)
	}()
	measureLine(t, "panic sobek host-callback-panic host_recovered=%v (sobek propagates Go panics to the caller)", recovered != nil)
}

// TestSobekStdlibSurface reports what the JavaScript runtime provides without host help.
func TestSobekStdlibSurface(t *testing.T) {
	vm := sobek.New()
	names := []string{"JSON", "Date", "Intl", "Map", "Set", "Promise", "Math", "RegExp", "fetch", "require", "process", "setTimeout"}
	var present []string
	var missing []string
	for _, name := range names {
		if sobek.IsUndefined(vm.Get(name)) {
			missing = append(missing, name)
		} else {
			present = append(present, name)
		}
	}
	slices.Sort(present)
	measureLine(t, "sobek globals present=%v missing=%v", present, missing)
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
