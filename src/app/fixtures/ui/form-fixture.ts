// biome-ignore-all lint: Dev-only UI fixtures intentionally exercise dynamic extension payload shapes.
import type {
  ExtensionContext,
  ExtensionFormValue,
} from '../../resources/nevermind-extension-api';

function valuesMarkdown(values: Record<string, ExtensionFormValue> = {}) {
  const rows = Object.entries(values).map(
    ([key, value]) =>
      `- **${key}**: ${Array.isArray(value) ? value.join(', ') : String(value)}`,
  );
  return rows.length ? rows.join('\n') : 'No values submitted.';
}

export function formView(ctx: ExtensionContext) {
  return ctx.ui.form({
    id: 'dev-ui-form',
    title: 'Dev UI · Form Fields',
    subtitle: 'Search fields, edit with Enter, then save',
    fields: [
      {
        id: 'intro',
        type: 'description',
        description:
          'Choose any field with the arrow keys. Enter edits it; Escape returns to the list. Save when ready.',
      },
      {
        id: 'name',
        label: 'Text',
        type: 'text',
        value: 'Nevermind',
        placeholder: 'Name',
        required: true,
        description: 'Plain single-line text.',
      },
      {
        id: 'notes',
        label: 'Textarea',
        type: 'textarea',
        value: 'Longer editable text',
        rows: 5,
        description: 'Multi-line input for notes, templates, and prompts.',
      },
      {
        id: 'password',
        label: 'Password',
        type: 'password',
        placeholder: 'Secret',
      },
      { id: 'url', label: 'URL', type: 'url', value: 'https://nvm.fyi' },
      { id: 'number', label: 'Number', type: 'number', value: '42' },
      { id: 'date', label: 'Date', type: 'date' },
      {
        id: 'enabled',
        label: 'Checkbox',
        type: 'checkbox',
        value: true,
        description: 'Boolean form value.',
      },
      {
        id: 'disabled',
        label: 'Disabled checkbox',
        type: 'checkbox',
        value: false,
        disabled: true,
        description: 'Visible but unavailable until its prerequisite is met.',
      },
      { id: 'separator', type: 'separator' },
      {
        id: 'color',
        label: 'Dropdown',
        type: 'dropdown',
        value: 'blue',
        options: [
          { title: 'Blue', value: 'blue' },
          { title: 'Green', value: 'green' },
          { title: 'Purple', value: 'purple' },
        ],
      },
      {
        id: 'tags',
        label: 'Multiselect',
        type: 'multiselect',
        value: ['templates', 'forms'],
        options: [
          { title: 'Templates', value: 'templates' },
          { title: 'Forms', value: 'forms' },
          { title: 'Files', value: 'files' },
        ],
      },
      {
        id: 'inputFile',
        label: 'File picker',
        type: 'file',
        placeholder: 'Choose one source file',
        description: 'Single path value from the host native file picker.',
      },
      {
        id: 'inputFiles',
        label: 'Files picker',
        type: 'files',
        placeholder: 'Choose one or more images',
        extensions: ['png', 'jpg', 'jpeg', 'webp'],
        filterName: 'Images',
        description: 'Array value from a multi-select native file picker.',
      },
      {
        id: 'outputFolder',
        label: 'Folder picker',
        type: 'folder',
        placeholder: 'Choose an output folder',
        description: 'Folder path value from the host native folder picker.',
      },
      {
        id: 'invalid',
        label: 'Error State',
        type: 'text',
        value: 'Bad value',
        error: 'Example field-level error copy.',
      },
    ],
    submitAction: ctx.actions.run('Show Submitted Values', (_ctx, action) =>
      ctx.ui.preview({
        title: 'Submitted Form Values',
        content: `# Submitted Form Values\n\n${valuesMarkdown(action.formValues)}`,
      }),
    ),
  });
}
