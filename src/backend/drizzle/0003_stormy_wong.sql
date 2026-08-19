ALTER TABLE "usage" ADD COLUMN "status" integer;--> statement-breakpoint
ALTER TABLE "usage" ADD COLUMN "latency_ms" integer;--> statement-breakpoint
CREATE INDEX "usage_created_idx" ON "usage" USING btree ("created_at");