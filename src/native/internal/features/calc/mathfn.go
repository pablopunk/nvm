package calc

import "math"

// arity is the accepted argument count range of a function. Max is unbounded for min and max.
type arity struct {
	min int
	max int
}

// unbounded marks a variadic function's maximum argument count.
const unbounded = math.MaxInt

// functionArity lists the functions the calculator knows and how many arguments each takes.
var functionArity = map[string]arity{
	"sqrt":  {1, 1},
	"abs":   {1, 1},
	"round": {1, 1},
	"floor": {1, 1},
	"ceil":  {1, 1},
	"sin":   {1, 1},
	"cos":   {1, 1},
	"tan":   {1, 1},
	"log":   {1, 1},
	"ln":    {1, 1},
	"min":   {1, unbounded},
	"max":   {1, unbounded},
}

// constants are the named values; "e" is Euler's number, not an exponent.
var constants = map[string]float64{
	"pi": math.Pi,
	"π":  math.Pi,
	"e":  math.E,
}

// constantValue returns the value of a named constant.
func constantValue(name string) (float64, bool) {
	value, ok := constants[name]
	return value, ok
}

// knownFunction reports whether name is a calculator function.
func knownFunction(name string) bool {
	_, ok := functionArity[name]
	return ok
}

// arityAccepts reports whether a known function takes count arguments.
func arityAccepts(name string, count int) bool {
	accepted, ok := functionArity[name]
	return ok && count >= accepted.min && count <= accepted.max
}

// callFunction applies a known function. Arity was checked by the parser.
func callFunction(name string, args []float64) float64 {
	switch name {
	case "sqrt":
		return math.Sqrt(args[0])
	case "abs":
		return math.Abs(args[0])
	case "round":
		return jsRound(args[0])
	case "floor":
		return math.Floor(args[0])
	case "ceil":
		return math.Ceil(args[0])
	case "sin":
		return math.Sin(args[0])
	case "cos":
		return math.Cos(args[0])
	case "tan":
		return math.Tan(args[0])
	case "log":
		return log10Exact(args[0])
	case "ln":
		return math.Log(args[0])
	case "min":
		return foldArgs(args, math.Min)
	case "max":
		return foldArgs(args, math.Max)
	}
	return math.NaN()
}

// foldArgs reduces the arguments with a binary function, propagating NaN as Math.min and Math.max do.
func foldArgs(args []float64, combine func(float64, float64) float64) float64 {
	result := args[0]
	for _, arg := range args[1:] {
		result = combine(result, arg)
	}
	return result
}

// jsRound rounds half toward positive infinity, as Math.round does, unlike math.Round which rounds half away
// from zero.
func jsRound(x float64) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return x
	}
	floor := math.Floor(x)
	if x-floor >= 0.5 {
		return floor + 1
	}
	return floor
}

// jsPow is the ** operator: unlike math.Pow it yields NaN for a NaN exponent and for ±1 raised to an infinite power.
func jsPow(x, y float64) float64 {
	if math.IsNaN(y) || ((x == 1 || x == -1) && math.IsInf(y, 0)) {
		return math.NaN()
	}
	return math.Pow(x, y)
}

// log10Exact is Math.log10: it returns exact integers for exact powers of ten, where math.Log10 can be one ulp
// off.
func log10Exact(x float64) float64 {
	result := math.Log10(x)
	if x > 0 {
		if rounded := math.Round(result); math.Pow(10, rounded) == x {
			return rounded
		}
	}
	return result
}
