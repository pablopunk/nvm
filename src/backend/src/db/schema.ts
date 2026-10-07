import { pgTable, text, timestamp, integer, bigint, bigserial, uuid, index, uniqueIndex, jsonb, boolean } from 'drizzle-orm/pg-core';
import { sql } from 'drizzle-orm';

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
    userId: uuid('user_id')
      .notNull()
      .references(() => users.id, { onDelete: 'cascade' }),
    delta: integer('delta').notNull(),
    kind: text('kind').notNull().default('paid'),
    reason: text('reason').notNull(),
    refId: text('ref_id'),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
  },
  (t) => [index('credit_ledger_user_idx').on(t.userId), uniqueIndex('credit_ledger_user_reason_ref_idx').on(t.userId, t.reason, t.refId)],
);

export const usage = pgTable(
  'usage',
  {
    id: bigserial('id', { mode: 'number' }).primaryKey(),
    userId: uuid('user_id')
      .notNull()
      .references(() => users.id, { onDelete: 'cascade' }),
    model: text('model').notNull(),
    inputTokens: integer('input_tokens').notNull(),
    outputTokens: integer('output_tokens').notNull(),
    cachedInputTokens: integer('cached_input_tokens').notNull().default(0),
    cacheWriteInputTokens: integer('cache_write_input_tokens').notNull().default(0),
    reasoningTokens: integer('reasoning_tokens').notNull().default(0),
    modality: text('modality').notNull().default('text'),
    audioDurationMs: integer('audio_duration_ms').notNull().default(0),
    costCredits: integer('cost_credits').notNull(),
    upstreamCostMicrocents: bigint('upstream_cost_microcents', {
      mode: 'number',
    })
      .notNull()
      .default(0),
    upstreamCostSource: text('upstream_cost_source').notNull().default('catalog_estimate'),
    costOverrun: boolean('cost_overrun').notNull().default(false),
    routeSlot: text('route_slot'),
    provider: text('provider'),
    requestId: text('request_id'),
    status: integer('status'),
    latencyMs: integer('latency_ms'),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
  },
  (t) => [index('usage_user_idx').on(t.userId), index('usage_created_idx').on(t.createdAt)],
);

// Reservations are deliberately separate from the append-only ledger. A
// reservation reduces what may be admitted, while only a final settlement
// creates a spendable-ledger debit for the measured usage.
export const creditReservations = pgTable(
  'credit_reservations',
  {
    requestId: text('request_id').primaryKey(),
    userId: uuid('user_id')
      .notNull()
      .references(() => users.id, { onDelete: 'cascade' }),
    kind: text('kind').notNull(),
    reservedCredits: integer('reserved_credits').notNull(),
    status: text('status').notNull().default('pending'),
    actualCredits: integer('actual_credits'),
    reservedFreeCredits: integer('reserved_free_credits'),
    reservedPaidCredits: integer('reserved_paid_credits'),
    actualFreeCredits: integer('actual_free_credits'),
    actualPaidCredits: integer('actual_paid_credits'),
    expiresAt: timestamp('expires_at', { withTimezone: true }).notNull(),
    settledAt: timestamp('settled_at', { withTimezone: true }),
    releasedAt: timestamp('released_at', { withTimezone: true }),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
    updatedAt: timestamp('updated_at', { withTimezone: true }).notNull().defaultNow(),
  },
  (t) => [
    index('credit_reservations_active_user_idx').on(t.userId, t.status),
    index('credit_reservations_pending_expiry_idx').on(t.status, t.expiresAt),
  ],
);

export const routeBreakers = pgTable(
  'route_breakers',
  {
    routeSlot: text('route_slot').notNull(),
    provider: text('provider').notNull(),
    modelId: text('model_id').notNull(),
    trippedAt: timestamp('tripped_at', { withTimezone: true }).notNull().defaultNow(),
    reason: text('reason').notNull(),
  },
  (t) => [uniqueIndex('route_breakers_route_provider_model_idx').on(t.routeSlot, t.provider, t.modelId)],
);

export const apiTokens = pgTable(
  'api_tokens',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    userId: uuid('user_id')
      .notNull()
      .references(() => users.id, { onDelete: 'cascade' }),
    tokenHash: text('token_hash').notNull().unique(),
    prefix: text('prefix').notNull(),
    name: text('name').notNull(),
    lastUsedAt: timestamp('last_used_at', { withTimezone: true }),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
    revokedAt: timestamp('revoked_at', { withTimezone: true }),
  },
  (t) => [index('api_tokens_user_idx').on(t.userId)],
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
    actorUserId: uuid('actor_user_id').references(() => users.id, {
      onDelete: 'set null',
    }),
    action: text('action').notNull(),
    targetType: text('target_type'),
    targetId: text('target_id'),
    meta: jsonb('meta'),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
  },
  (t) => [index('audit_log_created_idx').on(t.createdAt)],
);

export const subscriptions = pgTable('subscriptions', {
  userId: uuid('user_id')
    .primaryKey()
    .references(() => users.id, { onDelete: 'cascade' }),
  stripeSubId: text('stripe_sub_id').notNull().unique(),
  tier: text('tier').notNull(),
  status: text('status').notNull(),
  currentPeriodEnd: timestamp('current_period_end', {
    withTimezone: true,
  }).notNull(),
  lastEventCreatedAt: timestamp('last_event_created_at', {
    withTimezone: true,
  }),
  lastEventId: text('last_event_id'),
});

// Approved Stripe purchase terms are immutable facts. New offers require a
// new row; checkout must never infer historical terms from environment config.
export const billingPriceTerms = pgTable(
  'billing_price_terms',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    priceId: text('price_id').notNull(),
    version: integer('version').notNull(),
    kind: text('kind').notNull(),
    tier: text('tier'),
    credits: integer('credits').notNull(),
    unitAmount: integer('unit_amount').notNull(),
    currency: text('currency').notNull(),
    quantity: integer('quantity').notNull().default(1),
    interval: text('interval'),
    intervalCount: integer('interval_count'),
    source: text('source').notNull(),
    approvedAt: timestamp('approved_at', { withTimezone: true }).notNull(),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
  },
  (t) => [uniqueIndex('billing_price_terms_price_version_idx').on(t.priceId, t.version)],
);

export const stripeCustomerAttempts = pgTable('stripe_customer_attempts', {
  userId: uuid('user_id')
    .primaryKey()
    .references(() => users.id, { onDelete: 'cascade' }),
  idempotencyKey: text('idempotency_key').notNull().unique(),
  payload: jsonb('payload').notNull(),
  customerId: text('customer_id'),
  status: text('status').notNull().default('creating'),
  leaseExpiresAt: timestamp('lease_expires_at', { withTimezone: true }),
  lastError: text('last_error'),
  createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
  updatedAt: timestamp('updated_at', { withTimezone: true }).notNull().defaultNow(),
});

export const billingOrders = pgTable(
  'billing_orders',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    userId: uuid('user_id')
      .notNull()
      .references(() => users.id, { onDelete: 'cascade' }),
    attemptKey: uuid('attempt_key').notNull(),
    intentHash: text('intent_hash').notNull(),
    kind: text('kind').notNull(),
    priceTermsId: uuid('price_terms_id')
      .notNull()
      .references(() => billingPriceTerms.id),
    priceId: text('price_id').notNull(),
    tier: text('tier'),
    credits: integer('credits').notNull(),
    expectedUnitAmount: integer('expected_unit_amount').notNull(),
    expectedTotalAmount: integer('expected_total_amount').notNull(),
    currency: text('currency').notNull(),
    quantity: integer('quantity').notNull(),
    interval: text('interval'),
    intervalCount: integer('interval_count'),
    stripeCustomerId: text('stripe_customer_id').notNull(),
    requestSnapshot: jsonb('request_snapshot').notNull(),
    state: text('state').notNull().default('creating'),
    sessionId: text('session_id').unique(),
    sessionUrl: text('session_url'),
    paymentMethodTypes: jsonb('payment_method_types'),
    paymentIntentId: text('payment_intent_id'),
    leaseGeneration: integer('lease_generation').notNull().default(1),
    leaseExpiresAt: timestamp('lease_expires_at', { withTimezone: true }),
    firstAttemptAt: timestamp('first_attempt_at', { withTimezone: true }).notNull().defaultNow(),
    version: integer('version').notNull().default(1),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
    updatedAt: timestamp('updated_at', { withTimezone: true }).notNull().defaultNow(),
  },
  (t) => [
    uniqueIndex('billing_orders_user_attempt_idx').on(t.userId, t.attemptKey),
    index('billing_orders_user_state_idx').on(t.userId, t.state),
    uniqueIndex('billing_orders_one_active_user_idx')
      .on(t.userId)
      .where(sql`${t.state} in ('creating', 'open', 'processing', 'retry_required', 'retry_cancel_pending', 'needs_reconciliation')`),
  ],
);

export const billingRetryOperations = pgTable(
  'billing_retry_operations',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    userId: uuid('user_id')
      .notNull()
      .references(() => users.id, { onDelete: 'cascade' }),
    priorOrderId: uuid('prior_order_id')
      .notNull()
      .references(() => billingOrders.id),
    replacementOrderId: uuid('replacement_order_id').references(() => billingOrders.id),
    retryKey: uuid('retry_key').notNull(),
    payloadHash: text('payload_hash').notNull(),
    state: text('state').notNull(),
    cancelGeneration: integer('cancel_generation').notNull().default(1),
    cancelIdempotencyKey: text('cancel_idempotency_key').notNull(),
    leaseExpiresAt: timestamp('lease_expires_at', { withTimezone: true }),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
    updatedAt: timestamp('updated_at', { withTimezone: true }).notNull().defaultNow(),
  },
  (t) => [
    uniqueIndex('billing_retry_user_key_idx').on(t.userId, t.retryKey),
    uniqueIndex('billing_retry_one_unresolved_idx')
      .on(t.priorOrderId)
      .where(sql`${t.state} in ('cancel_pending', 'replacement_ready', 'checkout_creating')`),
  ],
);

export const billingDuplicateChargeConflicts = pgTable(
  'billing_duplicate_charge_conflicts',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    originalOrderId: uuid('original_order_id')
      .notNull()
      .references(() => billingOrders.id),
    replacementOrderId: uuid('replacement_order_id').references(() => billingOrders.id),
    stripeObjectId: text('stripe_object_id').notNull(),
    objectKind: text('object_kind').notNull(),
    stripeEventId: text('stripe_event_id').notNull(),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
  },
  (t) => [uniqueIndex('billing_duplicate_charge_object_idx').on(t.stripeObjectId)],
);

export const billingSubscriptionPriceVersions = pgTable(
  'billing_subscription_price_versions',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    subscriptionId: text('subscription_id').notNull(),
    userId: uuid('user_id')
      .notNull()
      .references(() => users.id, { onDelete: 'cascade' }),
    priceTermsId: uuid('price_terms_id')
      .notNull()
      .references(() => billingPriceTerms.id),
    priceId: text('price_id').notNull(),
    tier: text('tier').notNull(),
    credits: integer('credits').notNull(),
    interval: text('interval').notNull(),
    intervalCount: integer('interval_count').notNull(),
    unitAmount: integer('unit_amount').notNull(),
    currency: text('currency').notNull(),
    quantity: integer('quantity').notNull(),
    effectiveFrom: timestamp('effective_from', {
      withTimezone: true,
    }).notNull(),
    effectiveTo: timestamp('effective_to', { withTimezone: true }),
    sourceOrderId: uuid('source_order_id').references(() => billingOrders.id),
    sourceEventId: text('source_event_id'),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
  },
  (t) => [
    uniqueIndex('billing_sub_price_version_unique').on(t.subscriptionId, t.effectiveFrom),
    index('billing_sub_price_version_effective_idx').on(t.subscriptionId, t.effectiveFrom, t.effectiveTo),
  ],
);

export const stripeEvents = pgTable(
  'stripe_events',
  {
    eventId: text('event_id').primaryKey(),
    type: text('type').notNull(),
    apiVersion: text('api_version'),
    payload: jsonb('payload'),
    state: text('state').notNull().default('processing'),
    reconciliationReason: text('reconciliation_reason'),
    processedAt: timestamp('processed_at', { withTimezone: true }).notNull().defaultNow(),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
  },
  (t) => [index('stripe_events_type_idx').on(t.type)],
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
    userId: uuid('user_id')
      .notNull()
      .references(() => users.id, { onDelete: 'cascade' }),
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
  (t) => [uniqueIndex('request_dedup_user_key_idx').on(t.userId, t.idempotencyKey)],
);

export const modelProviders = pgTable(
  'model_providers',
  {
    id: bigserial('id', { mode: 'number' }).primaryKey(),
    routeSlot: text('route_slot').notNull(),
    modelId: text('model_id').notNull(),
    providerId: text('provider_id')
      .notNull()
      .references(() => providers.id, { onDelete: 'cascade' }),
    providerModelId: text('provider_model_id'),
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
  reviewerId: uuid('reviewer_id').references(() => users.id, {
    onDelete: 'set null',
  }),
  reviewNote: text('review_note'),
  ipHash: text('ip_hash'),
});

export const invites = pgTable(
  'invites',
  {
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
  },
  (t) => [uniqueIndex('invites_one_active_recipient').on(t.email).where(sql`${t.status} in ('queued', 'sending', 'sent')`)],
);

export const emailOutbox = pgTable('email_outbox', {
  id: uuid('id').primaryKey().defaultRandom(),
  inviteId: uuid('invite_id')
    .notNull()
    .references(() => invites.id, { onDelete: 'cascade' }),
  recipient: text('recipient').notNull(),
  templateVersion: text('template_version').notNull().default('invite-v1'),
  idempotencyKey: text('idempotency_key').notNull().unique(),
  tokenCiphertext: text('token_ciphertext').notNull(),
  status: text('status').notNull().default('queued'),
  attempts: integer('attempts').notNull().default(0),
  providerMessageId: text('provider_message_id'),
  lastError: text('last_error'),
  availableAt: timestamp('available_at', { withTimezone: true }).notNull().defaultNow(),
  leaseOwner: text('lease_owner'),
  leaseExpiresAt: timestamp('lease_expires_at', { withTimezone: true }),
  createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
});

export const providerEvents = pgTable(
  'provider_events',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    provider: text('provider').notNull(),
    eventId: text('event_id').notNull(),
    eventType: text('event_type').notNull(),
    payloadHash: text('payload_hash').notNull(),
    processedAt: timestamp('processed_at', { withTimezone: true }),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
  },
  (t) => [uniqueIndex('provider_events_provider_event_idx').on(t.provider, t.eventId)],
);

export const emailSuppressions = pgTable('email_suppressions', {
  email: text('email').primaryKey(),
  reason: text('reason').notNull(),
  createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
});

export const authIntents = pgTable('auth_intents', {
  id: uuid('id').primaryKey().defaultRandom(),
  inviteId: uuid('invite_id')
    .notNull()
    .references(() => invites.id, { onDelete: 'cascade' }),
  nonceHash: text('nonce_hash').notNull().unique(),
  expiresAt: timestamp('expires_at', { withTimezone: true }).notNull(),
  consumedAt: timestamp('consumed_at', { withTimezone: true }),
  createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
});
