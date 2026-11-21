//go:build spike

package spike

import (
	"context"
	"runtime"
	"testing"
	"time"
)

// TestMeasureYaegiLoadAndCall reports load time and per-call latency for the trivial and the
// realistic Go extension. Run with: go test -tags spike -run TestMeasureYaegi -v -count=1
func TestMeasureYaegiLoadAndCall(t *testing.T) {
	cases := []struct {
		name   string
		source string
		query  string
	}{
		{"trivial (3 items)", TrivialGo, "one"},
		{"realistic (~500 lines)", RealisticGo(), "item-0"},
	}
	for _, c := range cases {
		load, err := medianDuration(10, func() error {
			_, err := loadYaegi(c.source)
			return err
		})
		requireNoError(t, err)

		ext, err := loadYaegi(c.source)
		requireNoError(t, err)
		items, err := ext.Search(c.query)
		requireNoError(t, err)

		for range 200 {
			if _, err := ext.Search(c.query); err != nil {
				t.Fatal(err)
			}
		}
		samples := make([]time.Duration, 0, 2000)
		for range 2000 {
			start := time.Now()
			if _, err := ext.Search(c.query); err != nil {
				t.Fatal(err)
			}
			samples = append(samples, time.Since(start))
		}
		p50, p95, p99 := summarize(samples)
		measureLine(t, "yaegi load=%s lines=%d load_ms=%.1f items=%d call_p50_us=%.1f call_p95_us=%.1f call_p99_us=%.1f",
			c.name, sourceLines(c.source), float64(load.Microseconds())/1e3, len(items), p50, p95, p99)
	}
}

// TestMeasureYaegiEvalWithContextOverhead compares the two call paths: holding a reflect.Value
// (fast, not interruptible) and EvalWithContext with a source expression (interruptible, but it
// re-parses the call and starts a goroutine per call).
func TestMeasureYaegiEvalWithContextOverhead(t *testing.T) {
	for _, c := range []struct {
		name   string
		source string
		query  string
	}{
		{"trivial", TrivialGo, "one"},
		{"realistic", RealisticGo(), "item-0"},
	} {
		ext, err := loadYaegi(c.source)
		requireNoError(t, err)
		expression := `Search("` + c.query + `")`
		ctx := context.Background()
		samples := make([]time.Duration, 0, 1000)
		for range 1000 {
			start := time.Now()
			if _, err := ext.interp.EvalWithContext(ctx, expression); err != nil {
				t.Fatal(err)
			}
			samples = append(samples, time.Since(start))
		}
		p50, p95, p99 := summarize(samples)
		measureLine(t, "yaegi EvalWithContext-call case=%s calls=1000 p50_us=%.1f p95_us=%.1f p99_us=%.1f", c.name, p50, p95, p99)
	}
}

// TestMeasureYaegiHeapPerExtension loads N extensions, keeps them alive and reports the heap
// growth per extension after two forced collections.
func TestMeasureYaegiHeapPerExtension(t *testing.T) {
	const n = 50
	cases := []struct {
		name   string
		source string
	}{
		{"trivial", TrivialGo},
		{"realistic", RealisticGo()},
	}
	for _, c := range cases {
		before := heapAlloc()
		exts := make([]*yaegiExt, 0, n)
		for range n {
			ext, err := loadYaegi(c.source)
			requireNoError(t, err)
			exts = append(exts, ext)
		}
		after := heapAlloc()
		perExt := float64(after-before) / float64(n) / 1024 / 1024
		measureLine(t, "yaegi heap case=%s extensions=%d heap_per_ext_mib=%.2f total_mib=%.1f",
			c.name, n, perExt, float64(after-before)/1024/1024)
		runtime.KeepAlive(exts)
	}
}

// TestYaegiStdlibImportMatrix tries to import a list of standard packages with the Yaegi symbol
// table and reports which ones fail. A failure means extensions cannot import the package.
func TestYaegiStdlibImportMatrix(t *testing.T) {
	packages := []string{
		"bufio", "bytes", "cmp", "context", "crypto/sha256", "encoding/base64", "encoding/csv",
		"encoding/hex", "encoding/json", "encoding/xml", "errors", "fmt", "hash/crc32", "html",
		"io", "io/fs", "iter", "log", "log/slog", "maps", "math", "math/big", "math/bits",
		"math/rand", "math/rand/v2", "net/http", "net/mail", "net/url", "os", "os/exec",
		"path", "path/filepath", "reflect", "regexp", "runtime", "slices", "sort", "strconv",
		"strings", "sync", "sync/atomic", "text/template", "time", "unicode", "unicode/utf8",
		"unique", "structs", "weak", "unsafe", "testing/synctest", "encoding/binary",
	}
	var failed []string
	for _, pkg := range packages {
		source := "package main\n\nimport _ \"" + pkg + "\"\n\nfunc Run() int { return 1 }\n"
		if _, err := evalValue(source, "Run()"); err != nil {
			failed = append(failed, pkg)
			measureLine(t, "stdlib import=%s status=FAIL reason=%q", pkg, firstLine(err.Error()))
			continue
		}
		measureLine(t, "stdlib import=%s status=ok", pkg)
	}
	measureLine(t, "stdlib summary tested=%d failed=%d failed_list=%v", len(packages), len(failed), failed)
}

func firstLine(text string) string {
	for i, r := range text {
		if r == '\n' {
			return text[:i]
		}
	}
	return text
}
