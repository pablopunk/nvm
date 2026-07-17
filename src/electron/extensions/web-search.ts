import { getUrlFromQuery } from '../search-utils';
import { extensionContext } from './_context';

export function createWebSearchExtension() {
  return {
    id: 'nevermind.web',
    title: 'Web',
    capabilities: [] as const,
    commands: [],
    searchItems(_ctx, query) {
      const q = String(query || '').trim();
      if (!q) return [];
      const url = getUrlFromQuery(q);
      if (url) {
        return [
          {
            id: `open-url:${url}`,
            title: `Open ${url.replace(/^https?:\/\//, '')}`,
            subtitle: 'Open website',
            icon: 'globe',
            score: 100,
            dismissAfterRun: 'auto',
            primaryAction: {
              type: 'openUrl',
              title: 'Open Website',
              url,
              dismissAfterRun: 'auto',
            },
          },
        ];
      }
      return [
        {
          id: `web-search:${q}`,
          title: `Search the web for "${q}"`,
          subtitle: 'Search instead',
          icon: 'search',
          score:
            10 +
            extensionContext.usageBoost(`web-search:${q}`) +
            extensionContext.recentBoost(`web-search:${q}`),
          dismissAfterRun: 'auto',
          primaryAction: {
            type: 'openUrl',
            title: 'Search the Web',
            url: `https://www.google.com/search?q=${encodeURIComponent(q)}`,
            dismissAfterRun: 'auto',
          },
        },
      ];
    },
  };
}
