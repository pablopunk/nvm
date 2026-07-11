ALTER TABLE "subscriptions" ADD COLUMN "last_event_created_at" timestamp with time zone;--> statement-breakpoint
ALTER TABLE "subscriptions" ADD COLUMN "last_event_id" text;