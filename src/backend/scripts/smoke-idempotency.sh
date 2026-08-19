#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${1:-http://localhost:4321}"
PAT="${2:-}"
if [ -z "$PAT" ]; then
  echo "Usage: $0 <base_url> <pat_token>"
  echo "  base_url   - API base URL (default: http://localhost:4321)"
  echo "  pat_token  - Nevermind PAT (nvm_pat_...)"
  exit 1
fi

IDEMPOTENCY_KEY="smoke-$(date +%s)-$$"
MODEL="${3:-gpt-4o-mini}"

echo "=== Idempotency Smoke ==="
echo "Base URL:    $BASE_URL"
echo "Key:         $IDEMPOTENCY_KEY"
echo "Model:       $MODEL"

call1_tmp=$(mktemp)
call2_tmp=$(mktemp)

echo ""
echo "--- Call 1 ---"
curl -s -i -w "\n%{http_code}" \
  -X POST "$BASE_URL/api/v1/chat/completions" \
  -H "Authorization: Bearer $PAT" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: $IDEMPOTENCY_KEY" \
  -d "{\"model\":\"$MODEL\",\"messages\":[{\"role\":\"user\",\"content\":\"Say hello in one word.\"}]}" \
  > "$call1_tmp"

call1_status=$(tail -1 "$call1_tmp")
call1_request_id=$(grep -i '^x-request-id:' "$call1_tmp" | awk '{print $2}' | tr -d '\r')

echo "Status:      $call1_status"
echo "Request-ID:  $call1_request_id"

echo ""
echo "--- Call 2 (same Idempotency-Key) ---"
curl -s -i -w "\n%{http_code}" \
  -X POST "$BASE_URL/api/v1/chat/completions" \
  -H "Authorization: Bearer $PAT" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: $IDEMPOTENCY_KEY" \
  -d "{\"model\":\"$MODEL\",\"messages\":[{\"role\":\"user\",\"content\":\"Say hello in one word.\"}]}" \
  > "$call2_tmp"

call2_status=$(tail -1 "$call2_tmp")
call2_request_id=$(grep -i '^x-request-id:' "$call2_tmp" | awk '{print $2}' | tr -d '\r')

echo "Status:      $call2_status"
echo "Request-ID:  $call2_request_id"

echo ""
echo "--- Assertions ---"
passed=0
failed=0

if [ "$call1_request_id" = "$call2_request_id" ]; then
  echo "PASS: Both calls have the same x-request-id: $call1_request_id"
  passed=$((passed + 1))
else
  echo "FAIL: Request IDs differ: call1=$call1_request_id, call2=$call2_request_id"
  failed=$((failed + 1))
fi

if [ "$call1_status" = "200" ]; then
  echo "PASS: Call 1 returned 200"
  passed=$((passed + 1))
else
  echo "FAIL: Call 1 returned $call1_status"
  failed=$((failed + 1))
fi

if [ "$call2_status" = "200" ]; then
  echo "PASS: Call 2 returned 200 (replay)"
  passed=$((passed + 1))
else
  echo "FAIL: Call 2 returned $call2_status"
  failed=$((failed + 1))
fi

echo ""
if [ "$failed" -eq 0 ]; then
  echo "=== ALL CHECKS PASSED ($passed/$((passed + failed))) ==="
else
  echo "=== $failed CHECKS FAILED (${passed}/$((passed + failed))) ==="
fi

rm -f "$call1_tmp" "$call2_tmp"

if [ "$failed" -gt 0 ]; then
  exit 1
fi
