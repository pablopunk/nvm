DELETE FROM "credit_ledger" newer
USING "credit_ledger" older
WHERE newer."id" > older."id"
  AND newer."user_id" = older."user_id"
  AND newer."reason" = older."reason"
  AND newer."ref_id" = older."ref_id"
  AND newer."ref_id" IS NOT NULL;
--> statement-breakpoint
CREATE UNIQUE INDEX "credit_ledger_user_reason_ref_idx" ON "credit_ledger" USING btree ("user_id","reason","ref_id");