import assert from 'node:assert/strict';
import { test } from 'node:test';
import { isBillingPriceApprover } from './admin';

test('billing Price approval requires the configured WorkOS admin identity', () => {
  const ownerAdmin = {
    role: 'admin',
    workosUserId: 'owner-workos-user',
  } as const;
  const otherAdmin = {
    role: 'admin',
    workosUserId: 'other-workos-user',
  } as const;
  const ownerWithoutAdminRole = {
    role: 'user',
    workosUserId: 'owner-workos-user',
  } as const;

  assert.equal(isBillingPriceApprover(ownerAdmin, undefined), false);
  assert.equal(isBillingPriceApprover(ownerAdmin, 'owner-workos-user'), true);
  assert.equal(isBillingPriceApprover(otherAdmin, 'owner-workos-user'), false);
  assert.equal(
    isBillingPriceApprover(ownerWithoutAdminRole, 'owner-workos-user'),
    false,
  );
});
