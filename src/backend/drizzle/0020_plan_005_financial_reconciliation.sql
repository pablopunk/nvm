CREATE TABLE "financial_reconciliation_jobs" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"requested_by" uuid NOT NULL,
	"range_from" timestamp with time zone NOT NULL,
	"range_to" timestamp with time zone NOT NULL,
	"cursor" text,
	"state" text DEFAULT 'pending' NOT NULL,
	"lease_version" integer DEFAULT 0 NOT NULL,
	"lease_expires_at" timestamp with time zone,
	"imported_count" integer DEFAULT 0 NOT NULL,
	"updated_count" integer DEFAULT 0 NOT NULL,
	"unknown_count" integer DEFAULT 0 NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL,
	"completed_at" timestamp with time zone
);
--> statement-breakpoint
CREATE TABLE "financial_reconciliation_pages" (
	"id" bigserial PRIMARY KEY NOT NULL,
	"job_id" uuid NOT NULL,
	"lease_version" integer NOT NULL,
	"cursor_before" text,
	"cursor_after" text,
	"has_more" boolean NOT NULL,
	"imported_count" integer NOT NULL,
	"updated_count" integer NOT NULL,
	"unknown_count" integer NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL
);
--> statement-breakpoint
CREATE TABLE "stripe_balance_transactions" (
	"id" text PRIMARY KEY NOT NULL,
	"source_object_id" text,
	"order_id" uuid,
	"user_id" uuid,
	"currency" text,
	"amount" bigint,
	"fee" bigint,
	"net" bigint,
	"type" text,
	"reporting_category" text,
	"classified_as" text DEFAULT 'unclassified' NOT NULL,
	"classifier_version" integer DEFAULT 1 NOT NULL,
	"created_at_stripe" timestamp with time zone,
	"available_on" timestamp with time zone,
	"status" text,
	"fee_known" boolean DEFAULT false NOT NULL,
	"missing_fields" jsonb DEFAULT '[]'::jsonb NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL
);
--> statement-breakpoint
ALTER TABLE "financial_reconciliation_jobs" ADD CONSTRAINT "financial_reconciliation_jobs_requested_by_users_id_fk" FOREIGN KEY ("requested_by") REFERENCES "public"."users"("id") ON DELETE restrict ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "financial_reconciliation_pages" ADD CONSTRAINT "financial_reconciliation_pages_job_id_financial_reconciliation_jobs_id_fk" FOREIGN KEY ("job_id") REFERENCES "public"."financial_reconciliation_jobs"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "stripe_balance_transactions" ADD CONSTRAINT "stripe_balance_transactions_order_id_billing_orders_id_fk" FOREIGN KEY ("order_id") REFERENCES "public"."billing_orders"("id") ON DELETE set null ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "stripe_balance_transactions" ADD CONSTRAINT "stripe_balance_transactions_user_id_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."users"("id") ON DELETE set null ON UPDATE no action;--> statement-breakpoint
CREATE INDEX "financial_reconciliation_jobs_state_updated_idx" ON "financial_reconciliation_jobs" USING btree ("state","updated_at");--> statement-breakpoint
CREATE UNIQUE INDEX "financial_reconciliation_jobs_one_active_range_idx" ON "financial_reconciliation_jobs" USING btree ("range_from","range_to") WHERE "financial_reconciliation_jobs"."state" in ('pending', 'processing');--> statement-breakpoint
CREATE UNIQUE INDEX "financial_reconciliation_pages_job_lease_idx" ON "financial_reconciliation_pages" USING btree ("job_id","lease_version");--> statement-breakpoint
CREATE INDEX "financial_reconciliation_pages_job_created_idx" ON "financial_reconciliation_pages" USING btree ("job_id","created_at");--> statement-breakpoint
CREATE INDEX "stripe_balance_transactions_created_idx" ON "stripe_balance_transactions" USING btree ("created_at_stripe");--> statement-breakpoint
CREATE INDEX "stripe_balance_transactions_category_currency_idx" ON "stripe_balance_transactions" USING btree ("reporting_category","currency");--> statement-breakpoint
CREATE INDEX "stripe_balance_transactions_order_idx" ON "stripe_balance_transactions" USING btree ("order_id");