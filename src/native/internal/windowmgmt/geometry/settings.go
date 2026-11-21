package geometry

// Settings are the user-tunable numbers that shape layouts and options. Percent fields are whole
// percents (90 means 0.9 of the work area), as the settings UI stores them. Per-edge outer gaps are
// nil when they inherit OuterGap.
type Settings struct {
	OuterGap                    float64
	OuterGapTop                 *float64
	OuterGapRight               *float64
	OuterGapBottom              *float64
	OuterGapLeft                *float64
	InnerGap                    float64
	ApplyGapToMaximize          bool
	AlmostMaximizeWidthPercent  float64
	AlmostMaximizeHeightPercent float64
	ReasonableSizeWidthPercent  float64
	ReasonableSizeHeightPercent float64
	CenterHalfWidthPercent      float64
	ResizeStepPercent           float64
	Cycle                       bool
	CycleOrder                  CycleOrder
	DisplayOrder                DisplayOrder
	DisplayMoveSizing           DisplayMoveSizing
}

// DefaultSettings returns the defaults from DEFAULT_WINDOW_MANAGEMENT_SETTINGS.
func DefaultSettings() Settings {
	return Settings{
		OuterGap:                    8,
		InnerGap:                    4,
		ApplyGapToMaximize:          true,
		AlmostMaximizeWidthPercent:  90,
		AlmostMaximizeHeightPercent: 90,
		ReasonableSizeWidthPercent:  70,
		ReasonableSizeHeightPercent: 70,
		CenterHalfWidthPercent:      50,
		ResizeStepPercent:           5,
		Cycle:                       true,
		CycleOrder:                  CycleHalfTwoThirdsThird,
		DisplayOrder:                DisplayOrderLeftToRight,
		DisplayMoveSizing:           MoveProportional,
	}
}

const percentPerWhole = 100

func percentToFraction(percent float64) float64 {
	return percent / percentPerWhole
}

// LayoutSpecFor returns the layout for a preset, with the settings-dependent presets rebuilt from
// the user's percents. Every other preset keeps its catalog definition.
func LayoutSpecFor(id PresetID, settings Settings) (Layout, bool) {
	catalog, ok := Preset(id)
	if !ok {
		return Layout{}, false
	}
	switch id {
	case "almost-maximize":
		return scaledCenter(percentToFraction(settings.AlmostMaximizeWidthPercent),
			percentToFraction(settings.AlmostMaximizeHeightPercent)), true
	case "reasonable-size":
		return scaledCenter(percentToFraction(settings.ReasonableSizeWidthPercent),
			percentToFraction(settings.ReasonableSizeHeightPercent)), true
	case "center-half":
		width := percentToFraction(settings.CenterHalfWidthPercent)
		return plainFraction((1-width)/2, 0, width, 1), true
	case "make-larger":
		return resizeStep(DirectionLarger, percentToFraction(settings.ResizeStepPercent)), true
	case "make-smaller":
		return resizeStep(DirectionSmaller, percentToFraction(settings.ResizeStepPercent)), true
	}
	return catalog, true
}

// OptionsFromSettings maps settings to the engine options. Each outer gap edge falls back to the
// shared outer gap unless it has its own value, and a zero override still counts.
func OptionsFromSettings(settings Settings) Options {
	return Options{
		OuterGap: EdgeGaps{
			Top:    orDefault(settings.OuterGapTop, settings.OuterGap),
			Right:  orDefault(settings.OuterGapRight, settings.OuterGap),
			Bottom: orDefault(settings.OuterGapBottom, settings.OuterGap),
			Left:   orDefault(settings.OuterGapLeft, settings.OuterGap),
		},
		InnerGap:           settings.InnerGap,
		ApplyGapToMaximize: settings.ApplyGapToMaximize,
		Cycle:              settings.Cycle,
		CycleOrder:         settings.CycleOrder,
		DisplayOrder:       settings.DisplayOrder,
		DisplayMoveSizing:  settings.DisplayMoveSizing,
	}
}

func orDefault(override *float64, fallback float64) float64 {
	if override != nil {
		return *override
	}
	return fallback
}
