CREATE TABLE "credit_reservations" (
	"request_id" text PRIMARY KEY NOT NULL,
	"user_id" uuid NOT NULL,
	"kind" text NOT NULL,
	"reserved_credits" integer NOT NULL,
	"status" text DEFAULT 'pending' NOT NULL,
	"actual_credits" integer,
	"expires_at" timestamp with time zone NOT NULL,
	"settled_at" timestamp with time zone,
	"released_at" timestamp with time zone,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL
);
--> statement-breakpoint
ALTER TABLE "credit_reservations" ADD CONSTRAINT "credit_reservations_user_id_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."users"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
CREATE INDEX "credit_reservations_active_user_idx" ON "credit_reservations" USING btree ("user_id","status");--> statement-breakpoint
CREATE INDEX "credit_reservations_pending_expiry_idx" ON "credit_reservations" USING btree ("status","expires_at");