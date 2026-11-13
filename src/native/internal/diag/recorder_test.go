package diag

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func newRecorderForTest() *Recorder {
	return NewRecorder(fake.NewClock(), logx.NewRedactor())
}

func TestRecordKeepsReferenceComponentTimeAndMessage(t *testing.T) {
	recorder := newRecorderForTest()
	ref := recorder.Record("clipboard.poll", errors.New("pasteboard busy"), map[string]string{"operation": "poll"})

	record, ok := recorder.Find(ref)
	if !ok {
		t.Fatalf("Find(%q) found nothing", ref)
	}
	if record.Component != "clipboard.poll" || record.Message != "pasteboard busy" {
		t.Fatalf("record = %+v", record)
	}
	if record.Time.Format(time.RFC3339) != "2026-01-01T09:00:00Z" {
		t.Fatalf("time = %v, want the fake clock epoch", record.Time)
	}
	if record.Attributes["operation"] != "poll" {
		t.Fatalf("attributes = %v", record.Attributes)
	}
}

func TestRecorderKeepsOnlyTheNewest200Records(t *testing.T) {
	recorder := newRecorderForTest()
	first := recorder.Record("c", errors.New("0"), nil)
	for i := 1; i <= RecordCapacity+4; i++ {
		recorder.Record("c", fmt.Errorf("%d", i), nil)
	}
	recent := recorder.Recent()
	if len(recent) != RecordCapacity {
		t.Fatalf("kept %d records, want %d", len(recent), RecordCapacity)
	}
	if recent[0].Message != fmt.Sprint(RecordCapacity+4) {
		t.Fatalf("newest first = %q", recent[0].Message)
	}
	if _, ok := recorder.Find(first); ok {
		t.Fatal("the oldest record survived the cap")
	}
}

func TestAttributesOutsideTheAllowListAreDroppedAndCounted(t *testing.T) {
	recorder := newRecorderForTest()
	ref := recorder.Record("c", nil, map[string]string{
		"attempt":     "2",
		"query":       "my secret search",
		"file_name":   "tax.pdf",
		"outcome":     "failed",
		"nickname":    "pol",
		"stage":       "/home/pol/x",
		"operation":   "hello world",
		"duration_ms": "12",
	})
	record, _ := recorder.Find(ref)
	want := map[string]string{"attempt": "2", "outcome": "failed", "duration_ms": "12"}
	if !reflect.DeepEqual(record.Attributes, want) {
		t.Fatalf("attributes = %v, want %v", record.Attributes, want)
	}
	if record.Dropped != 5 {
		t.Fatalf("Dropped = %d, want 5 (query, file_name, nickname, stage, operation)", record.Dropped)
	}
}

func TestComponentMustBeACodeLabel(t *testing.T) {
	recorder := newRecorderForTest()
	ref := recorder.Record("/home/pol/my notes.txt", nil, nil)
	record, _ := recorder.Find(ref)
	if record.Component != "invalid" {
		t.Fatalf("component = %q, want invalid", record.Component)
	}
}

func TestExportHoldsOnlyTheRecordAndUnknownReferencesExportNothing(t *testing.T) {
	recorder := newRecorderForTest()
	ref := recorder.Record("ai.stream", errors.New("stream closed"), map[string]string{"stage": "stream", "attempt": "1"})
	text := recorder.Export(ref)
	for _, want := range []string{"Reference: " + ref, "Component: ai.stream", "Error: stream closed", "Attributes: attempt=1 stage=stream"} {
		if !strings.Contains(text, want) {
			t.Fatalf("export lacks %q:\n%s", want, text)
		}
	}
	if recorder.Export("NVM-ZZZZZ") != "" {
		t.Fatal("an unknown reference exported text")
	}
}

func TestFramesKeepFunctionAndBaseNameOnly(t *testing.T) {
	stack := "goroutine 7 [running]:\n" +
		"runtime/debug.Stack()\n\t/usr/local/go/src/runtime/debug/stack.go:26 +0x5e\n" +
		"github.com/pablopunk/nvm/native/internal/safego.guard.func1()\n\t/home/pol/src/nvm/src/native/internal/safego/run.go:150 +0x3a\n" +
		"panic({0x1234, 0x5678})\n\t/usr/local/go/src/runtime/panic.go:785 +0x132\n" +
		"main.doWork(...)\n\t/home/pol/secret-project/work.go:12\n"
	got := stackFrames(stack)
	want := []string{"safego.guard.func1 run.go:150", "main.doWork work.go:12"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("frames = %q, want %q", got, want)
	}
}

// TestRecordSchemaHoldsNoContentFields is the analytics schema guard for WP1.12. Adding a field to Record
// fails here until the field is reviewed and listed, and the listed types cannot hold arbitrary content.
func TestRecordSchemaHoldsNoContentFields(t *testing.T) {
	allowed := map[string]reflect.Type{
		"Ref":        reflect.TypeOf(""),
		"Component":  reflect.TypeOf(""),
		"Time":       reflect.TypeOf(time.Time{}),
		"Message":    reflect.TypeOf(""),
		"Frames":     reflect.TypeOf([]string(nil)),
		"Attributes": reflect.TypeOf(map[string]string(nil)),
		"Dropped":    reflect.TypeOf(0),
	}
	typ := reflect.TypeOf(Record{})
	seen := map[string]bool{}
	for i := range typ.NumField() {
		field := typ.Field(i)
		if !field.IsExported() {
			t.Errorf("unexported field %s: a record must not hide state", field.Name)
			continue
		}
		want, ok := allowed[field.Name]
		if !ok {
			t.Errorf("field %s is not in the reviewed schema", field.Name)
			continue
		}
		if field.Type != want {
			t.Errorf("field %s has type %s, want %s", field.Name, field.Type, want)
		}
		seen[field.Name] = true
	}
	for name := range allowed {
		if !seen[name] {
			t.Errorf("reviewed field %s is missing from Record", name)
		}
	}
}

func TestCanaryNeverReachesRecordsOrExports(t *testing.T) {
	recorder := newRecorderForTest()
	for _, err := range secretForms() {
		ref := recorder.Record("provider", err, map[string]string{"operation": "sync", "release": "1.2.3"})
		record, _ := recorder.Find(ref)
		if strings.Contains(record.Message, canary) || strings.Contains(recorder.Export(ref), canary) {
			t.Fatalf("canary survived into the record for %q: %q", err, record.Message)
		}
	}
}
