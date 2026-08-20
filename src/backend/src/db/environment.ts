const DATABASE_ENVIRONMENTS = ['development', 'preview', 'production'] as const;

export type DatabaseEnvironment = (typeof DATABASE_ENVIRONMENTS)[number];

type Environment = Record<string, string | undefined>;

function readDatabaseEnvironment(
  environment: Environment,
  name: 'NVM_ENV' | 'NVM_DATABASE_ENV',
): DatabaseEnvironment {
  const value = environment[name];
  if (!DATABASE_ENVIRONMENTS.includes(value as DatabaseEnvironment)) {
    throw new Error(
      `${name} must be one of: ${DATABASE_ENVIRONMENTS.join(', ')}`,
    );
  }
  return value as DatabaseEnvironment;
}

export function validateDatabaseEnvironment(environment: Environment) {
  const applicationEnvironment = readDatabaseEnvironment(environment, 'NVM_ENV');
  const databaseEnvironment = readDatabaseEnvironment(
    environment,
    'NVM_DATABASE_ENV',
  );

  if (applicationEnvironment !== databaseEnvironment) {
    throw new Error('NVM_ENV must match NVM_DATABASE_ENV');
  }

  const vercelEnvironment = environment.VERCEL_ENV;
  if (
    vercelEnvironment &&
    DATABASE_ENVIRONMENTS.includes(vercelEnvironment as DatabaseEnvironment) &&
    applicationEnvironment !== vercelEnvironment
  ) {
    throw new Error('NVM_ENV must match VERCEL_ENV');
  }

  return applicationEnvironment;
}

export function validateMigrationEnvironment(environment: Environment) {
  const target = validateDatabaseEnvironment(environment);
  if (
    target === 'production' &&
    environment.NVM_PRODUCTION_MIGRATION_APPROVED !== 'true'
  ) {
    throw new Error(
      'NVM_PRODUCTION_MIGRATION_APPROVED=true is required for production migrations',
    );
  }
  return target;
}
