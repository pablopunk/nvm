export function requireSameOrigin(request: Request): Response | null {
  const origin = request.headers.get('origin');
  if (!origin) return null;
  let originNormalized: string;
  try { originNormalized = new URL(origin).origin; } catch { return new Response('Forbidden', { status: 403 }); }
  if (originNormalized === new URL(request.url).origin) return null;
  return new Response('Forbidden', { status: 403 });
}
