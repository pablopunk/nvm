export function limitedOutput(value, limit = 200_000) {
  const text = String(value || '');
  return text.length > limit
    ? `${text.slice(0, limit)}\n... output truncated ...`
    : text;
}
