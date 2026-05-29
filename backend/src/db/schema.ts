import {
  pgTable,
  text,
  timestamp,
  integer,
  bigint,
  bigserial,
  uuid,
  index,
} from 'drizzle-orm/pg-core';

export const users = pgTable('users', {
  id: uuid('id').primaryKey().defaultRandom(),
  workosUserId: text('workos_user_id').notNull().unique(),
  email: text('email').notNull().unique(),
  plan: text('plan').notNull().default('free'),
  role: text('role').notNull().default('user'),
  stripeCustomerId: text('stripe_customer_id'),
  createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
});

export const appSettings = pgTable('app_settings', {
  key: text('key').primaryKey(),
  value: text('value').notNull(),
  updatedAt: timestamp('updated_at', { withTimezone: true }).notNull().defaultNow(),
});

export const creditLedger = pgTable(
  'credit_ledger',
  {
    id: bigserial('id', { mode: 'number' }).primaryKey(),
    userId: uuid('user_id').notNull().references(() => users.id, { onDelete: 'cascade' }),
    delta: integer('delta').notNull(),
    kind: text('kind').notNull().default('paid'),
    reason: text('reason').notNull(),
    refId: text('ref_id'),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
  },
  (t) => ({
    userIdx: index('credit_ledger_user_idx').on(t.userId),
  }),
);

export const usage = pgTable(
  'usage',
  {
    id: bigserial('id', { mode: 'number' }).primaryKey(),
    userId: uuid('user_id').notNull().references(() => users.id, { onDelete: 'cascade' }),
    model: text('model').notNull(),
    inputTokens: integer('input_tokens').notNull(),
    outputTokens: integer('output_tokens').notNull(),
    costCredits: integer('cost_credits').notNull(),
    upstreamCostMicrocents: bigint('upstream_cost_microcents', { mode: 'number' }).notNull().default(0),
    provider: text('provider'),
    requestId: text('request_id'),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
  },
  (t) => ({
    userIdx: index('usage_user_idx').on(t.userId),
  }),
);

export const apiTokens = pgTable(
  'api_tokens',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    userId: uuid('user_id').notNull().references(() => users.id, { onDelete: 'cascade' }),
    tokenHash: text('token_hash').notNull().unique(),
    prefix: text('prefix').notNull(),
    name: text('name').notNull(),
    lastUsedAt: timestamp('last_used_at', { withTimezone: true }),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
    revokedAt: timestamp('revoked_at', { withTimezone: true }),
  },
  (t) => ({
    userIdx: index('api_tokens_user_idx').on(t.userId),
  }),
);

export const deviceCodes = pgTable('device_codes', {
  code: text('code').primaryKey(),
  userId: uuid('user_id').references(() => users.id, { onDelete: 'cascade' }),
  deviceLabel: text('device_label').notNull(),
  approvedAt: timestamp('approved_at', { withTimezone: true }),
  expiresAt: timestamp('expires_at', { withTimezone: true }).notNull(),
  consumedAt: timestamp('consumed_at', { withTimezone: true }),
  createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
});

export const subscriptions = pgTable('subscriptions', {
  userId: uuid('user_id').primaryKey().references(() => users.id, { onDelete: 'cascade' }),
  stripeSubId: text('stripe_sub_id').notNull().unique(),
  tier: text('tier').notNull(),
  status: text('status').notNull(),
  currentPeriodEnd: timestamp('current_period_end', { withTimezone: true }).notNull(),
});
