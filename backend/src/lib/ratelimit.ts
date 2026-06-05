import { Ratelimit } from '@upstash/ratelimit';
import { Redis } from '@upstash/redis';
import { log } from './log';

const url = process.env.UPSTASH_REDIS_REST_URL ?? process.env.KV_REST_API_URL;
const token = process.env.UPSTASH_REDIS_REST_TOKEN ?? process.env.KV_REST_API_TOKEN;

const DASHBOARD_URL = 'https://nvm.fyi/dashboard';

const redis = url && token ? new Redis({ url, token }) : null;
if (!redis) {
  log.warn('ratelimit_disabled', { reason: 'UPSTASH_REDIS_REST_URL/TOKEN missing' });
}

function makeLimiter(prefix: string, limit: number, window: `${number} ${'s' | 'm' | 'h' | 'd'}`) {
  if (!redis) return null;
  return new Ratelimit({
    redis,
    prefix: `nvm:${prefix}`,
    limiter: Ratelimit.slidingWindow(limit, window),
    analytics: false,
  });
}

const chatPerMinFree = makeLimiter('chat:min:free', 5, '1 m');
const chatPerDayFree = makeLimiter('chat:day:free', 20, '1 d');
const chatPerMinPaid = makeLimiter('chat:min:paid', 60, '1 m');
const chatPerDayPaid = makeLimiter('chat:day:paid', 5000, '1 d');

export type RateLimitDecision = { ok: true } | { ok: false; retryAfterSec: number; scope: string };

type RateLimitOverrides = {
  chat?: typeof rateLimitChat;
  ip?: typeof rateLimitIp;
};

let testOverrides: RateLimitOverrides = {};

export function setRateLimitOverridesForTests(overrides: RateLimitOverrides) {
  testOverrides = overrides;
}

export function resetRateLimitOverridesForTests() {
  testOverrides = {};
}

async function checkPair(
  key: string,
  scope: string,
  perMin: Ratelimit | null,
  perDay: Ratelimit | null,
): Promise<RateLimitDecision> {
  if (!perMin || !perDay) return { ok: true };
  const minRes = await perMin.limit(key);
  if (!minRes.success) {
    return { ok: false, scope: `${scope}:minute`, retryAfterSec: Math.max(1, Math.ceil((minRes.reset - Date.now()) / 1000)) };
  }
  const dayRes = await perDay.limit(key);
  if (!dayRes.success) {
    return { ok: false, scope: `${scope}:day`, retryAfterSec: Math.max(1, Math.ceil((dayRes.reset - Date.now()) / 1000)) };
  }
  return { ok: true };
}

export async function rateLimitChat(userId: string, kind: 'free' | 'paid'): Promise<RateLimitDecision> {
  if (testOverrides.chat) return testOverrides.chat(userId, kind);
  const [perMin, perDay] = kind === 'free' ? [chatPerMinFree, chatPerDayFree] : [chatPerMinPaid, chatPerDayPaid];
  return checkPair(userId, `chat:${kind}`, perMin, perDay);
}

const ipLimiters = new Map<string, Ratelimit | null>();
function ipLimiter(scope: string, limit: number, window: `${number} ${'s' | 'm' | 'h' | 'd'}`) {
  const key = `${scope}:${limit}:${window}`;
  if (!ipLimiters.has(key)) ipLimiters.set(key, makeLimiter(`ip:${scope}`, limit, window));
  return ipLimiters.get(key)!;
}

export async function rateLimitIp(
  scope: 'auth' | 'tokens',
  ip: string | null,
  limit = 30,
  window: `${number} ${'s' | 'm' | 'h' | 'd'}` = '1 m',
): Promise<RateLimitDecision> {
  if (testOverrides.ip) return testOverrides.ip(scope, ip, limit, window);
  if (!ip) return { ok: true };
  const limiter = ipLimiter(scope, limit, window);
  if (!limiter) return { ok: true };
  const res = await limiter.limit(ip);
  if (res.success) return { ok: true };
  return { ok: false, scope: `ip:${scope}`, retryAfterSec: Math.max(1, Math.ceil((res.reset - Date.now()) / 1000)) };
}

export function clientIp(request: Request): string | null {
  const fwd = request.headers.get('x-forwarded-for');
  if (fwd) return fwd.split(',')[0]!.trim();
  return request.headers.get('x-real-ip');
}

export function tooManyRequests(decision: RateLimitDecision & { ok: false }): Response {
  return Response.json(
    { error: { type: 'rate_limited', message: `Rate limit exceeded (${decision.scope})`, retry_after: decision.retryAfterSec, dashboard_url: DASHBOARD_URL } },
    { status: 429, headers: { 'Retry-After': String(decision.retryAfterSec) } },
  );
}
