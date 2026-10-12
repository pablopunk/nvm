CREATE TABLE "route_breakers" (
	"route_slot" text NOT NULL,
	"provider" text NOT NULL,
	"model_id" text NOT NULL,
	"tripped_at" timestamp with time zone DEFAULT now() NOT NULL,
	"reason" text NOT NULL
);
--> statement-breakpoint
ALTER TABLE "credit_reservations" ADD COLUMN "reserved_free_credits" integer;--> statement-breakpoint
ALTER TABLE "credit_reservations" ADD COLUMN "reserved_paid_credits" integer;--> statement-breakpoint
ALTER TABLE "credit_reservations" ADD COLUMN "actual_free_credits" integer;--> statement-breakpoint
ALTER TABLE "credit_reservations" ADD COLUMN "actual_paid_credits" integer;--> statement-breakpoint
ALTER TABLE "usage" ADD COLUMN "cost_overrun" boolean DEFAULT false NOT NULL;--> statement-breakpoint
ALTER TABLE "usage" ADD COLUMN "route_slot" text;--> statement-breakpoint
CREATE UNIQUE INDEX "route_breakers_route_provider_model_idx" ON "route_breakers" USING btree ("route_slot","provider","model_id");