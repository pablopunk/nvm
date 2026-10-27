SELECT id FROM free_spend_cap_config WHERE id = 'global' FOR UPDATE;
--> statement-breakpoint
WITH usage_totals AS (
  SELECT request_id, user_id, count(*) AS records,
    max(cost_credits) AS credits,
    max(upstream_cost_microcents) AS cost,
    bool_and(upstream_cost_source IN ('provider_reported', 'catalog_estimate', 'not_billed')) AS trusted
  FROM usage
  WHERE request_id IS NOT NULL
  GROUP BY request_id, user_id
), recovered AS (
  SELECT r.request_id,
    coalesce(r.admission_month, to_char(r.created_at AT TIME ZONE 'UTC', 'YYYY-MM')) AS month_key,
    d.free_credits, d.paid_credits,
    ceil(u.cost::numeric * d.free_credits / d.total_credits)::bigint AS free_cost
  FROM credit_reservations r
  JOIN usage_totals u ON u.request_id = r.request_id AND u.user_id = r.user_id
  CROSS JOIN LATERAL (
    SELECT coalesce(sum(-l.delta::bigint) FILTER (WHERE l.kind = 'free'), 0) AS free_credits,
      coalesce(sum(-l.delta::bigint) FILTER (WHERE l.kind = 'paid'), 0) AS paid_credits,
      sum(-l.delta::bigint) AS total_credits
    FROM credit_ledger l
    WHERE l.user_id = r.user_id AND l.reason = 'ai_usage' AND l.delta < 0
      AND l.ref_id IN (r.request_id, r.request_id || ':free', r.request_id || ':paid')
  ) d
  WHERE r.status = 'settled' AND u.records = 1 AND u.trusted AND u.cost >= 0
    AND d.total_credits > 0 AND d.total_credits = r.actual_credits
    AND d.total_credits = u.credits AND d.free_credits + d.paid_credits = d.total_credits
    AND (r.admission_month IS NULL OR r.actual_free_credits IS NULL
      OR r.actual_paid_credits IS NULL OR r.actual_free_cost_microcents IS NULL)
    AND (r.actual_free_credits IS NULL OR r.actual_free_credits = d.free_credits)
    AND (r.actual_paid_credits IS NULL OR r.actual_paid_credits = d.paid_credits)
    AND (r.actual_free_cost_microcents IS NULL OR r.actual_free_cost_microcents = ceil(u.cost::numeric * d.free_credits / d.total_credits))
)
UPDATE credit_reservations r SET
  admission_month = recovered.month_key,
  actual_free_credits = recovered.free_credits::integer,
  actual_paid_credits = recovered.paid_credits::integer,
  actual_free_cost_microcents = recovered.free_cost,
  reserved_free_cost_microcents = 0,
  updated_at = now()
FROM recovered WHERE r.request_id = recovered.request_id;
--> statement-breakpoint
WITH baselines AS (
  SELECT b.month_key,
    coalesce((
      SELECT sum(r.actual_free_cost_microcents)
      FROM credit_reservations r
      WHERE r.admission_month = b.month_key AND r.status = 'settled'
        AND r.actual_free_credits IS NOT NULL AND r.actual_paid_credits IS NOT NULL
        AND r.actual_free_cost_microcents IS NOT NULL
        AND EXISTS (SELECT 1 FROM usage u WHERE u.request_id = r.request_id AND u.user_id = r.user_id
          AND u.upstream_cost_source IN ('provider_reported', 'catalog_estimate', 'not_billed'))
    ), 0)::bigint AS actual,
    coalesce((SELECT sum(r.reserved_free_cost_microcents) FROM credit_reservations r
      WHERE r.admission_month = b.month_key AND r.status = 'pending'), 0)::bigint AS reserved,
    NOT EXISTS (
      SELECT 1 FROM usage u LEFT JOIN credit_reservations r ON r.request_id = u.request_id AND r.user_id = u.user_id
      WHERE u.created_at >= (b.month_key || '-01T00:00:00Z')::timestamptz
        AND u.created_at < ((b.month_key || '-01')::date + interval '1 month') AT TIME ZONE 'UTC'
        AND (u.cost_credits > 0 OR u.upstream_cost_microcents > 0)
        AND (r.request_id IS NULL OR r.admission_month IS NULL OR (r.admission_month = b.month_key AND (
          r.status <> 'settled' OR r.actual_free_credits IS NULL OR r.actual_paid_credits IS NULL
          OR r.actual_free_cost_microcents IS NULL
          OR u.upstream_cost_source NOT IN ('provider_reported', 'catalog_estimate', 'not_billed')
        )))
    ) AS complete
  FROM free_spend_monthly_budgets b
)
UPDATE free_spend_monthly_budgets b SET
  actual_free_cost_microcents = baselines.actual,
  reserved_free_cost_microcents = baselines.reserved,
  baseline_complete = baselines.complete,
  halted = b.cap_microcents IS NOT NULL AND (NOT baselines.complete
    OR baselines.actual >= b.cap_microcents OR baselines.actual + baselines.reserved > b.cap_microcents),
  updated_at = now()
FROM baselines WHERE b.month_key = baselines.month_key;
