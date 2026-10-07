import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';

const profileSource = readFileSync(new URL('./profile.astro', import.meta.url), 'utf8');

function sourceBlock(start: string, end: string): string {
  const startIndex = profileSource.indexOf(start);
  const endIndex = profileSource.indexOf(end, startIndex + start.length);
  assert.notEqual(startIndex, -1, `missing profile caller block: ${start}`);
  assert.notEqual(endIndex, -1, `missing end of profile caller block: ${end}`);
  return profileSource.slice(startIndex, endIndex);
}

test('profile stores checkout attempts under a per-user key before sending checkout POST', () => {
  assert.match(profileSource, /nevermind\.billing\.attempt\.' \+ billingStatus\.dataset\.userId/);
  const createCall = sourceBlock('async function createBillingCheckout(button)', "document.querySelectorAll('.billing-checkout')");
  assert.ok(createCall.indexOf('saveBillingAttempt(attempt)') < createCall.indexOf("fetch('/api/billing/checkout'"));
  assert.match(createCall, /key: crypto\.randomUUID\(\)/);
  assert.match(createCall, /'Idempotency-Key': attempt\.key/);
});

test('profile retains saved attempts after none and lost responses, clearing only terminal or not_created outcomes', () => {
  const resultPolicy = sourceBlock('function showBillingResult(data)', 'function addAction(label, action)');
  const notCreatedPolicy = sourceBlock('async function readBillingResponse(response)', 'async function refreshBillingStatus()');
  assert.match(resultPolicy, /No checkout order is currently recorded\. Your saved attempt remains available/);
  assert.match(resultPolicy, /\['completed', 'expired', 'failed'\]\.includes\(data\.status\).*clearBillingAttempt\(\)/s);
  assert.match(notCreatedPolicy, /data\.status === 'not_created'[\s\S]*?clearBillingAttempt\(\)/);
  assert.match(profileSource, /Checkout response was lost\. Your attempt is saved/);
  assert.match(profileSource, /Could not load checkout status\. Any saved attempt remains retained/);
  assert.match(profileSource, /session_id/);
  assert.match(profileSource, /hasStripeReturn \? 'POST' : 'GET'/);
});

test('profile page load and status refresh do not invoke PaymentIntent cancellation', () => {
  const statusCallers = sourceBlock('async function refreshBillingStatus()', 'async function retryBilling(status)');
  const pageLoad = sourceBlock('const billingQuery = new URLSearchParams(location.search);', "document.querySelectorAll('.revoke')");
  assert.doesNotMatch(statusCallers, /cancelPaymentIntent|paymentIntents\.cancel/);
  assert.doesNotMatch(pageLoad, /cancelPaymentIntent|paymentIntents\.cancel/);
  assert.match(pageLoad, /fetch\('\/api\/billing\/checkout-status/);
});
