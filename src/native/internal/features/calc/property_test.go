package calc

import (
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"testing"
)

// oracleExpression is a generated expression with its text and the value computed while generating it. The oracle
// evaluates the same tree with plain float64 operations in the same order, independently of the parser.
type oracleExpression struct {
	text  string
	value float64
}

// oracleGenerator builds random fully parenthesized expressions from a seeded source, so runs are reproducible.
type oracleGenerator struct {
	rng *rand.Rand
}

// leaf returns a random number literal and its value.
func (g oracleGenerator) leaf() oracleExpression {
	if g.rng.Intn(2) == 0 {
		n := g.rng.Intn(50)
		return oracleExpression{text: fmt.Sprint(n), value: float64(n)}
	}
	text := fmt.Sprintf("%d.%02d", g.rng.Intn(200), g.rng.Intn(100))
	value, _ := strconv.ParseFloat(text, 64)
	return oracleExpression{text: text, value: value}
}

// expression returns a random expression of at most depth levels of nesting.
func (g oracleGenerator) expression(depth int) oracleExpression {
	if depth == 0 || g.rng.Intn(4) == 0 {
		return g.leaf()
	}
	switch g.rng.Intn(9) {
	case 0:
		operand := g.expression(depth - 1)
		return oracleExpression{text: "(-" + operand.text + ")", value: -operand.value}
	case 1:
		base, exponent := g.rng.Intn(6), g.rng.Intn(4)
		return oracleExpression{text: fmt.Sprintf("(%d ^ %d)", base, exponent), value: math.Pow(float64(base), float64(exponent))}
	case 2:
		operand := g.expression(depth - 1)
		return oracleExpression{text: "abs(" + operand.text + ")", value: math.Abs(operand.value)}
	case 3:
		left, right := g.expression(depth-1), g.expression(depth-1)
		return oracleExpression{text: "max(" + left.text + ", " + right.text + ")", value: math.Max(left.value, right.value)}
	case 4:
		operand := g.leaf()
		return oracleExpression{text: "(" + operand.text + " * 10%)", value: operand.value * (10.0 / 100)}
	}
	return g.binary(depth)
}

// root returns a random compound expression. A bare number is not a calculation, so roots always carry a
// parenthesis, which every compound form has and no number literal does.
func (g oracleGenerator) root(depth int) oracleExpression {
	for {
		if expr := g.expression(depth); strings.Contains(expr.text, "(") {
			return expr
		}
	}
}

// binary returns a parenthesized arithmetic operation. Division never divides by zero.
func (g oracleGenerator) binary(depth int) oracleExpression {
	left := g.expression(depth - 1)
	op := "+-*/"[g.rng.Intn(4)]
	right := g.expression(depth - 1)
	for op == '/' && right.value == 0 {
		right = g.leaf()
	}
	var value float64
	switch op {
	case '+':
		value = left.value + right.value
	case '-':
		value = left.value - right.value
	case '*':
		value = left.value * right.value
	default:
		value = left.value / right.value
	}
	return oracleExpression{text: "(" + left.text + " " + string(op) + " " + right.text + ")", value: value}
}

// TestRandomArithmeticMatchesOracle evaluates several hundred random expressions and checks each answer against the
// value the generator computed. The tolerance covers the 12-digit rounding every answer gets.
func TestRandomArithmeticMatchesOracle(t *testing.T) {
	c := newTestCalculator(testNow)
	g := oracleGenerator{rng: rand.New(rand.NewSource(20260606))}
	checked := 0
	for i := 0; i < 1000; i++ {
		expr := g.root(4)
		if math.IsNaN(expr.value) || math.IsInf(expr.value, 0) || math.Abs(expr.value) > 1e12 {
			continue
		}
		got, ok := c.Evaluate(expr.text)
		if !ok {
			t.Fatalf("Evaluate(%q) returned no answer, oracle %v", expr.text, expr.value)
		}
		if math.Abs(got.Value-expr.value) > 1e-9*math.Max(1, math.Abs(expr.value)) {
			t.Fatalf("Evaluate(%q) = %v, oracle %v", expr.text, got.Value, expr.value)
		}
		checked++
	}
	if checked < 300 {
		t.Fatalf("only %d expressions were checked; the generator filtered too much", checked)
	}
}

// TestRawTextIsIdempotent checks that feeding a result's raw text back in gives the same raw text, which is what the
// Continue Calculation action relies on.
func TestRawTextIsIdempotent(t *testing.T) {
	c := newTestCalculator(testNow)
	g := oracleGenerator{rng: rand.New(rand.NewSource(7))}
	for i := 0; i < 300; i++ {
		expr := g.root(3)
		if math.IsNaN(expr.value) || math.Abs(expr.value) > 1e12 {
			continue
		}
		first, ok := c.Evaluate(expr.text)
		if !ok || first.Kind == KindDate || first.Kind == KindTimezone {
			continue
		}
		second, ok := c.Evaluate("(" + first.Raw + ")")
		if !ok || second.Raw != first.Raw {
			t.Fatalf("raw %q of %q re-evaluated to %+v, ok %v", first.Raw, expr.text, second, ok)
		}
	}
}

// TestEvaluateNeverPanicsOnRandomInput feeds random strings built from the characters the grammar and the patterns
// care about, plus bytes that are not ASCII, and checks the invariants on every answer.
func TestEvaluateNeverPanicsOnRandomInput(t *testing.T) {
	alphabet := []rune("0123456789.,+-*/^%()= abcdefghilmnoprstuwyzkmbπ°€$£×÷−\u00a0\u2028" +
		string(rune(0x200b)) + "\U0001F600")
	rng := rand.New(rand.NewSource(99))
	c := newTestCalculator(testNow)
	for i := 0; i < 20000; i++ {
		var b strings.Builder
		length := rng.Intn(40)
		for j := 0; j < length; j++ {
			b.WriteRune(alphabet[rng.Intn(len(alphabet))])
		}
		checkEvaluateInvariants(t, c, b.String())
	}
}
