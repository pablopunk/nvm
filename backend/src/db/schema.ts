import {
  pgTable,
  text,
  timestamp,
  integer,
  bigint,
  bigserial,
  uuid,
  index,
  uniqueIndex,
  jsonb,
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
  (t) => [
    index('credit_ledger_user_idx').on(t.userId),
    uniqueIndex('credit_ledger_user_reason_ref_idx').on(t.userId, t.reason, t.refId),
  ],
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
    status: integer('status'),
    latencyMs: integer('latency_ms'),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
  },
  (t) => [
    index('usage_user_idx').on(t.userId),
    index('usage_created_idx').on(t.createdAt),
  ],
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
  (t) => [
    index('api_tokens_user_idx').on(t.userId),
  ],
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

export const auditLog = pgTable(
  'audit_log',
  {
    id: bigserial('id', { mode: 'number' }).primaryKey(),
    actorUserId: uuid('actor_user_id').references(() => users.id, { onDelete: 'set null' }),
    action: text('action').notNull(),
    targetType: text('target_type'),
    targetId: text('target_id'),
    meta: jsonb('meta'),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
  },
  (t) => [
    index('audit_log_created_idx').on(t.createdAt),
  ],
);

export const subscriptions = pgTable('subscriptions', {
  userId: uuid('user_id').primaryKey().references(() => users.id, { onDelete: 'cascade' }),
  stripeSubId: text('stripe_sub_id').notNull().unique(),
  tier: text('tier').notNull(),
  status: text('status').notNull(),
  currentPeriodEnd: timestamp('current_period_end', { withTimezone: true }).notNull(),
  lastEventCreatedAt: timestamp('last_event_created_at', { withTimezone: true }),
  lastEventId: text('last_event_id'),
});

export const stripeEvents = pgTable(
  'stripe_events',
  {
    eventId: text('event_id').primaryKey(),
    type: text('type').notNull(),
    apiVersion: text('api_version'),
    payload: jsonb('payload'),
    processedAt: timestamp('processed_at', { withTimezone: true }).notNull().defaultNow(),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
  },
  (t) => [
    index('stripe_events_type_idx').on(t.type),
  ],
);

export const providers = pgTable('providers', {
  id: text('id').primaryKey(),
  displayName: text('display_name').notNull(),
  enabled: text('enabled').notNull().default('true'),
  priority: integer('priority').notNull().default(0),
  createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
  updatedAt: timestamp('updated_at', { withTimezone: true }).notNull().defaultNow(),
});

export const requestDedup = pgTable(
  'request_dedup',
  {
    id: bigserial('id', { mode: 'number' }).primaryKey(),
    userId: uuid('user_id').notNull().references(() => users.id, { onDelete: 'cascade' }),
    idempotencyKey: text('idempotency_key').notNull(),
    requestHash: text('request_hash'),
    status: text('status').notNull().default('in_flight'),
    responseJson: jsonb('response_json'),
    responseHeaders: jsonb('response_headers'),
    upstreamStatus: integer('upstream_status'),
    requestId: text('request_id'),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
    completedAt: timestamp('completed_at', { withTimezone: true }),
  },
  (t) => [
    uniqueIndex('request_dedup_user_key_idx').on(t.userId, t.idempotencyKey),
  ],
);

export const modelProviders = pgTable(
  'model_providers',
  {
    id: bigserial('id', { mode: 'number' }).primaryKey(),
    routeSlot: text('route_slot').notNull(),
    modelId: text('model_id').notNull(),
    providerId: text('provider_id').notNull().references(() => providers.id, { onDelete: 'cascade' }),
    priority: integer('priority').notNull().default(0),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
  },
  (t) => [
    uniqueIndex('model_providers_route_model_provider_unique').on(t.routeSlot, t.modelId, t.providerId),
    index('model_providers_route_model_idx').on(t.routeSlot, t.modelId),
  ],
);

export const waitlistEntries = pgTable('waitlist_entries', {
  id: uuid('id').primaryKey().defaultRandom(),
  email: text('email').notNull().unique(),
  status: text('status').notNull().default('pending'),
  source: text('source').notNull().default('marketing'),
  submittedAt: timestamp('submitted_at', { withTimezone: true }).notNull().defaultNow(),
  reviewedAt: timestamp('reviewed_at', { withTimezone: true }),
  reviewerId: uuid('reviewer_id').references(() => users.id, { onDelete: 'set null' }),
  reviewNote: text('review_note'),
  ipHash: text('ip_hash'),
});

export const invites = pgTable('invites', {
  id: uuid('id').primaryKey().defaultRandom(),
  waitlistEntryId: uuid('waitlist_entry_id').references(() => waitlistEntries.id, { onDelete: 'set null' }),
  email: text('email').notNull(),
  tokenHash: text('token_hash').notNull().unique(),
  status: text('status').notNull().default('queued'),
  expiresAt: timestamp('expires_at', { withTimezone: true }).notNull(),
  createdBy: uuid('created_by').references(() => users.id, { onDelete: 'set null' }),
  createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
  sentAt: timestamp('sent_at', { withTimezone: true }),
  redeemedAt: timestamp('redeemed_at', { withTimezone: true }),
});

export const emailOutbox = pgTable('email_outbox', {
  id: uuid('id').primaryKey().defaultRandom(),
  inviteId: uuid('invite_id').notNull().references(() => invites.id, { onDelete: 'cascade' }),
  recipient: text('recipient').notNull(),
  templateVersion: text('template_version').notNull().default('invite-v1'),
  idempotencyKey: text('idempotency_key').notNull().unique(),
  status: text('status').notNull().default('queued'),
  attempts: integer('attempts').notNull().default(0),
  providerMessageId: text('provider_message_id'),
  lastError: text('last_error'),
  availableAt: timestamp('available_at', { withTimezone: true }).notNull().defaultNow(),
  createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
});

export const providerEvents = pgTable('provider_events', {
  id: uuid('id').primaryKey().defaultRandom(),
  provider: text('provider').notNull(),
  eventId: text('event_id').notNull(),
  eventType: text('event_type').notNull(),
  payloadHash: text('payload_hash').notNull(),
  processedAt: timestamp('processed_at', { withTimezone: true }),
  createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
}, (t) => [uniqueIndex('provider_events_provider_event_idx').on(t.provider, t.eventId)]);
