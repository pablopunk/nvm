UPDATE "request_dedup"
SET "request_hash" = 'legacy:' || "id"::text
WHERE "request_hash" IS NULL;
--> statement-breakpoint
ALTER TABLE "request_dedup" ALTER COLUMN "request_hash" SET NOT NULL;
