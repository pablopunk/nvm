CREATE TABLE "request_dedup" (
	"id" bigserial PRIMARY KEY NOT NULL,
	"user_id" uuid NOT NULL,
	"idempotency_key" text NOT NULL,
	"request_hash" text,
	"status" text DEFAULT 'in_flight' NOT NULL,
	"response_json" jsonb,
	"response_headers" jsonb,
	"upstream_status" integer,
	"request_id" text,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	"completed_at" timestamp with time zone
);
--> statement-breakpoint
ALTER TABLE "request_dedup" ADD CONSTRAINT "request_dedup_user_id_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."users"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
CREATE UNIQUE INDEX "request_dedup_user_key_idx" ON "request_dedup" USING btree ("user_id","idempotency_key");