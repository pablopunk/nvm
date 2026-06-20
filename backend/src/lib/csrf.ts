/**
 * Returns null (pass) for same-origin requests or 403 for cross-origin.
 *
 * When the Origin header is absent, returns null — same-origin navigations
 * (including form POSTs) often omit Origin, and these are already protected
 * by SameSite cookie attributes. Rejecting missing-Origin requests would
 * break legitimate same-origin flows like direct form submissions.
 */
export function requireSameOrigin(request: Request): Response | null {
  const origin = request.headers.get('origin');
  if (!origin) return null;
  let originNormalized: string;
  try { originNormalized = new URL(origin).origin; } catch { return new Response('Forbidden', { status: 403 }); }
  if (originNormalized === new URL(request.url).origin) return null;
  return new Response('Forbidden', { status: 403 });
}
