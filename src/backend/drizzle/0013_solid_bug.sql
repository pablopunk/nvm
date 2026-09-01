ALTER TABLE "model_providers" ADD COLUMN "provider_model_id" text;--> statement-breakpoint
UPDATE "model_providers" SET "provider_model_id" = "model_id";--> statement-breakpoint
ALTER TABLE "usage" ADD COLUMN "cached_input_tokens" integer DEFAULT 0 NOT NULL;--> statement-breakpoint
ALTER TABLE "usage" ADD COLUMN "cache_write_input_tokens" integer DEFAULT 0 NOT NULL;--> statement-breakpoint
ALTER TABLE "usage" ADD COLUMN "reasoning_tokens" integer DEFAULT 0 NOT NULL;--> statement-breakpoint
ALTER TABLE "usage" ADD COLUMN "upstream_cost_source" text DEFAULT 'catalog_estimate' NOT NULL;
