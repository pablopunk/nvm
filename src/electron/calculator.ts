export type CalculatorResult = {
  kind: 'calculation' | 'conversion' | 'date' | 'timezone' | 'rate'
  query: string
  expression: string
  value: number
  raw: string
  formatted: string
  unit?: string
  targetUnit?: string
  rateUnit?: string
  date?: string
  timezone?: string
  title: string
  subtitle: string
  alternate?: {
    raw?: string
    formatted?: string
    query?: string
  }
  swapQuery?: string
}

type CalculatorOperator = '+' | '-' | '*' | '/' | '^'

type Token =
  | { type: 'number'; value: number }
  | { type: 'identifier'; value: string }
  | { type: 'operator'; value: CalculatorOperator }
  | { type: 'percent' }
  | { type: 'leftParen' }
  | { type: 'rightParen' }
  | { type: 'comma' }
  | { type: 'eof' }

type ExpressionNode =
  | { type: 'number'; value: number }
  | { type: 'percent'; value: ExpressionNode }
  | { type: 'unary'; operator: '+' | '-'; value: ExpressionNode }
  | { type: 'binary'; operator: CalculatorOperator; left: ExpressionNode; right: ExpressionNode }
  | { type: 'function'; name: string; args: ExpressionNode[] }

const MAX_EXPRESSION_LENGTH = 500
const SUFFIX_MULTIPLIERS: Record<string, number> = { k: 1_000, m: 1_000_000, b: 1_000_000_000 }
const CONSTANTS: Record<string, number> = { pi: Math.PI, π: Math.PI, e: Math.E }
const FUNCTION_ARITY: Record<string, { min: number; max: number }> = {
  sqrt: { min: 1, max: 1 },
  abs: { min: 1, max: 1 },
  round: { min: 1, max: 1 },
  floor: { min: 1, max: 1 },
  ceil: { min: 1, max: 1 },
  sin: { min: 1, max: 1 },
  cos: { min: 1, max: 1 },
  tan: { min: 1, max: 1 },
  log: { min: 1, max: 1 },
  ln: { min: 1, max: 1 },
  min: { min: 1, max: Number.POSITIVE_INFINITY },
  max: { min: 1, max: Number.POSITIVE_INFINITY },
}

export function calculate(query: string) {
  return calculateDetailed(query)?.raw ?? null
}

export function calculateDetailed(query: string): CalculatorResult | null {
  const normalized = normalizeCalculationQuery(query)
  if (!normalized || !isLikelyCalculation(normalized.expression, normalized.explicit)) return null

  const unitPercentResult = calculateUnitPercentExpression(normalized.expression, query)
  if (unitPercentResult) return unitPercentResult

  const expression = normalizeNaturalMath(normalized.expression)
  const value = evaluateMathExpression(expression)
  if (value === null) return null
  const raw = formatRawNumber(value)
  const formatted = formatDisplayNumber(value)
  return {
    kind: 'calculation',
    query: query.trim(),
    expression,
    value,
    raw,
    formatted,
    title: `${normalized.expression} = ${formatted}`,
    subtitle: 'Copy result to clipboard',
    alternate: raw === formatted ? undefined : { raw, formatted },
  }
}

function normalizeCalculationQuery(query: string) {
  const trimmed = String(query || '').trim()
  if (!trimmed || trimmed.length > MAX_EXPRESSION_LENGTH) return null
  const withoutEquals = trimmed.replace(/^=\s*/, '')
  const withoutCalc = withoutEquals.replace(/^calc(?:ulate)?\s+/i, '')
  return { expression: withoutCalc.trim(), explicit: withoutEquals !== trimmed || withoutCalc !== withoutEquals }
}

function isLikelyCalculation(expression: string, explicit: boolean) {
  if (explicit) return expression.length > 0
  if (/[+*/%^()]/.test(expression)) return true
  if (/\d\s-\s\d/.test(expression)) return true
  if (/\b(sqrt|square root|power|squared|cubed|sin|cos|tan|log|ln|abs|round|floor|ceil|min|max|percent)\b/i.test(expression)) return true
  return /\d(?:\.\d+)?\s*[kmb]\b.*[+\-*/^]/i.test(expression)
}

function normalizeNaturalMath(expression: string) {
  let next = expression.trim()
  next = next.replace(/[×]/g, '*').replace(/[÷]/g, '/')
  next = next.replace(/^(?:what(?:'s| is)|calculate|calc)\s+/i, '')
  next = next.replace(/\bpercent\b/gi, '%')
  next = next.replace(/^square\s+root\s+of\s+(.+)$/i, 'sqrt($1)')
  next = next.replace(/\bpower\b/gi, '^')
  next = next.replace(/^(.+)\s+squared$/i, '($1)^2')
  next = next.replace(/^(.+)\s+cubed$/i, '($1)^3')
  if (/%/.test(next)) next = next.replace(/\bof\b/gi, '*')
  return next
}

function calculateUnitPercentExpression(expression: string, query: string): CalculatorResult | null {
  const match = expression.trim().match(/^([+-]?(?:\d+(?:,\d{3})*|\d*)(?:\.\d+)?)\s*([a-zA-Z]+)\s*([+-])\s*([+-]?(?:\d+(?:,\d{3})*|\d*)(?:\.\d+)?)\s*%$/)
  if (!match) return null
  const left = parseNumberLiteral(match[1])
  const percent = parseNumberLiteral(match[4])
  if (left === null || percent === null) return null
  const unit = match[2]
  const multiplier = match[3] === '+' ? 1 + percent / 100 : 1 - percent / 100
  const value = left * multiplier
  if (!Number.isFinite(value)) return null
  const raw = `${formatRawNumber(value)} ${unit}`
  const formatted = `${formatDisplayNumber(value)} ${unit}`
  return {
    kind: 'calculation',
    query: query.trim(),
    expression: expression.trim(),
    value,
    raw,
    formatted,
    unit,
    title: `${expression.trim()} = ${formatted}`,
    subtitle: 'Copy result to clipboard',
    alternate: raw === formatted ? undefined : { raw, formatted },
  }
}

function evaluateMathExpression(expression: string) {
  try {
    const parser = new CalculatorParser(tokenize(expression))
    const tree = parser.parse()
    const value = evaluateNode(tree)
    return Number.isFinite(value) ? normalizeFloatingPoint(value) : null
  } catch {
    return null
  }
}

function tokenize(expression: string): Token[] {
  const tokens: Token[] = []
  let index = 0
  while (index < expression.length) {
    const character = expression[index]
    if (/\s/.test(character)) {
      index += 1
      continue
    }
    if (character === '(') {
      tokens.push({ type: 'leftParen' })
      index += 1
      continue
    }
    if (character === ')') {
      tokens.push({ type: 'rightParen' })
      index += 1
      continue
    }
    if (character === ',') {
      tokens.push({ type: 'comma' })
      index += 1
      continue
    }
    if (character === '%') {
      tokens.push({ type: 'percent' })
      index += 1
      continue
    }
    if (character === '+' || character === '-' || character === '*' || character === '/' || character === '^') {
      tokens.push({ type: 'operator', value: character })
      index += 1
      continue
    }
    if (isNumberStart(expression, index)) {
      const result = readNumber(expression, index)
      tokens.push({ type: 'number', value: result.value })
      index = result.nextIndex
      continue
    }
    if (/[a-zA-Zπ]/.test(character)) {
      const start = index
      while (index < expression.length && /[a-zA-Zπ]/.test(expression[index])) index += 1
      tokens.push({ type: 'identifier', value: expression.slice(start, index).toLowerCase() })
      continue
    }
    throw new Error(`Unexpected character: ${character}`)
  }
  tokens.push({ type: 'eof' })
  return tokens
}

function isNumberStart(expression: string, index: number) {
  return /\d/.test(expression[index]) || (expression[index] === '.' && /\d/.test(expression[index + 1] || ''))
}

function readNumber(expression: string, index: number) {
  const start = index
  let sawDot = false
  while (index < expression.length) {
    const character = expression[index]
    if (/\d/.test(character)) {
      index += 1
      continue
    }
    if (character === '.' && !sawDot) {
      sawDot = true
      index += 1
      continue
    }
    if (character === ',' && /^,\d{3}(?!\d)/.test(expression.slice(index))) {
      index += 1
      continue
    }
    break
  }
  const parsed = parseNumberLiteral(expression.slice(start, index))
  if (parsed === null) throw new Error('Invalid number')
  let value = parsed
  const suffix = expression[index]?.toLowerCase()
  const next = expression[index + 1] || ''
  if (suffix && SUFFIX_MULTIPLIERS[suffix] && !/[a-zA-Z]/.test(next)) {
    value *= SUFFIX_MULTIPLIERS[suffix]
    index += 1
  }
  return { value, nextIndex: index }
}

function parseNumberLiteral(value: string) {
  if (!value || value === '.' || value === '+' || value === '-') return null
  const parsed = Number(value.replace(/,/g, ''))
  return Number.isFinite(parsed) ? parsed : null
}

class CalculatorParser {
  private index = 0

  constructor(private readonly tokens: Token[]) {}

  parse() {
    const expression = this.parseExpression(0)
    if (this.current().type !== 'eof') throw new Error('Unexpected trailing token')
    return expression
  }

  private parseExpression(minPrecedence: number): ExpressionNode {
    let left = this.parseUnary()
    while (true) {
      const token = this.current()
      if (token.type !== 'operator' || precedence(token.value) < minPrecedence) break
      const operator = token.value
      const operatorPrecedence = precedence(operator)
      this.index += 1
      const right = this.parseExpression(operatorPrecedence + (operator === '^' ? 0 : 1))
      left = { type: 'binary', operator, left, right }
    }
    return left
  }

  private parseUnary(): ExpressionNode {
    const token = this.current()
    if (token.type === 'operator' && (token.value === '+' || token.value === '-')) {
      const operator = token.value
      this.index += 1
      return { type: 'unary', operator, value: this.parseUnary() }
    }
    return this.parsePostfix()
  }

  private parsePostfix(): ExpressionNode {
    let node = this.parsePrimary()
    while (this.current().type === 'percent') {
      this.index += 1
      node = { type: 'percent', value: node }
    }
    return node
  }

  private parsePrimary(): ExpressionNode {
    const token = this.current()
    if (token.type === 'number') {
      this.index += 1
      let value = token.value
      const suffix = this.current()
      if (suffix.type === 'identifier' && SUFFIX_MULTIPLIERS[suffix.value]) {
        value *= SUFFIX_MULTIPLIERS[suffix.value]
        this.index += 1
      }
      return { type: 'number', value }
    }
    if (token.type === 'identifier') {
      this.index += 1
      if (CONSTANTS[token.value] !== undefined) return { type: 'number', value: CONSTANTS[token.value] }
      if (!FUNCTION_ARITY[token.value]) throw new Error(`Unknown identifier: ${token.value}`)
      return { type: 'function', name: token.value, args: this.parseFunctionArgs() }
    }
    if (token.type === 'leftParen') {
      this.index += 1
      const expression = this.parseExpression(0)
      this.expect('rightParen')
      return expression
    }
    throw new Error('Expected expression')
  }

  private parseFunctionArgs() {
    if (this.current().type !== 'leftParen') return [this.parseUnary()]
    this.index += 1
    const args: ExpressionNode[] = []
    if (this.current().type === 'rightParen') throw new Error('Missing function argument')
    while (true) {
      args.push(this.parseExpression(0))
      if (this.current().type !== 'comma') break
      this.index += 1
    }
    this.expect('rightParen')
    return args
  }

  private expect(type: Token['type']) {
    if (this.current().type !== type) throw new Error(`Expected ${type}`)
    this.index += 1
  }

  private current() {
    return this.tokens[this.index]
  }
}

function precedence(operator: CalculatorOperator) {
  if (operator === '+' || operator === '-') return 1
  if (operator === '*' || operator === '/') return 2
  if (operator === '^') return 3
  return 0
}

function evaluateNode(node: ExpressionNode): number {
  switch (node.type) {
    case 'number':
      return node.value
    case 'percent':
      return evaluateNode(node.value) / 100
    case 'unary': {
      const value = evaluateNode(node.value)
      return node.operator === '-' ? -value : value
    }
    case 'binary': {
      const left = evaluateNode(node.left)
      if ((node.operator === '+' || node.operator === '-') && node.right.type === 'percent') {
        const percentage = evaluateNode(node.right)
        return node.operator === '+' ? left + left * percentage : left - left * percentage
      }
      const right = evaluateNode(node.right)
      if (node.operator === '+') return left + right
      if (node.operator === '-') return left - right
      if (node.operator === '*') return left * right
      if (node.operator === '/') return left / right
      return Math.pow(left, right)
    }
    case 'function':
      return evaluateFunction(node.name, node.args.map(evaluateNode))
  }
}

function evaluateFunction(name: string, args: number[]) {
  const arity = FUNCTION_ARITY[name]
  if (!arity || args.length < arity.min || args.length > arity.max) throw new Error('Invalid function arity')
  if (name === 'sqrt') return Math.sqrt(args[0])
  if (name === 'abs') return Math.abs(args[0])
  if (name === 'round') return Math.round(args[0])
  if (name === 'floor') return Math.floor(args[0])
  if (name === 'ceil') return Math.ceil(args[0])
  if (name === 'sin') return Math.sin(args[0])
  if (name === 'cos') return Math.cos(args[0])
  if (name === 'tan') return Math.tan(args[0])
  if (name === 'log') return Math.log10(args[0])
  if (name === 'ln') return Math.log(args[0])
  if (name === 'min') return Math.min(...args)
  if (name === 'max') return Math.max(...args)
  throw new Error(`Unknown function: ${name}`)
}

function normalizeFloatingPoint(value: number) {
  return Number(Number(value).toPrecision(12))
}

function formatRawNumber(value: number) {
  const normalized = normalizeFloatingPoint(value)
  if (Number.isInteger(normalized)) return String(normalized)
  return String(normalized)
}

function formatDisplayNumber(value: number) {
  const normalized = normalizeFloatingPoint(value)
  return new Intl.NumberFormat(undefined, { maximumFractionDigits: 12 }).format(normalized)
}
