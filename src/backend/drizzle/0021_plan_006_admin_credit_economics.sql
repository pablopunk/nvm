CREATE TABLE "admin_mutation_results" (
	"id" bigserial PRIMARY KEY NOT NULL,
	"actor_user_id" uuid NOT NULL,
	"idempotency_key" uuid NOT NULL,
	"payload_hash" text NOT NULL,
	"response_status" integer,
	"response_json" jsonb,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL
);
--> statement-breakpoint
CREATE TABLE "free_spend_cap_config" (
	"id" text PRIMARY KEY NOT NULL,
	"cap_microcents" bigint,
	"updated_by" uuid,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL
);
--> statement-breakpoint
INSERT INTO "free_spend_cap_config" ("id", "cap_microcents")
VALUES ('global', NULL)
ON CONFLICT ("id") DO NOTHING;
--> statement-breakpoint
CREATE TABLE "free_spend_monthly_budgets" (
	"month_key" text PRIMARY KEY NOT NULL,
	"cap_microcents" bigint,
	"actual_free_cost_microcents" bigint DEFAULT 0 NOT NULL,
	"reserved_free_cost_microcents" bigint DEFAULT 0 NOT NULL,
	"halted" boolean DEFAULT false NOT NULL,
	"baseline_complete" boolean DEFAULT false NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL
);
--> statement-breakpoint
ALTER TABLE "credit_reservations" ADD COLUMN "admission_month" text;--> statement-breakpoint
ALTER TABLE "credit_reservations" ADD COLUMN "estimated_provider_cost_microcents" bigint;--> statement-breakpoint
ALTER TABLE "credit_reservations" ADD COLUMN "estimated_total_credits" integer;--> statement-breakpoint
ALTER TABLE "credit_reservations" ADD COLUMN "estimate_cost_source" text;--> statement-breakpoint
ALTER TABLE "credit_reservations" ADD COLUMN "reserved_free_cost_microcents" bigint;--> statement-breakpoint
ALTER TABLE "credit_reservations" ADD COLUMN "actual_free_cost_microcents" bigint;--> statement-breakpoint
ALTER TABLE "usage" ADD COLUMN "audio_token_counts_available" boolean DEFAULT false NOT NULL;--> statement-breakpoint
ALTER TABLE "admin_mutation_results" ADD CONSTRAINT "admin_mutation_results_actor_user_id_users_id_fk" FOREIGN KEY ("actor_user_id") REFERENCES "public"."users"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "free_spend_cap_config" ADD CONSTRAINT "free_spend_cap_config_updated_by_users_id_fk" FOREIGN KEY ("updated_by") REFERENCES "public"."users"("id") ON DELETE set null ON UPDATE no action;--> statement-breakpoint
CREATE UNIQUE INDEX "admin_mutation_results_actor_key_idx" ON "admin_mutation_results" USING btree ("actor_user_id","idempotency_key");--> statement-breakpoint
CREATE INDEX "free_spend_monthly_budgets_updated_idx" ON "free_spend_monthly_budgets" USING btree ("updated_at");
