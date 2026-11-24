CREATE TABLE "model_capability_probes" (
	"provider" text NOT NULL,
	"model_id" text NOT NULL,
	"capability" text NOT NULL,
	"passed" boolean NOT NULL,
	"detail" text,
	"probed_at" bigint NOT NULL
);
--> statement-breakpoint
CREATE UNIQUE INDEX "model_capability_probes_model_capability_unique" ON "model_capability_probes" USING btree ("provider","model_id","capability");