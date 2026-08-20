import assert from 'node:assert/strict';
import test from 'node:test';
import {
  validateDatabaseEnvironment,
  validateMigrationEnvironment,
} from './environment';

for (const environment of ['development', 'preview', 'production']) {
  test(`accepts the ${environment} database environment`, function () {
    assert.equal(
      validateDatabaseEnvironment({
        NVM_ENV: environment,
        NVM_DATABASE_ENV: environment,
      }),
      environment,
    );
  });
}

test('rejects missing environment identities', function () {
  assert.throws(() => validateDatabaseEnvironment({}), /NVM_ENV/);
});

test('rejects unknown environment identities', function () {
  assert.throws(
    () =>
      validateDatabaseEnvironment({
        NVM_ENV: 'staging',
        NVM_DATABASE_ENV: 'staging',
      }),
    /NVM_ENV/,
  );
});

test('rejects an application and database environment mismatch', function () {
  assert.throws(
    () =>
      validateDatabaseEnvironment({
        NVM_ENV: 'development',
        NVM_DATABASE_ENV: 'production',
      }),
    /NVM_ENV must match NVM_DATABASE_ENV/,
  );
});

test('rejects a deployed environment mismatch', function () {
  assert.throws(
    () =>
      validateDatabaseEnvironment({
        NVM_ENV: 'production',
        NVM_DATABASE_ENV: 'production',
        VERCEL_ENV: 'preview',
      }),
    /NVM_ENV must match VERCEL_ENV/,
  );
});

test('requires explicit approval for a production migration', function () {
  const environment = {
    NVM_ENV: 'production',
    NVM_DATABASE_ENV: 'production',
  };
  assert.throws(
    () => validateMigrationEnvironment(environment),
    /NVM_PRODUCTION_MIGRATION_APPROVED=true/,
  );
  assert.equal(
    validateMigrationEnvironment({
      ...environment,
      NVM_PRODUCTION_MIGRATION_APPROVED: 'true',
    }),
    'production',
  );
});
