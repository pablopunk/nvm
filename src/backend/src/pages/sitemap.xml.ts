import type { APIRoute } from 'astro';
import { publicPages } from '../lib/seo';

export const prerender = true;

export const GET: APIRoute = function sitemap({ site }) {
  const urls = Object.keys(publicPages).map(function entry(path) {
    return `<url><loc>${new URL(path, site).href}</loc></url>`;
  });
  return new Response(
    `<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">${urls.join('')}</urlset>`,
    { headers: { 'Content-Type': 'application/xml; charset=utf-8' } },
  );
};
