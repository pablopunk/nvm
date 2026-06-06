import assert from 'node:assert/strict'
import test from 'node:test'
import { calculate, calculateDetailed } from './calculator'

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

test('keeps compatibility wrapper raw and ungrouped', () => {
  assert.equal(calculate('1,000 + 2,500'), '3500')
  assert.equal(formatted('1,000 + 2,500'), '3,500')
})

test('does not trigger for plain non-calculator queries', () => {
  assert.equal(raw('calendar'), null)
  assert.equal(raw('hello world'), null)
  assert.equal(raw('123'), null)
})
