package services

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"testing"
)

type meteredEngine struct {
	fakeDictationEngine
	sink func(float64, string, func() bool)
}

func (e *meteredEngine) StartWithLevel(options api.DictationStartOptions, sink func(float64, string, func() bool)) error {
	e.sink = sink
	return e.Start(options)
}

type meteredSurface struct {
	recordingIndicators
	owner, id, operation string
	level                float64
	current              func() bool
}

func (s *meteredSurface) ReportRecordingLevel(owner, id, operation string, level float64, current func() bool) {
	s.owner, s.id, s.operation, s.level, s.current = owner, id, operation, level, current
}

func TestDictationMeterUsesCallingExtensionIdentity(t *testing.T) {
	engine := &meteredEngine{}
	surface := &meteredSurface{}
	h := dictationHandler(t, engine, api.CapabilityDictation)
	h.deps.Indicators = surface
	reply := dictationCall(t, h, protocol.OpStart, api.DictationStartOptions{IndicatorID: "my-meter"})
	if reply.Failure != nil || engine.sink == nil {
		t.Fatalf("meter binding failed: %+v", reply)
	}
	live := true
	engine.sink(0.7, "recording-one", func() bool { return live })
	if surface.owner != "acme.dictation" || surface.id != "my-meter" || surface.operation != "recording-one" || surface.level != 0.7 || !surface.current() {
		t.Fatalf("wrong meter route: %+v", surface)
	}
	live = false
	if surface.current() {
		t.Fatal("surface lost the operation lifetime")
	}
}

func TestDictationMeterCannotBypassPermissionOrMissingSurface(t *testing.T) {
	for _, permission := range []bool{false, true} {
		engine := &meteredEngine{}
		var capabilities []api.Capability
		if permission {
			capabilities = append(capabilities, api.CapabilityDictation)
		}
		h := dictationHandler(t, engine, capabilities...)
		reply := dictationCall(t, h, protocol.OpStart, api.DictationStartOptions{IndicatorID: "meter"})
		if reply.Failure == nil || engine.sink != nil || len(engine.started) != 0 {
			t.Fatalf("unavailable binding started a microphone: %+v", reply)
		}
	}
}
