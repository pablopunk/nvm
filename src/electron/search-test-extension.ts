const CONTROLLED_PROVIDER_DELAY_MS = 500;

export function createProgressiveSearchTestExtension() {
  return {
    id: 'pab85.search-progressive',
    title: 'PAB-85 Progressive Search',
    commands: [
      {
        id: 'immediate',
        title: 'PAB-85 Immediate Search',
        subtitle: 'Registered result available before provider settlement',
        aliases: [
          'PAB-85 Immediate Search Alpha',
          'PAB-85 Immediate Search Stale',
          'PAB-85 Immediate Search Current',
        ],
        run: () => undefined,
      },
    ],
    searchItems(ctx: { signal?: AbortSignal }, query: string) {
      if (!query.startsWith('PAB-85 Immediate Search')) {
        return [];
      }
      return new Promise<unknown[]>((resolve) => {
        const timer = setTimeout(
          () =>
            resolve([
              {
                id: `delayed:${query}`,
                title: `PAB-85 Delayed Provider: ${query}`,
                subtitle: 'Controlled 500 ms provider contribution',
                icon: 'clock',
              },
            ]),
          CONTROLLED_PROVIDER_DELAY_MS,
        );
        timer.unref?.();
        ctx.signal?.addEventListener(
          'abort',
          () => {
            clearTimeout(timer);
            resolve([]);
          },
          { once: true },
        );
      });
    },
  };
}
