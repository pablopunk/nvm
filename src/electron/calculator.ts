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

type UnitDimension = 'length' | 'mass' | 'volume' | 'data' | 'time' | 'design' | 'temperature'

type UnitDefinition = {
  dimension: UnitDimension
  symbol: string
  toBase: (value: number) => number
  fromBase: (value: number) => number
}

export type RateExpression = {
  amount: number
  sourceCurrency: string
  targetCurrency: string
  expression: string
  rateUnit?: string
}

export type RateQuote = {
  rate: number
  provider: string
  updatedAt: number
  fetchedAt: number
}

type CalculatorOptions = { now?: Date | number | string }

type ZonedParts = { year: number; month: number; day: number; hour: number; minute: number }

const UNIT_BY_ALIAS = createUnitMap()
const CURRENCY_BY_ALIAS = createCurrencyMap()
const TIMEZONE_BY_ALIAS = createTimezoneMap()
const WEEKDAY_INDEX: Record<string, number> = { sunday: 0, sun: 0, monday: 1, mon: 1, tuesday: 2, tue: 2, wednesday: 3, wed: 3, thursday: 4, thu: 4, friday: 5, fri: 5, saturday: 6, sat: 6 }
const MONTH_INDEX: Record<string, number> = { jan: 0, january: 0, feb: 1, february: 1, mar: 2, march: 2, apr: 3, april: 3, may: 4, jun: 5, june: 5, jul: 6, july: 6, aug: 7, august: 7, sep: 8, sept: 8, september: 8, oct: 9, october: 9, nov: 10, november: 10, dec: 11, december: 11 }

function createUnitMap() {
  const units = new Map<string, UnitDefinition>()
  const addLinear = (dimension: UnitDimension, symbol: string, factor: number, aliases: string[]) => {
    addUnit(units, { dimension, symbol, toBase: (value) => value * factor, fromBase: (value) => value / factor }, aliases)
  }
  const addTemperature = (symbol: string, toBase: (value: number) => number, fromBase: (value: number) => number, aliases: string[]) => {
    addUnit(units, { dimension: 'temperature', symbol, toBase, fromBase }, aliases)
  }

  addLinear('length', 'mm', 0.001, ['mm', 'millimeter', 'millimeters'])
  addLinear('length', 'cm', 0.01, ['cm', 'centimeter', 'centimeters'])
  addLinear('length', 'm', 1, ['m', 'meter', 'meters', 'metre', 'metres'])
  addLinear('length', 'km', 1000, ['km', 'kilometer', 'kilometers', 'kilometre', 'kilometres'])
  addLinear('length', 'in', 0.0254, ['in', 'inch', 'inches'])
  addLinear('length', 'ft', 0.3048, ['ft', 'foot', 'feet'])
  addLinear('length', 'yd', 0.9144, ['yd', 'yard', 'yards'])
  addLinear('length', 'mi', 1609.344, ['mi', 'mile', 'miles'])

  addLinear('mass', 'mg', 0.000001, ['mg', 'milligram', 'milligrams'])
  addLinear('mass', 'g', 0.001, ['g', 'gram', 'grams'])
  addLinear('mass', 'kg', 1, ['kg', 'kilogram', 'kilograms'])
  addLinear('mass', 'oz', 0.028349523125, ['oz', 'ounce', 'ounces'])
  addLinear('mass', 'lb', 0.45359237, ['lb', 'lbs', 'pound', 'pounds'])

  addLinear('volume', 'ml', 0.001, ['ml', 'milliliter', 'milliliters', 'millilitre', 'millilitres'])
  addLinear('volume', 'l', 1, ['l', 'liter', 'liters', 'litre', 'litres'])
  addLinear('volume', 'tsp', 0.00492892159375, ['tsp', 'teaspoon', 'teaspoons'])
  addLinear('volume', 'tbsp', 0.01478676478125, ['tbsp', 'tablespoon', 'tablespoons'])
  addLinear('volume', 'cup', 0.2365882365, ['cup', 'cups'])
  addLinear('volume', 'pint', 0.473176473, ['pint', 'pints'])
  addLinear('volume', 'quart', 0.946352946, ['quart', 'quarts'])
  addLinear('volume', 'gal', 3.785411784, ['gal', 'gallon', 'gallons'])

  addLinear('data', 'B', 1, ['b', 'byte', 'bytes'])
  addLinear('data', 'KB', 1000, ['kb', 'kilobyte', 'kilobytes'])
  addLinear('data', 'MB', 1000 ** 2, ['mb', 'megabyte', 'megabytes'])
  addLinear('data', 'GB', 1000 ** 3, ['gb', 'gigabyte', 'gigabytes'])
  addLinear('data', 'TB', 1000 ** 4, ['tb', 'terabyte', 'terabytes'])
  addLinear('data', 'KiB', 1024, ['kib', 'kibibyte', 'kibibytes'])
  addLinear('data', 'MiB', 1024 ** 2, ['mib', 'mebibyte', 'mebibytes'])
  addLinear('data', 'GiB', 1024 ** 3, ['gib', 'gibibyte', 'gibibytes'])
  addLinear('data', 'TiB', 1024 ** 4, ['tib', 'tebibyte', 'tebibytes'])

  addLinear('time', 'ms', 0.001, ['ms', 'millisecond', 'milliseconds'])
  addLinear('time', 's', 1, ['s', 'sec', 'secs', 'second', 'seconds'])
  addLinear('time', 'min', 60, ['min', 'mins', 'minute', 'minutes'])
  addLinear('time', 'h', 3600, ['h', 'hr', 'hrs', 'hour', 'hours'])
  addLinear('time', 'day', 86400, ['day', 'days'])
  addLinear('time', 'week', 604800, ['week', 'weeks'])

  addLinear('design', 'px', 1, ['px', 'pixel', 'pixels'])
  addLinear('design', 'rem', 16, ['rem', 'rems'])

  addTemperature('°C', (value) => value, (value) => value, ['c', '°c', 'celsius'])
  addTemperature('°F', (value) => (value - 32) * 5 / 9, (value) => value * 9 / 5 + 32, ['f', '°f', 'fahrenheit'])
  addTemperature('K', (value) => value - 273.15, (value) => value + 273.15, ['k', 'kelvin'])
  return units
}

function addUnit(units: Map<string, UnitDefinition>, unit: UnitDefinition, aliases: string[]) {
  for (const alias of aliases) units.set(normalizeUnitAlias(alias), unit)
}

function createCurrencyMap() {
  const currencies = new Map<string, string>()
  const add = (code: string, aliases: string[]) => {
    currencies.set(code.toLowerCase(), code)
    for (const alias of aliases) currencies.set(normalizeCurrencyAlias(alias), code)
  }
  add('USD', ['$', 'dollar', 'dollars', 'usd'])
  add('EUR', ['€', 'euro', 'euros', 'eur'])
  add('GBP', ['£', 'pound', 'pounds', 'sterling', 'gbp'])
  add('JPY', ['¥', 'yen', 'jpy'])
  add('INR', ['₹', 'rupee', 'rupees', 'inr'])
  add('CHF', ['franc', 'francs', 'chf'])
  add('CAD', ['cad'])
  add('AUD', ['aud'])
  add('NZD', ['nzd'])
  add('CNY', ['yuan', 'rmb', 'cny'])
  return currencies
}

function createTimezoneMap() {
  const zones = new Map<string, string>()
  const add = (zone: string, aliases: string[]) => {
    zones.set(normalizeTimezoneAlias(zone), zone)
    for (const alias of aliases) zones.set(normalizeTimezoneAlias(alias), zone)
  }
  add('Europe/London', ['ldn', 'london', 'uk', 'gmt', 'bst'])
  add('America/Los_Angeles', ['sf', 'sfo', 'san francisco', 'la', 'los angeles', 'pt', 'pst', 'pdt'])
  add('America/New_York', ['nyc', 'new york', 'ny', 'et', 'est', 'edt'])
  add('Asia/Tokyo', ['tokyo', 'jst', 'jp'])
  add('Asia/Dubai', ['dubai', 'uae'])
  add('Europe/Madrid', ['madrid', 'spain', 'cet', 'cest'])
  add('Europe/Paris', ['paris', 'france'])
  add('Europe/Berlin', ['berlin', 'germany'])
  add('UTC', ['utc', 'z'])
  return zones
}

export function calculate(query: string, options: CalculatorOptions = {}) {
  return calculateDetailed(query, options)?.raw ?? null
}

export function calculateDetailed(query: string, options: CalculatorOptions = {}): CalculatorResult | null {
  const normalized = normalizeCalculationQuery(query)
  if (!normalized || !isLikelyCalculation(normalized.expression, normalized.explicit)) return null

  const dateTimeResult = calculateDateTimeExpression(normalized.expression, query, options)
  if (dateTimeResult) return dateTimeResult

  const conversionResult = calculateConversionExpression(normalized.expression, query)
  if (conversionResult) return conversionResult

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
  if (looksLikeDateTimeExpression(expression)) return true
  if (looksLikeConversionExpression(expression)) return true
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

function looksLikeConversionExpression(expression: string) {
  return conversionExpressionMatch(expression) !== null || parseRateExpression(expression) !== null
}

function looksLikeDateTimeExpression(expression: string) {
  return /^(?:(?:days?\s+until\s+)?(?:\d{1,2}\s+[a-z]+|[a-z]+\s+\d{1,2})|\d+\s+days?\s+ago|(?:next\s+)?(?:sun|mon|tue|wed|thu|fri|sat|sunday|monday|tuesday|wednesday|thursday|friday|saturday)(?:\s+in\s+\d+\s+weeks?)?|tomorrow|yesterday|time\s+in\s+.+|\d{1,2}(?::\d{2})?\s*(?:am|pm)?\s+.+\s+in\s+.+)$/i.test(expression.trim())
}

function calculateDateTimeExpression(expression: string, query: string, options: CalculatorOptions): CalculatorResult | null {
  return calculateDateExpression(expression, query, options) || calculateTimezoneExpression(expression, query, options)
}

function calculateDateExpression(expression: string, query: string, options: CalculatorOptions): CalculatorResult | null {
  const now = optionDate(options)
  const today = startOfLocalDay(now)
  const monthDay = parseMonthDayExpression(expression)
  if (monthDay) {
    const target = nextMonthDay(monthDay.day, monthDay.month, today)
    if (!target) return null
    const days = Math.round((target.getTime() - today.getTime()) / 86400000)
    if (monthDay.until) {
      const text = `${days} ${days === 1 ? 'day' : 'days'}`
      return dateResult('date', query, expression, days, text, text, `Until ${formatDateDisplay(target)}`)
    }
    return dateResult('date', query, expression, target.getTime(), isoLocalDate(target), formatDateDisplay(target), 'Date')
  }

  if (/^tomorrow$/i.test(expression.trim())) {
    const date = addLocalDays(today, 1)
    return dateResult('date', query, expression, date.getTime(), isoLocalDate(date), formatDateDisplay(date), 'Date')
  }

  if (/^yesterday$/i.test(expression.trim())) {
    const date = addLocalDays(today, -1)
    return dateResult('date', query, expression, date.getTime(), isoLocalDate(date), formatDateDisplay(date), 'Date')
  }

  const daysAgo = expression.trim().match(/^(\d+)\s+days?\s+ago$/i)
  if (daysAgo) {
    const date = addLocalDays(today, -Number(daysAgo[1]))
    return dateResult('date', query, expression, date.getTime(), isoLocalDate(date), formatDateDisplay(date), 'Date')
  }

  const weekdayInWeeks = expression.trim().match(/^(?:next\s+)?([a-z]+)(?:\s+in\s+(\d+)\s+weeks?)?$/i)
  if (weekdayInWeeks) {
    const weekday = WEEKDAY_INDEX[weekdayInWeeks[1].toLowerCase()]
    if (weekday === undefined) return null
    const date = addLocalDays(nextWeekday(today, weekday), Number(weekdayInWeeks[2] || 0) * 7)
    return dateResult('date', query, expression, date.getTime(), isoLocalDate(date), formatDateDisplay(date), 'Date')
  }
  return null
}

function calculateTimezoneExpression(expression: string, query: string, options: CalculatorOptions): CalculatorResult | null {
  const now = optionDate(options)
  const currentTime = expression.trim().match(/^time\s+in\s+(.+)$/i)
  if (currentTime) {
    const zone = timezoneForAlias(currentTime[1])
    if (!zone) return null
    const formatted = formatTimeInZone(now, zone)
    return timezoneResult(query, expression, now.getTime(), formatted, zone)
  }

  const conversion = expression.trim().match(/^(\d{1,2})(?::(\d{2}))?\s*(am|pm)?\s+(.+?)\s+in\s+(.+)$/i)
  if (!conversion) return null
  const sourceZone = timezoneForAlias(conversion[4])
  const targetZone = timezoneForAlias(conversion[5])
  if (!sourceZone || !targetZone) return null
  const hour = parseClockHour(Number(conversion[1]), conversion[3])
  const minute = conversion[2] ? Number(conversion[2]) : 0
  if (hour === null || minute < 0 || minute > 59) return null
  const sourceParts = zonedParts(now, sourceZone)
  const instant = instantForZonedTime(sourceZone, { year: sourceParts.year, month: sourceParts.month, day: sourceParts.day, hour, minute })
  if (!instant) return null
  const formatted = formatTimeInZone(instant, targetZone)
  return timezoneResult(query, expression, instant.getTime(), formatted, targetZone)
}

function dateResult(kind: 'date', query: string, expression: string, value: number, raw: string, formatted: string, subtitle = 'Copy result to clipboard'): CalculatorResult {
  return { kind, query: query.trim(), expression: expression.trim(), value, raw, formatted, date: raw, title: `${expression.trim()} = ${formatted}`, subtitle, alternate: raw === formatted ? undefined : { raw, formatted } }
}

function timezoneResult(query: string, expression: string, value: number, formatted: string, zone: string): CalculatorResult {
  return { kind: 'timezone', query: query.trim(), expression: expression.trim(), value, raw: formatted, formatted, timezone: zone, title: `${expression.trim()} = ${formatted}`, subtitle: `Time in ${zone}` }
}

export function calculateRateResult(query: string, parsed: RateExpression, quote: RateQuote): CalculatorResult | null {
  const value = normalizeFloatingPoint(parsed.amount * quote.rate)
  if (!Number.isFinite(value)) return null
  const rawAmount = formatRawNumber(value, 4)
  const formattedAmount = formatCurrencyDisplay(value, parsed.targetCurrency)
  const raw = parsed.rateUnit ? `${rawAmount} ${parsed.targetCurrency}/${parsed.rateUnit}` : `${rawAmount} ${parsed.targetCurrency}`
  const formatted = parsed.rateUnit ? `${formattedAmount}/${parsed.rateUnit}` : formattedAmount
  const age = rateAgeLabel(quote.fetchedAt)
  return {
    kind: 'rate',
    query: query.trim(),
    expression: parsed.expression,
    value,
    raw,
    formatted,
    unit: parsed.sourceCurrency,
    targetUnit: parsed.targetCurrency,
    rateUnit: parsed.rateUnit,
    title: `${parsed.expression} = ${formatted}`,
    subtitle: `Copy converted result to clipboard · ${quote.provider}${age ? ` · ${age}` : ''}`,
    alternate: raw === formatted ? undefined : { raw, formatted },
    swapQuery: `${rawAmount} ${parsed.targetCurrency}${parsed.rateUnit ? `/${parsed.rateUnit}` : ''} to ${parsed.sourceCurrency}`,
  }
}

export function parseRateExpression(query: string): RateExpression | null {
  const normalized = normalizeCalculationQuery(query)
  if (!normalized) return null
  const expression = normalized.expression
  const amount = '([+-]?(?:(?:\\d+(?:,\\d{3})*)|(?:\\d+)|(?:\\d*\\.\\d+))(?:\\.\\d+)?\\s*[kmb]?)'
  const currency = '([$€£¥₹]|[a-zA-Z]{3,})'
  const rateUnit = '(?:\\s*/\\s*([a-zA-Z]+))?'
  const suffixMatch = expression.trim().match(new RegExp(`^${amount}\\s*${currency}${rateUnit}\\s+(?:in|to|as)\\s+${currency}$`, 'i'))
  const prefixMatch = expression.trim().match(new RegExp(`^${currency}\\s*${amount}${rateUnit}\\s+(?:in|to|as)\\s+${currency}$`, 'i'))
  const sourceAlias = suffixMatch?.[2] || prefixMatch?.[1]
  const amountValue = suffixMatch?.[1] || prefixMatch?.[2]
  const rateUnitValue = suffixMatch?.[3] || prefixMatch?.[3]
  const targetAlias = suffixMatch?.[4] || prefixMatch?.[4]
  if (!sourceAlias || !amountValue || !targetAlias) return null
  const sourceCurrency = currencyForAlias(sourceAlias)
  const targetCurrency = currencyForAlias(targetAlias)
  if (!sourceCurrency || !targetCurrency || sourceCurrency === targetCurrency) return null
  const parsedAmount = parseAmountWithSuffix(amountValue)
  if (parsedAmount === null) return null
  return {
    amount: parsedAmount,
    sourceCurrency,
    targetCurrency,
    expression: expression.trim(),
    rateUnit: rateUnitValue ? normalizeRateUnit(rateUnitValue) : undefined,
  }
}

function calculateConversionExpression(expression: string, query: string): CalculatorResult | null {
  const match = conversionExpressionMatch(expression)
  if (!match) return null
  const amount = parseNumberLiteral(match.amount)
  if (amount === null) return null
  const source = unitForAlias(match.sourceUnit)
  if (!source) return null

  const targetAlias = normalizeUnitAlias(match.targetUnit)
  const isTimespan = targetAlias === 'timespan'
  const target = isTimespan ? null : unitForAlias(match.targetUnit)
  if (isTimespan && source.dimension !== 'time') return null
  if (!isTimespan && (!target || source.dimension !== target.dimension)) return null

  const baseValue = source.toBase(amount)
  const value = isTimespan ? baseValue : target!.fromBase(baseValue)
  if (!Number.isFinite(value)) return null

  const raw = isTimespan ? formatTimespan(value) : `${formatRawNumber(value, 4)} ${target!.symbol}`
  const formatted = isTimespan ? raw : `${formatDisplayNumber(value, 4)} ${target!.symbol}`
  const swapQuery = isTimespan ? undefined : `${formatRawNumber(value, 4)} ${target!.symbol} to ${source.symbol}`
  return {
    kind: 'conversion',
    query: query.trim(),
    expression: expression.trim(),
    value,
    raw,
    formatted,
    unit: source.symbol,
    targetUnit: isTimespan ? 'timespan' : target!.symbol,
    title: `${expression.trim()} = ${formatted}`,
    subtitle: 'Copy converted result to clipboard',
    alternate: raw === formatted ? undefined : { raw, formatted },
    swapQuery,
  }
}

function conversionExpressionMatch(expression: string) {
  const number = '[+-]?(?:(?:\\d+(?:,\\d{3})*)|(?:\\d+)|(?:\\d*\\.\\d+))(?:\\.\\d+)?'
  const unit = '[a-zA-Z°]+'
  const match = expression.trim().match(new RegExp(`^(?:convert\\s+)?(${number})\\s*(${unit})\\s+(?:in|to|as)\\s+(${unit}|timespan)$`, 'i'))
  if (!match) return null
  return { amount: match[1], sourceUnit: match[2], targetUnit: match[3] }
}

function unitForAlias(alias: string) {
  return UNIT_BY_ALIAS.get(normalizeUnitAlias(alias)) || null
}

function normalizeUnitAlias(alias: string) {
  return String(alias || '').trim().toLowerCase().replace(/\./g, '')
}

function optionDate(options: CalculatorOptions) {
  const date = options.now === undefined ? new Date() : new Date(options.now)
  return Number.isFinite(date.getTime()) ? date : new Date()
}

function parseMonthDayExpression(expression: string) {
  const trimmed = expression.trim()
  const dayFirst = trimmed.match(/^(days?\s+until\s+)?(\d{1,2})\s+([a-z]+)$/i)
  if (dayFirst) return { until: Boolean(dayFirst[1]), day: Number(dayFirst[2]), month: dayFirst[3] }
  const monthFirst = trimmed.match(/^(days?\s+until\s+)?([a-z]+)\s+(\d{1,2})$/i)
  if (monthFirst) return { until: Boolean(monthFirst[1]), day: Number(monthFirst[3]), month: monthFirst[2] }
  return null
}

function startOfLocalDay(date: Date) {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate())
}

function addLocalDays(date: Date, days: number) {
  const next = new Date(date)
  next.setDate(next.getDate() + days)
  return startOfLocalDay(next)
}

function nextMonthDay(day: number, monthName: string, today: Date) {
  const month = MONTH_INDEX[monthName.toLowerCase()]
  if (month === undefined || day < 1 || day > 31) return null
  let date = new Date(today.getFullYear(), month, day)
  if (date.getMonth() !== month) return null
  if (date.getTime() < today.getTime()) date = new Date(today.getFullYear() + 1, month, day)
  return date
}

function nextWeekday(today: Date, weekday: number) {
  const delta = (weekday - today.getDay() + 7) % 7 || 7
  return addLocalDays(today, delta)
}

function isoLocalDate(date: Date) {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

function formatDateDisplay(date: Date) {
  return new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric', year: 'numeric' }).format(date)
}

function timezoneForAlias(alias: string) {
  const trimmed = String(alias || '').trim()
  const known = TIMEZONE_BY_ALIAS.get(normalizeTimezoneAlias(trimmed))
  if (known) return known
  try {
    new Intl.DateTimeFormat('en-US', { timeZone: trimmed }).format(new Date())
    return trimmed
  } catch {
    return null
  }
}

function normalizeTimezoneAlias(alias: string) {
  return String(alias || '').trim().toLowerCase().replace(/[._-]+/g, ' ').replace(/\s+/g, ' ')
}

function parseClockHour(hour: number, meridiem?: string) {
  if (hour < 0 || hour > 23) return null
  if (!meridiem) return hour
  if (hour < 1 || hour > 12) return null
  const lower = meridiem.toLowerCase()
  if (lower === 'am') return hour === 12 ? 0 : hour
  return hour === 12 ? 12 : hour + 12
}

function zonedParts(date: Date, timeZone: string): ZonedParts {
  const parts = new Intl.DateTimeFormat('en-US', { timeZone, year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false }).formatToParts(date)
  const value = (type: string) => Number(parts.find((part) => part.type === type)?.value || 0)
  const hour = value('hour')
  return { year: value('year'), month: value('month'), day: value('day'), hour: hour === 24 ? 0 : hour, minute: value('minute') }
}

function instantForZonedTime(timeZone: string, target: ZonedParts) {
  const localTimestamp = Date.UTC(target.year, target.month - 1, target.day, target.hour, target.minute)
  let candidate = new Date(localTimestamp - timezoneOffsetMs(new Date(localTimestamp), timeZone))
  for (let attempt = 0; attempt < 3; attempt += 1) {
    const parts = zonedParts(candidate, timeZone)
    if (parts.year === target.year && parts.month === target.month && parts.day === target.day && parts.hour === target.hour && parts.minute === target.minute) return candidate
    const represented = Date.UTC(parts.year, parts.month - 1, parts.day, parts.hour, parts.minute)
    candidate = new Date(candidate.getTime() + localTimestamp - represented)
  }
  return null
}

function timezoneOffsetMs(date: Date, timeZone: string) {
  const parts = zonedParts(date, timeZone)
  return Date.UTC(parts.year, parts.month - 1, parts.day, parts.hour, parts.minute) - Math.floor(date.getTime() / 60000) * 60000
}

function formatTimeInZone(date: Date, timeZone: string) {
  return new Intl.DateTimeFormat(undefined, { timeZone, hour: '2-digit', minute: '2-digit', hour12: false }).format(date)
}

function currencyForAlias(alias: string) {
  const normalized = normalizeCurrencyAlias(alias)
  if (/^[a-z]{3}$/.test(normalized)) return normalized.toUpperCase()
  return CURRENCY_BY_ALIAS.get(normalized) || null
}

function normalizeCurrencyAlias(alias: string) {
  return String(alias || '').trim().toLowerCase().replace(/\./g, '')
}

function normalizeRateUnit(unit: string) {
  const normalized = String(unit || '').trim().toLowerCase()
  return normalized === 'hours' ? 'hour' : normalized.endsWith('s') ? normalized.slice(0, -1) : normalized
}

function parseAmountWithSuffix(value: string) {
  const trimmed = String(value || '').trim()
  const suffix = trimmed.match(/[kmb]$/i)?.[0]?.toLowerCase()
  const number = suffix ? trimmed.slice(0, -1) : trimmed
  const parsed = parseNumberLiteral(number.trim())
  if (parsed === null) return null
  return parsed * (suffix ? SUFFIX_MULTIPLIERS[suffix] : 1)
}

function rateAgeLabel(fetchedAt: number) {
  if (!fetchedAt) return ''
  const ageMs = Date.now() - fetchedAt
  if (ageMs < 0) return ''
  const hours = Math.floor(ageMs / 36e5)
  if (hours < 1) return 'fresh'
  if (hours < 48) return `${hours}h old`
  return `${Math.floor(hours / 24)}d old`
}

function formatCurrencyDisplay(value: number, currency: string) {
  try {
    return new Intl.NumberFormat(undefined, { style: 'currency', currency, maximumFractionDigits: 4 }).format(normalizeFloatingPoint(value))
  } catch {
    return `${formatDisplayNumber(value, 4)} ${currency}`
  }
}

function formatTimespan(seconds: number) {
  const sign = seconds < 0 ? '-' : ''
  let remaining = Math.round(Math.abs(seconds))
  const days = Math.floor(remaining / 86400)
  remaining -= days * 86400
  const hours = Math.floor(remaining / 3600)
  remaining -= hours * 3600
  const minutes = Math.floor(remaining / 60)
  remaining -= minutes * 60
  const parts = []
  if (days) parts.push(`${days}d`)
  if (hours) parts.push(`${hours}h`)
  if (minutes) parts.push(`${minutes}m`)
  if (remaining || parts.length === 0) parts.push(`${remaining}s`)
  return `${sign}${parts.join(' ')}`
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

function formatRawNumber(value: number, maximumFractionDigits = 12) {
  const normalized = normalizeFloatingPoint(value)
  if (Number.isInteger(normalized)) return String(normalized)
  return normalized.toLocaleString('en-US', { useGrouping: false, maximumFractionDigits })
}

function formatDisplayNumber(value: number, maximumFractionDigits = 12) {
  const normalized = normalizeFloatingPoint(value)
  return new Intl.NumberFormat(undefined, { maximumFractionDigits }).format(normalized)
}
