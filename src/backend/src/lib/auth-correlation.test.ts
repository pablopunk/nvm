import assert from 'node:assert/strict';
import test from 'node:test';
import {
  appendAuthCorrelationClear,
  authCorrelationCookie,
  authCorrelationMatches,
  clearAuthCorrelationCookie,
  readAuthCorrelationCookie,
} from './auth-correlation';

test('serializes a host-only callback correlation cookie', () => {
  const cookie = authCorrelationCookie('v2.state/value', true);
  assert.match(cookie, /^nvm_auth_state=v2\.state%2Fvalue;/);
  assert.match(cookie, /Path=\/api\/auth\/callback/);
  assert.match(cookie, /HttpOnly/);
  assert.match(cookie, /SameSite=Lax/);
  assert.match(cookie, /Max-Age=600/);
  assert.match(cookie, /Secure/);
  assert.doesNotMatch(cookie, /Domain=/i);
  assert.equal(readAuthCorrelationCookie(cookie), 'v2.state/value');
});

test('omits Secure on HTTP and rejects missing, malformed, or mismatched cookies', () => {
  const cookie = authCorrelationCookie('expected', false);
  assert.doesNotMatch(cookie, /Secure/);
  assert.equal(authCorrelationMatches(cookie, 'expected'), true);
  assert.equal(authCorrelationMatches(cookie, 'different'), false);
  assert.equal(authCorrelationMatches(null, 'expected'), false);
  assert.equal(readAuthCorrelationCookie('nvm_auth_state=%E0%A4%A'), null);
});

test('appends correlation cleanup without replacing existing cookies', () => {
  const headers = new Headers();
  headers.append('Set-Cookie', 'nvm_session=session; Path=/');
  const response = appendAuthCorrelationClear(
    new Response(null, { status: 302, headers }),
    true,
  );
  const setCookies = response.headers.getSetCookie();
  assert.equal(setCookies.length, 2);
  assert.equal(setCookies[0], 'nvm_session=session; Path=/');
  assert.equal(setCookies[1], clearAuthCorrelationCookie(true));
  assert.match(setCookies[1]!, /Max-Age=0/);
});
