import type { APIRoute } from 'astro';

export const prerender = true;

export const GET: APIRoute = function robots({ site }) {
  return new Response(
    `User-agent: *\nAllow: /\nDisallow: /api/\n\nSitemap: ${new URL('/sitemap.xml', site).href}\n`,
    { headers: { 'Content-Type': 'text/plain; charset=utf-8' } },
  );
};
