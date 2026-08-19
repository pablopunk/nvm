import assert from 'node:assert/strict';
import test from 'node:test';
import { inviteUrl } from './email';

test('invite links use the canonical web origin when no public site URL is configured', () => {
  const previous = process.env.PUBLIC_SITE_URL;
  delete process.env.PUBLIC_SITE_URL;
  try {
    assert.equal(inviteUrl('invite-token'), 'https://www.nvm.fyi/invite#invite-token');
  } finally {
    if (previous === undefined) delete process.env.PUBLIC_SITE_URL;
    else process.env.PUBLIC_SITE_URL = previous;
  }
});
