async function responseErrorMessage(response: Response): Promise<string> {
  const body = await response.text();
  const type = response.headers.get('content-type') || '';
  if (type.includes('application/json')) {
    try {
      return JSON.parse(body).error?.message || response.statusText || `Request failed (${response.status})`;
    } catch { /* Fall through to a safe message. */ }
  }
  if (type.startsWith('text/plain')) return body.trim().slice(0, 300) || response.statusText;
  return response.statusText || `Request failed (${response.status})`;
}

/** An idempotent admin write whose error message is safe to show in the console. */
export async function adminMutation<T = unknown>(path: string, method: 'POST' | 'PUT', payload: unknown): Promise<T> {
  const response = await fetch(path, {
    method,
    headers: { 'Content-Type': 'application/json', 'Idempotency-Key': crypto.randomUUID() },
    body: JSON.stringify(payload),
  });
  if (!response.ok) throw new Error(await responseErrorMessage(response));
  return response.json() as Promise<T>;
}
