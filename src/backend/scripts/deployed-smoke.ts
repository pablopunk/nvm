import { runDeployedSmoke } from '../src/lib/deployed-smoke';

try {
  const result = await runDeployedSmoke(process.env.NVM_SMOKE_BASE_URL);
  console.log(JSON.stringify({ ok: true, ...result }, null, 2));
} catch (error) {
  console.error(
    JSON.stringify(
      {
        ok: false,
        error: error instanceof Error ? error.message : 'Unknown smoke failure',
      },
      null,
      2,
    ),
  );
  process.exitCode = 1;
}
