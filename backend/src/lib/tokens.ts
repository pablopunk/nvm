import { and, eq, isNull } from 'drizzle-orm';
import { randomBytes, createHash } from 'node:crypto';
import { db, type Database } from '../db/client';
import { apiTokens, users } from '../db/schema';

const TOKEN_PREFIX = 'nvm_pat_';

function hashToken(token: string) {
  return createHash('sha256').update(token).digest('hex');
}

export function generateToken() {
  const secret = randomBytes(32).toString('base64url');
  const token = `${TOKEN_PREFIX}${secret}`;
  return { token, prefix: token.slice(0, TOKEN_PREFIX.length + 6), hash: hashToken(token) };
}

type TokenWriter = Pick<Database, 'insert'>;

export async function createApiToken(
  userId: string,
  name: string,
  writer: TokenWriter = db,
) {
  const { token, prefix, hash } = generateToken();
  const [row] = await writer
    .insert(apiTokens)
    .values({ userId, tokenHash: hash, prefix, name })
    .returning({ id: apiTokens.id, prefix: apiTokens.prefix, name: apiTokens.name, createdAt: apiTokens.createdAt });
  return { ...row, token };
}

export async function listApiTokens(userId: string) {
  return db
    .select({
      id: apiTokens.id,
      prefix: apiTokens.prefix,
      name: apiTokens.name,
      createdAt: apiTokens.createdAt,
      lastUsedAt: apiTokens.lastUsedAt,
    })
    .from(apiTokens)
    .where(and(eq(apiTokens.userId, userId), isNull(apiTokens.revokedAt)));
}

export async function revokeApiToken(userId: string, tokenId: string) {
  await db
    .update(apiTokens)
    .set({ revokedAt: new Date() })
    .where(and(eq(apiTokens.id, tokenId), eq(apiTokens.userId, userId)));
}

async function resolveToken(token: string | null) {
  if (!token || !token.startsWith(TOKEN_PREFIX)) return null;
  const hash = hashToken(token);
  const [row] = await db
    .select({ user: users, tokenId: apiTokens.id })
    .from(apiTokens)
    .innerJoin(users, eq(users.id, apiTokens.userId))
    .where(and(eq(apiTokens.tokenHash, hash), isNull(apiTokens.revokedAt)))
    .limit(1);
  return row ?? null;
}

function extractBearer(authHeader: string | null): string | null {
  if (!authHeader?.startsWith('Bearer ')) return null;
  return authHeader.slice('Bearer '.length).trim();
}

export type PatHeaderName = 'authorization' | 'x-api-key' | 'x-goog-api-key';

export function extractPatFromHeaders(request: Request, headerName: PatHeaderName): string | null {
  const raw = request.headers.get(headerName);
  if (!raw) return null;
  if (headerName === 'authorization') return extractBearer(raw);
  return raw.trim();
}

export async function getUserFromHeaders(request: Request, headerName: PatHeaderName) {
  const token = extractPatFromHeaders(request, headerName);
  const row = await resolveToken(token);
  if (!row) return null;
  db.update(apiTokens).set({ lastUsedAt: new Date() }).where(eq(apiTokens.id, row.tokenId)).catch(() => {});
  return row.user;
}

export async function getUserFromBearer(authHeader: string | null) {
  const row = await resolveToken(extractBearer(authHeader));
  if (!row) return null;
  db.update(apiTokens).set({ lastUsedAt: new Date() }).where(eq(apiTokens.id, row.tokenId)).catch(() => {});
  return row.user;
}

export async function getTokenAndUserFromBearer(authHeader: string | null) {
  return resolveToken(extractBearer(authHeader));
}
