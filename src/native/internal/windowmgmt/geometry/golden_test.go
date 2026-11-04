package geometry

import "testing"

func TestGoldenPresetFramesMatchTheTypeScriptEngine(t *testing.T) {
	golden := loadGolden(t)
	if len(golden.PresetCases) < 1000 {
		t.Fatalf("golden has %d preset cases; regenerate it", len(golden.PresetCases))
	}
	for index, c := range golden.PresetCases {
		layout := mustPreset(t, PresetID(c.Preset))
		display := golden.Displays[c.Display].display()
		window := WindowFrame{Frame: c.Window.Frame.rect(), DisplayID: display.ID, IsResizable: c.Window.IsResizable}
		frame, _, err := Resolve(layout, display, window, golden.Options[c.Options].options(), nil)
		if err != nil {
			t.Fatalf("case %d (%s): %v", index, c.Preset, err)
		}
		if frame != c.Expected.rect() {
			t.Fatalf("case %d %s display %s resizable %v: got %+v, TypeScript says %+v", index, c.Preset, display.ID, window.IsResizable, frame, c.Expected.rect())
		}
	}
}

func TestGoldenExactFramesMatchTheTypeScriptEngine(t *testing.T) {
	golden := loadGolden(t)
	for index, c := range golden.ExactCases {
		display := golden.Displays[c.Display].display()
		window := WindowFrame{Frame: c.Window.Frame.rect(), DisplayID: display.ID, IsResizable: c.Window.IsResizable}
		frame, _, err := Resolve(c.Spec.layout(), display, window, golden.Options[c.Options].options(), nil)
		if err != nil {
			t.Fatalf("exact case %d: %v", index, err)
		}
		if frame != c.Expected.rect() {
			t.Fatalf("exact case %d (%+v): got %+v, TypeScript says %+v", index, c.Spec, frame, c.Expected.rect())
		}
	}
}

func TestGoldenDisplayPickMatchesTheTypeScriptEngine(t *testing.T) {
	golden := loadGolden(t)
	for index, c := range golden.PickCases {
		displays := make([]Display, 0, len(c.Displays))
		for _, d := range c.Displays {
			displays = append(displays, d.display())
		}
		picked, err := PickDisplayForFrame(displays, c.Frame.rect())
		if err != nil || picked.ID != c.Expected {
			t.Fatalf("pick case %d: got %q (%v), TypeScript says %q", index, picked.ID, err, c.Expected)
		}
	}
}

func TestGoldenAdjacentDisplayMatchesTheTypeScriptEngine(t *testing.T) {
	golden := loadGolden(t)
	for index, c := range golden.AdjacentCases {
		displays := make([]Display, 0, len(c.Displays))
		var from Display
		for _, d := range c.Displays {
			displays = append(displays, d.display())
			if d.ID == c.From {
				from = d.display()
			}
		}
		next, ok := AdjacentDisplay(displays, from, Direction(c.Direction), DisplayOrder(c.Order))
		if c.Expected == nil {
			if ok {
				t.Fatalf("adjacent case %d: got %q, TypeScript says none", index, next.ID)
			}
			continue
		}
		if !ok || next.ID != *c.Expected {
			t.Fatalf("adjacent case %d: got (%q, %v), TypeScript says %q", index, next.ID, ok, *c.Expected)
		}
	}
}

func TestGoldenDisplayMoveMatchesTheTypeScriptEngine(t *testing.T) {
	golden := loadGolden(t)
	for index, c := range golden.MoveCases {
		from := Display{ID: c.From.ID, Scale: 1, Frame: c.From.Frame.rect(), WorkArea: c.From.WorkArea.rect()}
		to := Display{ID: c.To.ID, Scale: 1, Frame: c.To.Frame.rect(), WorkArea: c.To.WorkArea.rect()}
		frame, err := MoveFrameToDisplay(c.Frame.rect(), from, to, DisplayMoveSizing(c.Sizing))
		if err != nil || frame != c.Expected.rect() {
			t.Fatalf("move case %d: got %+v (%v), TypeScript says %+v", index, frame, err, c.Expected.rect())
		}
	}
}

func TestGoldenCycleSequencesMatchTheTypeScriptEngine(t *testing.T) {
	golden := loadGolden(t)
	for index, c := range golden.CycleCases {
		layouts, err := CycleSequence(Edge(c.Group), CycleOrder(c.Order))
		if err != nil {
			t.Fatalf("cycle case %d: %v", index, err)
		}
		if len(layouts) != len(c.Expected) {
			t.Fatalf("cycle case %d: %d steps, TypeScript has %d", index, len(layouts), len(c.Expected))
		}
		for step, layout := range layouts {
			want := c.Expected[step]
			if got := (goldenRect{X: layout.X, Y: layout.Y, Width: layout.Width, Height: layout.Height}); got != want {
				t.Fatalf("cycle case %d step %d: got %+v, TypeScript says %+v", index, step, got, want)
			}
		}
	}
}

func TestGoldenCycleStepsMatchTheTypeScriptEngine(t *testing.T) {
	golden := loadGolden(t)
	for index, c := range golden.CycleStepCases {
		options := DefaultOptions()
		options.Cycle = c.CycleEnabled
		options.CycleOrder = CycleOrder(c.CycleOrder)
		var last *AppliedLayout
		if c.Last != nil {
			last = &AppliedLayout{Frame: c.Last.Frame.rect(), CycleGroup: Edge(c.Last.CycleGroup), CycleIndex: c.Last.CycleIndex}
		}
		window := WindowFrame{Frame: c.Current.rect(), IsResizable: true}
		layout, record, err := ResolveCycleStep(c.Spec.layout(), last, window, options)
		if err != nil {
			t.Fatalf("cycle step %d: %v", index, err)
		}
		gotSpec := goldenRect{X: layout.X, Y: layout.Y, Width: layout.Width, Height: layout.Height}
		if gotSpec != c.Expected.Spec {
			t.Fatalf("cycle step %d: spec %+v, TypeScript says %+v", index, gotSpec, c.Expected.Spec)
		}
		wantGroup := ""
		if c.Expected.CycleGroup != nil {
			wantGroup = *c.Expected.CycleGroup
		}
		if string(record.Group) != wantGroup || record.Index != c.Expected.CycleIndex {
			t.Fatalf("cycle step %d: record %+v, TypeScript says group %q index %d", index, record, wantGroup, c.Expected.CycleIndex)
		}
	}
}

func TestGoldenNeighborsMatchTheTypeScriptEngine(t *testing.T) {
	golden := loadGolden(t)
	for index, c := range golden.NeighborCases {
		candidates := make([]NeighborWindow, 0, len(c.Candidates))
		for _, candidate := range c.Candidates {
			candidates = append(candidates, candidate.window())
		}
		id, found := NearestWindowInDirection(c.Origin.window(), candidates, NeighborDirection(c.Direction), NeighborOptions{Wrap: c.Wrap})
		if c.Expected == nil {
			if found {
				t.Fatalf("neighbor case %d: got %q, TypeScript says none", index, id)
			}
			continue
		}
		if !found || id != *c.Expected {
			t.Fatalf("neighbor case %d: got (%q, %v), TypeScript says %q", index, id, found, *c.Expected)
		}
	}
}

func TestGoldenValidationMatchesTheTypeScriptEngine(t *testing.T) {
	golden := loadGolden(t)
	for index, c := range golden.ValidationCases {
		err := DescribeInvalidRequest(c.Spec.layout(), c.Options.options())
		switch {
		case c.Expected == nil && err != nil:
			t.Fatalf("validation case %d: got %q, TypeScript accepts", index, err.Error())
		case c.Expected != nil && (err == nil || err.Error() != *c.Expected):
			t.Fatalf("validation case %d: got %v, TypeScript says %q", index, err, *c.Expected)
		}
	}
}
