import { AsyncLocalStorage } from 'node:async_hooks';
import { env } from '../lib/env';
import { assertPreviewDatabaseBinding } from '../lib/auth-config';
import { createNeonDb } from './neon';
import { createPostgresDb } from './postgres';

export type Database = ReturnType<typeof createNeonDb>['db'];

const testDbStorage = new AsyncLocalStorage<Database | undefined>();
const driver = process.env.NVM_DB_DRIVER || 'neon';
if (process.env.VERCEL_ENV === 'preview') assertPreviewDatabaseBinding();
const connectionString = env('DATABASE_URL');
type DatabaseConnection = { db: Database; pool: { end: () => Promise<void> } };
const defaultConnection: DatabaseConnection =
  driver === 'postgres'
    ? (createPostgresDb(connectionString) as unknown as DatabaseConnection)
    : createNeonDb(connectionString);

export const db = new Proxy(defaultConnection.db, {
  get(_target, property, receiver) {
    return Reflect.get(getDb(), property, receiver);
  },
}) as Database;

export function getDb(): Database {
  return testDbStorage.getStore() || defaultConnection.db;
}

export function withTestDb<T>(
  database: Database,
  callback: () => T | Promise<T>,
) {
  return testDbStorage.run(database, callback);
}

// Compatibility for the existing fake-backed Node/tsx contract tests. This
// enters the current async context instead of mutating a process-global db.
export function setDbForTests(database: Database) {
  testDbStorage.enterWith(database);
}

export function resetDbForTests() {
  testDbStorage.enterWith(undefined);
}

export async function closeDefaultDb() {
  await defaultConnection.pool.end();
}
