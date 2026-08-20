const { spawnSync } = require('node:child_process');

function runTests() {
  const args = process.argv.slice(2);
  const postgres = args[0] === '--postgres';
  if (postgres) args.shift();
  const environment = {
    ...process.env,
    NVM_ENV: 'development',
    NVM_DATABASE_ENV: 'development',
    ...(postgres ? { NVM_DB_DRIVER: 'postgres' } : {}),
  };
  const result = spawnSync(process.execPath, args, {
    env: environment,
    stdio: 'inherit',
  });
  if (result.error) throw result.error;
  process.exit(result.status ?? 1);
}

runTests();
