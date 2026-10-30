// biome-ignore-all lint: Dev-only UI fixtures intentionally exercise dynamic extension payload shapes.
import { showExtensionFeedback } from '../../extensions/feedback.ts';
import type { ExtensionContext } from '../../resources/nevermind-extension-api';

export function crudCollectionView(ctx: ExtensionContext) {
  const preview = ctx.actions.run('Preview draft', () =>
    ctx.ui.previewView({
      id: 'dev-ui-crud-preview',
      title: 'Release notes draft',
      content:
        '# Release notes draft\n\nA generic collection record can open any host-rendered preview.',
    }),
  );
  const edit = ctx.input.prompt({
    title: 'Edit draft',
    fields: [
      {
        id: 'title',
        label: 'Title',
        type: 'text',
        value: 'Release notes draft',
      },
    ],
    action: ctx.actions.run('Save draft', (innerCtx, action) =>
      showExtensionFeedback(
        innerCtx,
        'CRUD Collection',
        `Saved ${String(action.formValues?.title || 'draft')}`,
        'success',
      ),
    ),
  });
  const remove = ctx.actions.run('Remove draft', (innerCtx) =>
    showExtensionFeedback(
      innerCtx,
      'CRUD Collection',
      'Draft removed',
      'success',
    ),
  );
  return ctx.ui.collection({
    id: 'dev-ui-crud-collection',
    title: 'Dev UI · CRUD Collection',
    subtitle:
      'Host-owned add, preview, edit, and remove with default shortcuts and confirmation',
    searchBarPlaceholder: 'Find a record',
    emptyView: {
      title: 'No records',
      subtitle: 'Use Add record to exercise the empty state.',
    },
    add: ctx.actions.run('Add record', (innerCtx) =>
      showExtensionFeedback(
        innerCtx,
        'CRUD Collection',
        'Add action invoked',
        'success',
      ),
    ),
    items: [
      {
        id: 'release-notes',
        title: 'Release notes draft',
        subtitle: 'A reusable CRUD record fixture',
        icon: 'file-pen-line',
        preview,
        edit,
        remove,
      },
      {
        id: 'roadmap',
        title: 'Roadmap draft',
        subtitle: 'Custom remove confirmation and no preview',
        icon: 'map',
        edit,
        remove: {
          ...remove,
          title: 'Discard roadmap',
          confirmMessage: 'Discard the roadmap draft permanently?',
          confirmLabel: 'Discard',
        },
      },
    ],
  });
}
