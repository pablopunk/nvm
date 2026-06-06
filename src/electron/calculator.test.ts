import assert from 'node:assert/strict'
import test from 'node:test'
import { calculate, calculateDetailed, calculateRateResult, parseRateExpression } from './calculator'

function raw(query: string) {
  return calculateDetailed(query)?.raw ?? null
}

function formatted(query: string) {
  return calculateDetailed(query)?.formatted ?? null
}

test('evaluates arithmetic without executing JavaScript', () => {
  assert.equal(raw('1 + 2 * 3'), '7')
  assert.equal(raw('(1 + 2) * 3'), '9')
  assert.equal(raw('2^10'), '1024')
  assert.equal(raw('= 1 + 2'), '3')
  assert.equal(raw('calc 1 + 2'), '3')
  assert.equal(raw('calculate 1 + 2'), '3')
  assert.equal(raw('process.exit()'), null)
  assert.equal(raw('Math.max(1,2)'), null)
})

test('supports constants and math functions', () => {
  assert.equal(raw('sqrt(625)'), '25')
  assert.equal(raw('square root of 625'), '25')
  assert.equal(raw('sin(pi/2)'), '1')
  assert.equal(raw('min(3, 1, 2)'), '1')
  assert.equal(raw('max(3, 1, 2)'), '3')
  assert.equal(raw('round(1.4)'), '1')
})

test('supports natural power phrases', () => {
  assert.equal(raw('2 power 10'), '1024')
  assert.equal(raw('4 power 6'), '4096')
  assert.equal(raw('5 squared'), '25')
  assert.equal(raw('5 cubed'), '125')
})

test('supports large number suffixes in arithmetic contexts', () => {
  assert.equal(raw('50k + 20k'), '70000')
  assert.equal(formatted('50k + 20k'), '70,000')
  assert.equal(raw('1.2m / 3'), '400000')
  assert.equal(raw('3b / 15'), '200000000')
  assert.equal(raw('50 k + 20 k'), '70000')
})

test('supports human percent semantics', () => {
  assert.equal(raw('32% of 5'), '1.6')
  assert.equal(raw('20% * 150'), '30')
  assert.equal(raw('150 + 20%'), '180')
  assert.equal(raw('200 - 15%'), '170')
  assert.equal(raw('50 * 10%'), '5')
  assert.equal(raw('50 / 10%'), '500')
})

test('supports simple unit-bearing percent changes', () => {
  assert.equal(raw('19m + 47%'), '27.93 m')
  assert.equal(formatted('19m + 47%'), '27.93 m')
  assert.equal(raw('19m - 47%'), '10.07 m')
})

test('converts local offline units', () => {
  assert.equal(raw('10ft in m'), '3.048 m')
  assert.equal(raw('500 miles to km'), '804.672 km')
  assert.equal(raw('5kg in lbs'), '11.0231 lb')
  assert.equal(raw('100 c to f'), '212 °F')
  assert.equal(raw('23C to F'), '73.4 °F')
  assert.equal(raw('29 inches to cm'), '73.66 cm')
  assert.equal(raw('4 feet to cm'), '121.92 cm')
  assert.equal(raw('3 teaspoon in ml'), '14.7868 ml')
  assert.equal(raw('16px to rem'), '1 rem')
  assert.equal(raw('145 mins to timespan'), '2h 25m')
})

test('keeps conversion dimensions strict', () => {
  assert.equal(raw('10ft to kg'), null)
  assert.equal(raw('10kg to timespan'), null)
})

test('handles local date and timezone questions', () => {
  const now = new Date('2026-06-06T12:00:00Z')
  assert.equal(calculateDetailed('days until 25 Dec', { now })?.raw, '202 days')
  assert.equal(calculateDetailed('35 days ago', { now })?.raw, '2026-05-02')
  assert.equal(calculateDetailed('Monday in 3 weeks', { now })?.raw, '2026-06-29')
  assert.equal(calculateDetailed('time in Tokyo', { now })?.raw, '21:00')
  assert.equal(calculateDetailed('5pm ldn in sf', { now })?.raw, '09:00')
})

test('parses and formats cached fiat rates', () => {
  const quote = { rate: 0.8, provider: 'Frankfurter', updatedAt: Date.now(), fetchedAt: Date.now() }
  const usdToGbp = parseRateExpression('10 usd in gbp')
  assert.deepEqual(usdToGbp && { amount: usdToGbp.amount, sourceCurrency: usdToGbp.sourceCurrency, targetCurrency: usdToGbp.targetCurrency }, { amount: 10, sourceCurrency: 'USD', targetCurrency: 'GBP' })
  assert.equal(calculateRateResult('10 usd in gbp', usdToGbp!, quote)?.raw, '8 GBP')

  const prefixed = parseRateExpression('$1.2m in gbp')
  assert.deepEqual(prefixed && { amount: prefixed.amount, sourceCurrency: prefixed.sourceCurrency, targetCurrency: prefixed.targetCurrency }, { amount: 1_200_000, sourceCurrency: 'USD', targetCurrency: 'GBP' })
  assert.equal(calculateRateResult('$1.2m in gbp', prefixed!, quote)?.raw, '960000 GBP')

  const hourly = parseRateExpression('8 dollars/hour in gbp')
  assert.deepEqual(hourly && { amount: hourly.amount, sourceCurrency: hourly.sourceCurrency, targetCurrency: hourly.targetCurrency, rateUnit: hourly.rateUnit }, { amount: 8, sourceCurrency: 'USD', targetCurrency: 'GBP', rateUnit: 'hour' })
  assert.equal(calculateRateResult('8 dollars/hour in gbp', hourly!, quote)?.raw, '6.4 GBP/hour')
})

test('keeps compatibility wrapper raw and ungrouped', () => {
  assert.equal(calculate('1,000 + 2,500'), '3500')
  assert.equal(formatted('1,000 + 2,500'), '3,500')
  assert.equal(calculate('10ft in m'), '3.048 m')
})

test('does not trigger for plain non-calculator queries', () => {
  assert.equal(raw('calendar'), null)
  assert.equal(raw('hello world'), null)
  assert.equal(raw('123'), null)
})
