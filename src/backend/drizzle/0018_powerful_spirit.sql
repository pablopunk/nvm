CREATE TABLE "billing_duplicate_charge_conflicts" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"original_order_id" uuid NOT NULL,
	"replacement_order_id" uuid,
	"stripe_object_id" text NOT NULL,
	"object_kind" text NOT NULL,
	"stripe_event_id" text NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL
);
--> statement-breakpoint
ALTER TABLE "billing_duplicate_charge_conflicts" ADD CONSTRAINT "billing_duplicate_charge_conflicts_original_order_id_billing_orders_id_fk" FOREIGN KEY ("original_order_id") REFERENCES "public"."billing_orders"("id") ON DELETE no action ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "billing_duplicate_charge_conflicts" ADD CONSTRAINT "billing_duplicate_charge_conflicts_replacement_order_id_billing_orders_id_fk" FOREIGN KEY ("replacement_order_id") REFERENCES "public"."billing_orders"("id") ON DELETE no action ON UPDATE no action;--> statement-breakpoint
CREATE UNIQUE INDEX "billing_duplicate_charge_object_idx" ON "billing_duplicate_charge_conflicts" USING btree ("stripe_object_id");