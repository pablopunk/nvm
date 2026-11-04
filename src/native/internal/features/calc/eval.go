package calc

import "math"

// evaluateMathExpression parses and evaluates an expression. It fails when the text does not parse or the value
// is not finite, and otherwise returns the value rounded to 12 significant digits.
func evaluateMathExpression(expression string) (float64, bool) {
	tokens, err := tokenize(expression)
	if err != nil {
		return 0, false
	}
	tree, err := parseTokens(tokens)
	if err != nil {
		return 0, false
	}
	value := evaluateNode(tree)
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	return normalizeFloatingPoint(value), true
}

// evaluateNode computes a tree. Percent-of-operand additions follow the original: "a + b%" means a + a*b/100,
// while "a * b%" multiplies by b/100.
func evaluateNode(tree *node) float64 {
	switch tree.kind {
	case nodeNumber:
		return tree.number
	case nodePercent:
		return evaluateNode(tree.left) / 100
	case nodeUnary:
		value := evaluateNode(tree.left)
		if tree.op == '-' {
			return -value
		}
		return value
	case nodeFunction:
		args := make([]float64, len(tree.args))
		for i, arg := range tree.args {
			args[i] = evaluateNode(arg)
		}
		return callFunction(tree.name, args)
	}
	return evaluateBinary(tree)
}

// evaluateBinary applies a binary operator, special-casing "x + y%" and "x - y%" as percentage changes.
func evaluateBinary(tree *node) float64 {
	left := evaluateNode(tree.left)
	if (tree.op == '+' || tree.op == '-') && tree.right.kind == nodePercent {
		percentage := evaluateNode(tree.right)
		if tree.op == '+' {
			return left + left*percentage
		}
		return left - left*percentage
	}
	right := evaluateNode(tree.right)
	switch tree.op {
	case '+':
		return left + right
	case '-':
		return left - right
	case '*':
		return left * right
	case '/':
		return left / right
	}
	return jsPow(left, right)
}
