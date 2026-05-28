import 'dotenv/config';
import { Pool, neonConfig } from '@neondatabase/serverless';
import { drizzle } from 'drizzle-orm/neon-serverless';
import { sql } from 'drizzle-orm';
import ws from 'ws';
import * as schema from '../src/db/schema';

neonConfig.webSocketConstructor = ws;
const pool = new Pool({ connectionString: process.env.DATABASE_URL });
const db = drizzle(pool, { schema });

type Row = { provider: string; modelId: string; inUsd: number; outUsd: number };

const ROWS: Row[] = [
  // Opencode Zen — approximated to public retail $/Mtok for now.
  { provider: 'opencode_zen', modelId: 'gemini-3-flash', inUsd: 0.3, outUsd: 2.5 },
  { provider: 'opencode_zen', modelId: 'gemini-3.5-flash', inUsd: 0.3, outUsd: 2.5 },
  { provider: 'opencode_zen', modelId: 'gemini-3.1-pro', inUsd: 1.25, outUsd: 10.0 },
  { provider: 'opencode_zen', modelId: 'claude-haiku-4-5', inUsd: 0.8, outUsd: 4.0 },
  { provider: 'opencode_zen', modelId: 'claude-sonnet-4-6', inUsd: 3.0, outUsd: 15.0 },
  { provider: 'opencode_zen', modelId: 'deepseek-v4-flash-free', inUsd: 0, outUsd: 0 },
];

for (const r of ROWS) {
  await db.execute(sql`
    insert into model_costs (provider, model_id, input_usd_per_mtok, output_usd_per_mtok)
    values (${r.provider}, ${r.modelId}, ${r.inUsd}, ${r.outUsd})
    on conflict (provider, model_id) do update
      set input_usd_per_mtok = excluded.input_usd_per_mtok,
          output_usd_per_mtok = excluded.output_usd_per_mtok,
          updated_at = now()
  `);
  console.log('seeded', r.provider, r.modelId);
}

await pool.end();
