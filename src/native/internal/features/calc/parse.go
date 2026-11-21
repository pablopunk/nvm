package calc

// maxNestingDepth bounds how deep the parser recurses through parentheses, unary signs and right-associative
// powers. Input is also capped at 500 characters, so hostile text cannot exhaust the stack.
const maxNestingDepth = 64

type nodeKind uint8

const (
	nodeNumber nodeKind = iota
	nodePercent
	nodeUnary
	nodeBinary
	nodeFunction
)

// node is one expression tree node. Percent and unary nodes use left as their operand; binary nodes use left and
// right; function nodes use name and args.
type node struct {
	kind   nodeKind
	number float64
	op     byte
	left   *node
	right  *node
	name   string
	args   []*node
}

// parser is a recursive-descent parser over tokens with the TypeScript original's precedence rules: + and -
// bind weakest, then * and /, then ^ which is right-associative. A unary sign binds tighter than ^ on its
// left, as in the original.
type parser struct {
	tokens []token
	pos    int
	depth  int
}

// parseTokens builds the expression tree for a token stream, requiring the whole stream to be consumed.
func parseTokens(tokens []token) (*node, error) {
	p := &parser{tokens: tokens}
	tree, err := p.expression(0)
	if err != nil {
		return nil, err
	}
	if p.current().kind != tokenEOF {
		return nil, errSyntax
	}
	return tree, nil
}

// enter counts one level of nesting and fails past maxNestingDepth. Callers pair it with leave.
func (p *parser) enter() error {
	p.depth++
	if p.depth > maxNestingDepth {
		return errSyntax
	}
	return nil
}

// leave undoes one enter.
func (p *parser) leave() {
	p.depth--
}

// current returns the token at the cursor.
func (p *parser) current() token {
	return p.tokens[p.pos]
}

// expression parses binary operators whose precedence is at least minPrecedence.
func (p *parser) expression(minPrecedence int) (*node, error) {
	if err := p.enter(); err != nil {
		return nil, err
	}
	defer p.leave()
	left, err := p.unary()
	if err != nil {
		return nil, err
	}
	for {
		tok := p.current()
		if tok.kind != tokenOperator || precedence(tok.text[0]) < minPrecedence {
			return left, nil
		}
		op := tok.text[0]
		p.pos++
		nextMinimum := precedence(op) + 1
		if op == '^' {
			nextMinimum = precedence(op)
		}
		right, err := p.expression(nextMinimum)
		if err != nil {
			return nil, err
		}
		left = &node{kind: nodeBinary, op: op, left: left, right: right}
	}
}

// unary parses prefix plus and minus, then a postfix chain.
func (p *parser) unary() (*node, error) {
	tok := p.current()
	if tok.kind != tokenOperator || (tok.text != "+" && tok.text != "-") {
		return p.postfix()
	}
	if err := p.enter(); err != nil {
		return nil, err
	}
	defer p.leave()
	p.pos++
	operand, err := p.unary()
	if err != nil {
		return nil, err
	}
	return &node{kind: nodeUnary, op: tok.text[0], left: operand}, nil
}

// postfix parses a primary and any number of percent signs after it.
func (p *parser) postfix() (*node, error) {
	tree, err := p.primary()
	if err != nil {
		return nil, err
	}
	for p.current().kind == tokenPercent {
		p.pos++
		tree = &node{kind: nodePercent, left: tree}
	}
	return tree, nil
}

// primary parses a number with an optional magnitude suffix word, a constant, a function call, or a
// parenthesized expression.
func (p *parser) primary() (*node, error) {
	tok := p.current()
	switch tok.kind {
	case tokenNumber:
		p.pos++
		value := tok.number
		if next := p.current(); next.kind == tokenIdentifier {
			if multiplier, ok := suffixMultiplier(next.text); ok {
				value *= multiplier
				p.pos++
			}
		}
		return &node{kind: nodeNumber, number: value}, nil
	case tokenIdentifier:
		p.pos++
		if value, ok := constantValue(tok.text); ok {
			return &node{kind: nodeNumber, number: value}, nil
		}
		if !knownFunction(tok.text) {
			return nil, errSyntax
		}
		args, err := p.functionArgs()
		if err != nil {
			return nil, err
		}
		if !arityAccepts(tok.text, len(args)) {
			return nil, errSyntax
		}
		return &node{kind: nodeFunction, name: tok.text, args: args}, nil
	case tokenLeftParen:
		p.pos++
		inner, err := p.expression(0)
		if err != nil {
			return nil, err
		}
		if err := p.expect(tokenRightParen); err != nil {
			return nil, err
		}
		return inner, nil
	}
	return nil, errSyntax
}

// functionArgs parses the argument list after a function name. Without parentheses a single unary operand is
// taken, so "sqrt 625" works. The argument count is checked against the function's arity here.
func (p *parser) functionArgs() ([]*node, error) {
	if p.current().kind != tokenLeftParen {
		operand, err := p.unary()
		if err != nil {
			return nil, err
		}
		return []*node{operand}, nil
	}
	p.pos++
	if p.current().kind == tokenRightParen {
		return nil, errSyntax
	}
	var args []*node
	for {
		arg, err := p.expression(0)
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
		if p.current().kind != tokenComma {
			break
		}
		p.pos++
	}
	if err := p.expect(tokenRightParen); err != nil {
		return nil, err
	}
	return args, nil
}

// expect consumes a token of the given kind or fails.
func (p *parser) expect(kind tokenKind) error {
	if p.current().kind != kind {
		return errSyntax
	}
	p.pos++
	return nil
}

// precedence orders the binary operators; zero means not an operator.
func precedence(op byte) int {
	switch op {
	case '+', '-':
		return 1
	case '*', '/':
		return 2
	case '^':
		return 3
	}
	return 0
}
