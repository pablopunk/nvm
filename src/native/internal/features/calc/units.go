package calc

// dimension groups units that convert into each other. Units of different dimensions never convert.
type dimension string

const (
	dimensionLength      dimension = "length"
	dimensionMass        dimension = "mass"
	dimensionVolume      dimension = "volume"
	dimensionData        dimension = "data"
	dimensionTime        dimension = "time"
	dimensionDesign      dimension = "design"
	dimensionTemperature dimension = "temperature"
)

// unit converts between its own scale and a dimension's base unit. Linear units only need factor; temperatures
// carry explicit conversions.
type unit struct {
	dimension dimension
	symbol    string
	toBase    func(float64) float64
	fromBase  func(float64) float64
}

// unitsByAlias maps every accepted spelling, lower-cased and without dots, to its unit.
var unitsByAlias = buildUnitTable()

// buildUnitTable declares every unit the calculator knows, in the order the TypeScript original declares them.
func buildUnitTable() map[string]unit {
	units := map[string]unit{}
	addLinear := func(dim dimension, symbol string, factor float64, aliases ...string) {
		addUnit(units, unit{
			dimension: dim,
			symbol:    symbol,
			toBase:    func(value float64) float64 { return value * factor },
			fromBase:  func(value float64) float64 { return value / factor },
		}, aliases)
	}
	addTemperature := func(symbol string, toBase, fromBase func(float64) float64, aliases ...string) {
		addUnit(units, unit{dimension: dimensionTemperature, symbol: symbol, toBase: toBase, fromBase: fromBase}, aliases)
	}
	addLengthUnits(addLinear)
	addMassVolumeUnits(addLinear)
	addDataTimeDesignUnits(addLinear)
	addTemperature("°C",
		func(value float64) float64 { return value },
		func(value float64) float64 { return value },
		"c", "°c", "celsius")
	addTemperature("°F",
		func(value float64) float64 { return ((value - 32) * 5) / 9 },
		func(value float64) float64 { return (value*9)/5 + 32 },
		"f", "°f", "fahrenheit")
	addTemperature("K",
		func(value float64) float64 { return value - 273.15 },
		func(value float64) float64 { return value + 273.15 },
		"k", "kelvin")
	return units
}

// linearAdder registers a unit with a multiplicative factor to its dimension's base unit.
type linearAdder func(dim dimension, symbol string, factor float64, aliases ...string)

// addLengthUnits registers the length units, base metre.
func addLengthUnits(add linearAdder) {
	add(dimensionLength, "mm", 0.001, "mm", "millimeter", "millimeters")
	add(dimensionLength, "cm", 0.01, "cm", "centimeter", "centimeters")
	add(dimensionLength, "m", 1, "m", "meter", "meters", "metre", "metres")
	add(dimensionLength, "km", 1000, "km", "kilometer", "kilometers", "kilometre", "kilometres")
	add(dimensionLength, "in", 0.0254, "in", "inch", "inches")
	add(dimensionLength, "ft", 0.3048, "ft", "foot", "feet")
	add(dimensionLength, "yd", 0.9144, "yd", "yard", "yards")
	add(dimensionLength, "mi", 1609.344, "mi", "mile", "miles")
}

// addMassVolumeUnits registers the mass and volume units, base kilogram and litre.
func addMassVolumeUnits(add linearAdder) {
	add(dimensionMass, "mg", 0.000001, "mg", "milligram", "milligrams")
	add(dimensionMass, "g", 0.001, "g", "gram", "grams")
	add(dimensionMass, "kg", 1, "kg", "kilogram", "kilograms")
	add(dimensionMass, "oz", 0.028349523125, "oz", "ounce", "ounces")
	add(dimensionMass, "lb", 0.45359237, "lb", "lbs", "pound", "pounds")
	add(dimensionVolume, "ml", 0.001, "ml", "milliliter", "milliliters", "millilitre", "millilitres")
	add(dimensionVolume, "l", 1, "l", "liter", "liters", "litre", "litres")
	add(dimensionVolume, "tsp", 0.00492892159375, "tsp", "teaspoon", "teaspoons")
	add(dimensionVolume, "tbsp", 0.01478676478125, "tbsp", "tablespoon", "tablespoons")
	add(dimensionVolume, "cup", 0.2365882365, "cup", "cups")
	add(dimensionVolume, "pint", 0.473176473, "pint", "pints")
	add(dimensionVolume, "quart", 0.946352946, "quart", "quarts")
	add(dimensionVolume, "gal", 3.785411784, "gal", "gallon", "gallons")
}

// addDataTimeDesignUnits registers data sizes (base byte), time (base second) and design units (base pixel).
func addDataTimeDesignUnits(add linearAdder) {
	add(dimensionData, "B", 1, "b", "byte", "bytes")
	add(dimensionData, "KB", 1000, "kb", "kilobyte", "kilobytes")
	add(dimensionData, "MB", 1000*1000, "mb", "megabyte", "megabytes")
	add(dimensionData, "GB", 1000*1000*1000, "gb", "gigabyte", "gigabytes")
	add(dimensionData, "TB", 1000*1000*1000*1000, "tb", "terabyte", "terabytes")
	add(dimensionData, "KiB", 1024, "kib", "kibibyte", "kibibytes")
	add(dimensionData, "MiB", 1024*1024, "mib", "mebibyte", "mebibytes")
	add(dimensionData, "GiB", 1024*1024*1024, "gib", "gibibyte", "gibibytes")
	add(dimensionData, "TiB", 1024*1024*1024*1024, "tib", "tebibyte", "tebibytes")
	add(dimensionTime, "ms", 0.001, "ms", "millisecond", "milliseconds")
	add(dimensionTime, "s", 1, "s", "sec", "secs", "second", "seconds")
	add(dimensionTime, "min", 60, "min", "mins", "minute", "minutes")
	add(dimensionTime, "h", 3600, "h", "hr", "hrs", "hour", "hours")
	add(dimensionTime, "day", 86400, "day", "days")
	add(dimensionTime, "week", 604800, "week", "weeks")
	add(dimensionDesign, "px", 1, "px", "pixel", "pixels")
	add(dimensionDesign, "rem", 16, "rem", "rems")
}

// addUnit registers a unit under each alias, normalized the way lookups normalize them.
func addUnit(units map[string]unit, u unit, aliases []string) {
	for _, alias := range aliases {
		units[normalizeUnitAlias(alias)] = u
	}
}

// unitForAlias finds a unit by any accepted spelling.
func unitForAlias(alias string) (unit, bool) {
	u, ok := unitsByAlias[normalizeUnitAlias(alias)]
	return u, ok
}
