CREATE TABLE "billing_orders" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"user_id" uuid NOT NULL,
	"attempt_key" uuid NOT NULL,
	"intent_hash" text NOT NULL,
	"kind" text NOT NULL,
	"price_terms_id" uuid NOT NULL,
	"price_id" text NOT NULL,
	"tier" text,
	"credits" integer NOT NULL,
	"expected_unit_amount" integer NOT NULL,
	"expected_total_amount" integer NOT NULL,
	"currency" text NOT NULL,
	"quantity" integer NOT NULL,
	"interval" text,
	"interval_count" integer,
	"stripe_customer_id" text NOT NULL,
	"request_snapshot" jsonb NOT NULL,
	"state" text DEFAULT 'creating' NOT NULL,
	"session_id" text,
	"session_url" text,
	"payment_method_types" jsonb,
	"payment_intent_id" text,
	"lease_generation" integer DEFAULT 1 NOT NULL,
	"lease_expires_at" timestamp with time zone,
	"first_attempt_at" timestamp with time zone DEFAULT now() NOT NULL,
	"version" integer DEFAULT 1 NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "billing_orders_session_id_unique" UNIQUE("session_id")
);
--> statement-breakpoint
CREATE TABLE "billing_retry_operations" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"user_id" uuid NOT NULL,
	"prior_order_id" uuid NOT NULL,
	"replacement_order_id" uuid,
	"retry_key" uuid NOT NULL,
	"payload_hash" text NOT NULL,
	"state" text NOT NULL,
	"cancel_generation" integer DEFAULT 1 NOT NULL,
	"cancel_idempotency_key" text NOT NULL,
	"lease_expires_at" timestamp with time zone,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL
);
--> statement-breakpoint
CREATE TABLE "billing_subscription_price_versions" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"subscription_id" text NOT NULL,
	"user_id" uuid NOT NULL,
	"price_terms_id" uuid NOT NULL,
	"price_id" text NOT NULL,
	"tier" text NOT NULL,
	"credits" integer NOT NULL,
	"interval" text NOT NULL,
	"interval_count" integer NOT NULL,
	"unit_amount" integer NOT NULL,
	"currency" text NOT NULL,
	"quantity" integer NOT NULL,
	"effective_from" timestamp with time zone NOT NULL,
	"effective_to" timestamp with time zone,
	"source_order_id" uuid,
	"source_event_id" text,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL
);
--> statement-breakpoint
CREATE TABLE "stripe_customer_attempts" (
	"user_id" uuid PRIMARY KEY NOT NULL,
	"idempotency_key" text NOT NULL,
	"payload" jsonb NOT NULL,
	"status" text DEFAULT 'creating' NOT NULL,
	"lease_expires_at" timestamp with time zone,
	"last_error" text,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "stripe_customer_attempts_idempotency_key_unique" UNIQUE("idempotency_key")
);
--> statement-breakpoint
ALTER TABLE "stripe_events" ADD COLUMN "state" text DEFAULT 'processing' NOT NULL;--> statement-breakpoint
ALTER TABLE "stripe_events" ADD COLUMN "reconciliation_reason" text;--> statement-breakpoint
ALTER TABLE "billing_orders" ADD CONSTRAINT "billing_orders_user_id_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."users"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "billing_orders" ADD CONSTRAINT "billing_orders_price_terms_id_billing_price_terms_id_fk" FOREIGN KEY ("price_terms_id") REFERENCES "public"."billing_price_terms"("id") ON DELETE no action ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "billing_retry_operations" ADD CONSTRAINT "billing_retry_operations_user_id_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."users"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "billing_retry_operations" ADD CONSTRAINT "billing_retry_operations_prior_order_id_billing_orders_id_fk" FOREIGN KEY ("prior_order_id") REFERENCES "public"."billing_orders"("id") ON DELETE no action ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "billing_retry_operations" ADD CONSTRAINT "billing_retry_operations_replacement_order_id_billing_orders_id_fk" FOREIGN KEY ("replacement_order_id") REFERENCES "public"."billing_orders"("id") ON DELETE no action ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "billing_subscription_price_versions" ADD CONSTRAINT "billing_subscription_price_versions_user_id_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."users"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "billing_subscription_price_versions" ADD CONSTRAINT "billing_subscription_price_versions_price_terms_id_billing_price_terms_id_fk" FOREIGN KEY ("price_terms_id") REFERENCES "public"."billing_price_terms"("id") ON DELETE no action ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "billing_subscription_price_versions" ADD CONSTRAINT "billing_subscription_price_versions_source_order_id_billing_orders_id_fk" FOREIGN KEY ("source_order_id") REFERENCES "public"."billing_orders"("id") ON DELETE no action ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "stripe_customer_attempts" ADD CONSTRAINT "stripe_customer_attempts_user_id_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."users"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
CREATE UNIQUE INDEX "billing_orders_user_attempt_idx" ON "billing_orders" USING btree ("user_id","attempt_key");--> statement-breakpoint
CREATE INDEX "billing_orders_user_state_idx" ON "billing_orders" USING btree ("user_id","state");--> statement-breakpoint
CREATE UNIQUE INDEX "billing_orders_one_active_user_idx" ON "billing_orders" USING btree ("user_id") WHERE "billing_orders"."state" in ('creating', 'open', 'processing', 'retry_required', 'retry_cancel_pending', 'needs_reconciliation');--> statement-breakpoint
CREATE UNIQUE INDEX "billing_retry_user_key_idx" ON "billing_retry_operations" USING btree ("user_id","retry_key");--> statement-breakpoint
CREATE UNIQUE INDEX "billing_retry_one_unresolved_idx" ON "billing_retry_operations" USING btree ("prior_order_id") WHERE "billing_retry_operations"."state" in ('cancel_pending', 'replacement_ready', 'checkout_creating');--> statement-breakpoint
CREATE UNIQUE INDEX "billing_sub_price_version_unique" ON "billing_subscription_price_versions" USING btree ("subscription_id","effective_from");--> statement-breakpoint
CREATE INDEX "billing_sub_price_version_effective_idx" ON "billing_subscription_price_versions" USING btree ("subscription_id","effective_from","effective_to");