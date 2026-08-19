import type {
  ExtensionContext,
  NevermindExtension,
} from '../resources/nevermind-extension-api';

const FILE_CANDIDATES = [
  {
    id: 'selection:roadmap',
    title: 'product-roadmap.md',
    subtitle: '~/Documents/product-roadmap.md',
  },
  {
    id: 'selection:brief',
    title: 'design-brief.pdf',
    subtitle: '~/Downloads/design-brief.pdf',
  },
  {
    id: 'selection:mockup',
    title: 'launch-mockup.png',
    subtitle: '~/Desktop/launch-mockup.png',
  },
] as const;

// biome-ignore lint/complexity/noExcessiveLinesPerFunction: selection state and its patch handlers share one view-local closure.
function fileSelectionListView(ctx: ExtensionContext) {
  const selectedIds = new Set<string>([FILE_CANDIDATES[0].id]);
  function fileItem(file: (typeof FILE_CANDIDATES)[number]) {
    const selected = selectedIds.has(file.id);
    const toggle = ctx.actions.run(
      `${selected ? 'Deselect' : 'Select'} ${file.title}`,
      () => {
        if (selectedIds.has(file.id)) {
          selectedIds.delete(file.id);
        } else {
          selectedIds.add(file.id);
        }
        return { patch: { items: [fileItem(file)] } };
      },
    );
    return ctx.ui.item({
      ...file,
      icon: 'file',
      accessories: [
        {
          text: selected ? 'Selected' : 'Optional',
          tone: selected ? 'success' : 'muted',
        },
      ],
      primaryAction: toggle,
      actions: [toggle],
    });
  }
  const review = ctx.actions.run('Review selected files', () => {
    const selected = FILE_CANDIDATES.filter((file) => selectedIds.has(file.id));
    if (selected.length === 0) {
      return ctx.ui.toast({
        message: 'Select at least one file',
        tone: 'error',
      });
    }
    return ctx.ui.preview({
      title: 'Selected Files',
      content: [
        '# Selected Files',
        '',
        ...selected.map((file) => `- ${file.subtitle}`),
      ].join('\n'),
    });
  });
  return ctx.ui.list({
    id: 'dev-ui-file-selection-list',
    title: 'Dev UI · File Selection List',
    subtitle:
      'Keyboard-first multi-select: Enter toggles a file, then review the selection.',
    searchBarPlaceholder: 'Filter file candidates',
    sections: [
      {
        title: 'Next step',
        items: [
          ctx.ui.item({
            id: 'review-file-selection',
            title: 'Review selected files',
            subtitle: 'Open the selected paths',
            icon: 'files',
            primaryAction: review,
            actions: [review],
          }),
        ],
      },
      { title: 'File candidates', items: FILE_CANDIDATES.map(fileItem) },
    ],
  });
}

const extension: NevermindExtension = {
  id: 'dev.file-selection-list',
  title: 'Dev UI · File Selection List',
  subtitle: 'Keyboard-first multi-select list fixture',
  commands: [
    {
      id: 'file-selection-list',
      title: 'Dev UI: File Selection List',
      subtitle: 'Patched list rows, selection state, and review flow',
      icon: 'files',
      run: fileSelectionListView,
    },
  ],
};

// biome-ignore lint/style/noDefaultExport: the extension loader resolves a module's default export.
export default extension;
