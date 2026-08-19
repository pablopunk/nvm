CREATE TABLE "model_providers" (
	"id" bigserial PRIMARY KEY NOT NULL,
	"route_slot" text NOT NULL,
	"model_id" text NOT NULL,
	"provider_id" text NOT NULL,
	"priority" integer DEFAULT 0 NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL
);
--> statement-breakpoint
CREATE TABLE "providers" (
	"id" text PRIMARY KEY NOT NULL,
	"display_name" text NOT NULL,
	"enabled" text DEFAULT 'true' NOT NULL,
	"priority" integer DEFAULT 0 NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL
);
--> statement-breakpoint
ALTER TABLE "model_providers" ADD CONSTRAINT "model_providers_provider_id_providers_id_fk" FOREIGN KEY ("provider_id") REFERENCES "public"."providers"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
CREATE UNIQUE INDEX "model_providers_route_model_provider_unique" ON "model_providers" USING btree ("route_slot","model_id","provider_id");--> statement-breakpoint
CREATE INDEX "model_providers_route_model_idx" ON "model_providers" USING btree ("route_slot","model_id");
--> statement-breakpoint
INSERT INTO "providers" ("id", "display_name", "priority") VALUES
  ('opencode_zen', 'Opencode Zen', 0),
  ('openrouter', 'OpenRouter', 1),
  ('anthropic', 'Anthropic', 2),
  ('openai', 'OpenAI', 3),
  ('google', 'Google AI', 4)
ON CONFLICT ("id") DO NOTHING;