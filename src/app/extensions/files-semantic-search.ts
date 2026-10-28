import type {
  ExtensionContext,
  ExtensionIndexedFile,
  ExtensionItem,
} from '../resources/nevermind-extension-api';

const MAX_CANDIDATES = 16;

export function semanticFileSearch(
  ctx: ExtensionContext,
  fileItem: (file: ExtensionIndexedFile) => ExtensionItem,
) {
  return ctx.input.prompt({
    title: 'Find Files by Meaning',
    message: 'Search recent file names using AI.',
    fields: [
      {
        id: 'query',
        label: 'What are you looking for?',
        type: 'text',
        required: true,
      },
    ],
    submitTitle: 'Find Files',
    action: ctx.actions.run('Find Files', (innerCtx, action) => {
      const query = String(action.formValues?.query || '').trim();
      const files = innerCtx.desktop.files.recent({ limit: MAX_CANDIDATES });
      const local = innerCtx.desktop.files.searchIndex(query, {
        limit: MAX_CANDIDATES,
      });
      const runAi = innerCtx.actions.run(
        'Find matching files with AI',
        async (searchCtx) => {
          if (!searchCtx.ai) throw new Error('AI is unavailable');
          const result = await searchCtx.ai.decide({
            input: {
              query,
              files: files.map((file, index) => ({
                id: `file_${index}`,
                name: file.name,
              })),
            },
            questions: files.map((file, index) => ({
              id: `file_${index}`,
              type: 'predicate' as const,
              question: `Is file_${index} relevant to the search query?`,
            })),
            signal: AbortSignal.timeout(30_000),
          });
          const matches = files.filter((_, index) => {
            const answer = result.answers[`file_${index}`];
            return answer?.type === 'predicate' && answer.probability >= 0.5;
          });
          const merged = [
            ...local,
            ...matches.filter(
              (file) => !local.some((item) => item.path === file.path),
            ),
          ];
          return {
            patch: { mode: 'replace' as const, items: merged.map(fileItem) },
          };
        },
      );
      return innerCtx.ui.list({
        id: 'find-files-by-meaning',
        title: 'Find Files by Meaning',
        subtitle: `Search: ${query}`,
        searchBarPlaceholder: 'Filter files',
        items: [
          ...(files.length
            ? [
                {
                  id: 'find-with-ai',
                  title: 'Find more files with AI',
                  subtitle: `Send ${files.length} recent file names for analysis`,
                  icon: 'sparkles',
                  primaryAction: runAi,
                },
              ]
            : []),
          ...local.map(fileItem),
        ],
        emptyView: { title: 'No files found', subtitle: 'Try another search.' },
      });
    }),
  });
}
