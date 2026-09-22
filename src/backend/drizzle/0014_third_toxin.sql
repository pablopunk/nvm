ALTER TABLE "usage" ADD COLUMN "modality" text DEFAULT 'text' NOT NULL;--> statement-breakpoint
ALTER TABLE "usage" ADD COLUMN "audio_duration_ms" integer DEFAULT 0 NOT NULL;