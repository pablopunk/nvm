CREATE TABLE "pricing_snapshots" (
	"key" text PRIMARY KEY NOT NULL,
	"validated_at" bigint NOT NULL,
	"payload" jsonb NOT NULL
);
