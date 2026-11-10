#!/usr/bin/env bash
# MyGo 0.7.4 staples during packaging. Retry only Apple's confirmed transient
# CloudKit connection reset; all other stapler errors fail on the first attempt.
set -uo pipefail

if [[ "${1-}" != stapler || "${2-}" != staple ]]; then
  exec /usr/bin/xcrun "$@"
fi

max_attempts=4
delays=(5 15 30)
attempt=1
output_file="$(mktemp)"
trap 'rm -f "$output_file"' EXIT

while :; do
  if /usr/bin/xcrun "$@" >"$output_file" 2>&1; then
    cat "$output_file"
    exit 0
  else
    status=$?
  fi

  cat "$output_file" >&2
  if (( attempt >= max_attempts )) || ! grep -Fq 'NSURLErrorDomain Code=-1005' "$output_file" || \
    ! grep -Fq 'api.apple-cloudkit.com/database/1/com.apple.gk.ticket-delivery/production/public/records/lookup' "$output_file"; then
    exit "$status"
  fi

  delay="${delays[$((attempt - 1))]}"
  echo "Apple notarization ticket lookup lost its network connection; retrying stapler in ${delay}s (attempt $((attempt + 1))/${max_attempts})." >&2
  sleep "$delay"
  ((attempt += 1))
done
