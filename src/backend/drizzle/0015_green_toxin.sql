CREATE TABLE "billing_price_terms" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"price_id" text NOT NULL,
	"version" integer NOT NULL,
	"kind" text NOT NULL,
	"tier" text,
	"credits" integer NOT NULL,
	"unit_amount" integer NOT NULL,
	"currency" text NOT NULL,
	"quantity" integer DEFAULT 1 NOT NULL,
	"interval" text,
	"interval_count" integer,
	"source" text NOT NULL,
	"approved_at" timestamp with time zone NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL
);
--> statement-breakpoint
CREATE UNIQUE INDEX "billing_price_terms_price_version_idx" ON "billing_price_terms" USING btree ("price_id","version");
--> statement-breakpoint
CREATE FUNCTION reject_billing_price_terms_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
	RAISE EXCEPTION 'billing price terms are immutable';
END;
$$;
--> statement-breakpoint
CREATE TRIGGER billing_price_terms_immutable
BEFORE UPDATE OR DELETE ON "billing_price_terms"
FOR EACH ROW EXECUTE FUNCTION reject_billing_price_terms_mutation();
