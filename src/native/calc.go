package main

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// calcResult evaluates math, percentages, unit conversions and number bases.
func calcResult(q string) (display, copyText string, ok bool) {
	q = strings.TrimSpace(q)
	if q == "" {
		return
	}
	if d, c, ok := convertUnits(q); ok {
		return d, c, true
	}
	if v, ok := percentOf(q); ok {
		s := formatNumber(v)
		return q + " = " + s, s, true
	}
	hasOp := inlinePercent.MatchString(strings.ToLower(q)) || strings.ContainsAny(q, "+-*/^%(") || strings.Contains(q, "0x") || regexp.MustCompile(`[a-z]+\(`).MatchString(q)
	if !hasOp {
		return
	}
	expr := inlinePercent.ReplaceAllString(strings.ToLower(q), "(($2)*$1/100)")
	ps := &parser{s: strings.ReplaceAll(expr, " ", "")}
	v := ps.expr()
	if ps.err || ps.i != len(ps.s) || math.IsNaN(v) || math.IsInf(v, 0) {
		return
	}
	s := formatNumber(v)
	extra := ""
	if v == math.Trunc(v) && math.Abs(v) < 1e15 {
		extra = fmt.Sprintf("  (0x%x · 0b%b)", int64(math.Abs(v)), int64(math.Abs(v)))
	}
	return q + " = " + s + extra, s, true
}

func formatNumber(v float64) string {
	if v == math.Trunc(v) && math.Abs(v) < 1e15 {
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return strconv.FormatFloat(v, 'g', 12, 64)
}

var inlinePercent = regexp.MustCompile(`([\d.]+)\s*%\s*of\s*([\d.]+)`)

var percentRe = regexp.MustCompile(`^([\d.]+)\s*%\s*(of|off|on)\s*([\d.]+)$`)
var percentAddRe = regexp.MustCompile(`^([\d.]+)\s*([+-])\s*([\d.]+)\s*%$`)

func percentOf(q string) (float64, bool) {
	q = strings.ToLower(q)
	if m := percentRe.FindStringSubmatch(q); m != nil {
		p, _ := strconv.ParseFloat(m[1], 64)
		n, _ := strconv.ParseFloat(m[3], 64)
		switch m[2] {
		case "of":
			return n * p / 100, true
		case "off":
			return n * (1 - p/100), true
		default:
			return n * (1 + p/100), true
		}
	}
	if m := percentAddRe.FindStringSubmatch(q); m != nil {
		n, _ := strconv.ParseFloat(m[1], 64)
		p, _ := strconv.ParseFloat(m[3], 64)
		if m[2] == "+" {
			return n * (1 + p/100), true
		}
		return n * (1 - p/100), true
	}
	return 0, false
}

type unit struct {
	names []string
	to    float64 // factor to base unit
}

var unitGroups = map[string][]unit{
	"length": {{[]string{"m", "meter", "meters"}, 1}, {[]string{"km", "kilometer", "kilometers"}, 1000}, {[]string{"cm"}, 0.01}, {[]string{"mm"}, 0.001},
		{[]string{"mi", "mile", "miles"}, 1609.344}, {[]string{"ft", "foot", "feet"}, 0.3048}, {[]string{"in", "inch", "inches"}, 0.0254}, {[]string{"yd", "yard", "yards"}, 0.9144}},
	"mass":   {{[]string{"kg", "kilogram", "kilograms"}, 1}, {[]string{"g", "gram", "grams"}, 0.001}, {[]string{"lb", "lbs", "pound", "pounds"}, 0.45359237}, {[]string{"oz", "ounce", "ounces"}, 0.028349523}, {[]string{"t", "ton", "tons"}, 1000}},
	"volume": {{[]string{"l", "liter", "liters"}, 1}, {[]string{"ml"}, 0.001}, {[]string{"gal", "gallon", "gallons"}, 3.785411784}, {[]string{"cup", "cups"}, 0.2365882365}, {[]string{"floz"}, 0.0295735}},
	"time":   {{[]string{"s", "sec", "second", "seconds"}, 1}, {[]string{"min", "minute", "minutes"}, 60}, {[]string{"h", "hr", "hour", "hours"}, 3600}, {[]string{"d", "day", "days"}, 86400}, {[]string{"w", "week", "weeks"}, 604800}, {[]string{"ms"}, 0.001}},
	"data":   {{[]string{"b", "byte", "bytes"}, 1}, {[]string{"kb"}, 1e3}, {[]string{"mb"}, 1e6}, {[]string{"gb"}, 1e9}, {[]string{"tb"}, 1e12}, {[]string{"kib"}, 1024}, {[]string{"mib"}, 1 << 20}, {[]string{"gib"}, 1 << 30}},
}

var convRe = regexp.MustCompile(`^(-?[\d.]+)\s*([a-zA-Z°]+)\s+(?:to|in|as|into)\s+([a-zA-Z°]+)$`)

func findUnit(name string) (string, unit, bool) {
	name = strings.ToLower(name)
	for g, us := range unitGroups {
		for _, u := range us {
			for _, n := range u.names {
				if n == name {
					return g, u, true
				}
			}
		}
	}
	return "", unit{}, false
}

func convertUnits(q string) (string, string, bool) {
	m := convRe.FindStringSubmatch(q)
	if m == nil {
		return "", "", false
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return "", "", false
	}
	from, to := strings.ToLower(strings.TrimPrefix(m[2], "°")), strings.ToLower(strings.TrimPrefix(m[3], "°"))
	if t, ok := temperature(v, from, to); ok {
		s := formatNumber(math.Round(t*100) / 100)
		return fmt.Sprintf("%s %s = %s %s", m[1], strings.ToUpper(from), s, strings.ToUpper(to)), s, true
	}
	g1, u1, ok1 := findUnit(from)
	g2, u2, ok2 := findUnit(to)
	if !ok1 || !ok2 || g1 != g2 {
		return "", "", false
	}
	r := v * u1.to / u2.to
	s := strconv.FormatFloat(r, 'g', 8, 64)
	return fmt.Sprintf("%s %s = %s %s", m[1], from, s, to), s, true
}

func temperature(v float64, from, to string) (float64, bool) {
	toC := map[string]func(float64) float64{"c": func(x float64) float64 { return x }, "f": func(x float64) float64 { return (x - 32) * 5 / 9 }, "k": func(x float64) float64 { return x - 273.15 }}
	fromC := map[string]func(float64) float64{"c": func(x float64) float64 { return x }, "f": func(x float64) float64 { return x*9/5 + 32 }, "k": func(x float64) float64 { return x + 273.15 }}
	a, ok1 := toC[from]
	b, ok2 := fromC[to]
	if !ok1 || !ok2 {
		return 0, false
	}
	return b(a(v)), true
}

// ---- recursive-descent expression parser ----

type parser struct {
	s   string
	i   int
	err bool
}

func (p *parser) peek() byte {
	if p.i < len(p.s) {
		return p.s[p.i]
	}
	return 0
}

func (p *parser) expr() float64 {
	v := p.term()
	for p.peek() == '+' || p.peek() == '-' {
		op := p.s[p.i]
		p.i++
		r := p.term()
		if op == '+' {
			v += r
		} else {
			v -= r
		}
	}
	return v
}

func (p *parser) term() float64 {
	v := p.pow()
	for p.peek() == '*' || p.peek() == '/' || p.peek() == '%' {
		op := p.s[p.i]
		p.i++
		r := p.pow()
		switch op {
		case '*':
			v *= r
		case '/':
			v /= r
		default:
			v = math.Mod(v, r)
		}
	}
	return v
}

func (p *parser) pow() float64 {
	v := p.unary()
	if p.peek() == '^' {
		p.i++
		return math.Pow(v, p.pow())
	}
	return v
}

var funcs = map[string]func(float64) float64{
	"sqrt": math.Sqrt, "sin": math.Sin, "cos": math.Cos, "tan": math.Tan, "log": math.Log10, "ln": math.Log,
	"abs": math.Abs, "round": math.Round, "floor": math.Floor, "ceil": math.Ceil, "exp": math.Exp,
}

func (p *parser) unary() float64 {
	switch c := p.peek(); {
	case c == '-':
		p.i++
		return -p.unary()
	case c == '(':
		p.i++
		v := p.expr()
		if p.peek() != ')' {
			p.err = true
			return 0
		}
		p.i++
		return v
	case unicode.IsLetter(rune(c)):
		start := p.i
		for p.i < len(p.s) && (unicode.IsLetter(rune(p.s[p.i])) || unicode.IsDigit(rune(p.s[p.i]))) {
			p.i++
		}
		name := p.s[start:p.i]
		if strings.HasPrefix(name, "0x") {
			p.i = start
			break
		}
		switch name {
		case "pi":
			return math.Pi
		case "e":
			return math.E
		}
		f, ok := funcs[name]
		if !ok || p.peek() != '(' {
			p.err = true
			return 0
		}
		return f(p.unary())
	}
	start := p.i
	if strings.HasPrefix(p.s[p.i:], "0x") {
		p.i += 2
		for p.i < len(p.s) && strings.ContainsRune("0123456789abcdef", rune(p.s[p.i])) {
			p.i++
		}
		n, err := strconv.ParseInt(p.s[start+2:p.i], 16, 64)
		if err != nil {
			p.err = true
		}
		return float64(n)
	}
	for p.i < len(p.s) && (unicode.IsDigit(rune(p.s[p.i])) || p.s[p.i] == '.') {
		p.i++
	}
	v, err := strconv.ParseFloat(p.s[start:p.i], 64)
	if err != nil {
		p.err = true
	}
	return v
}
