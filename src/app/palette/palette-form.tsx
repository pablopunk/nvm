import { Check, ChevronRight, FileUp } from 'lucide-react';
import { useEffect, useState } from 'react';
import { formFieldValue, formRowId, invalidFormFields } from './form-fields';
import type { CommandAction, CommandView } from './model';
import type { ActionPanelRow, FormField, FormValue } from './ui';

export function usePaletteForm(
  view: CommandView | null,
  values: Record<string, FormValue>,
  setValues: (
    update: (current: Record<string, FormValue>) => Record<string, FormValue>,
  ) => void,
  submit: (action: CommandAction) => void,
  select: (value: string) => void,
) {
  const active = view?.type === 'form';
  const fields = active ? view.fields || [] : [];
  const [editingId, setEditingId] = useState<string | null>(null);
  const [query, setQuery] = useState('');
  const [errors, setErrors] = useState<Record<string, string>>({});
  const field = fields.find((candidate) => candidate.id === editingId);
  const editing = Boolean(field);
  const editor = editing && field?.type === 'textarea';
  const choices = editing && !editor;

  useEffect(() => {
    setEditingId(null);
    setQuery('');
    setErrors({});
  }, [view]);

  function update(id: string, value: FormValue) {
    setValues((current) => ({ ...current, [id]: value }));
    setErrors((current) => {
      if (!current[id]) return current;
      const next = { ...current };
      delete next[id];
      return next;
    });
  }

  function closeEditor() {
    const id = editingId;
    setEditingId(null);
    setQuery('');
    if (id) requestAnimationFrame(() => select(formRowId(id)));
  }

  function save() {
    if (!view?.submitAction) return;
    const nextErrors = invalidFormFields(fields, values);
    setErrors(nextErrors);
    const invalidId = Object.keys(nextErrors)[0];
    if (invalidId) {
      select(formRowId(invalidId));
      return;
    }
    submit({ ...view.submitAction, formValues: values });
  }

  async function choosePaths(fieldToEdit: FormField) {
    const result = await window.nvm.pickFormFieldPaths({
      type: fieldToEdit.type as 'file' | 'files' | 'folder',
      title: fieldToEdit.label,
      buttonLabel: fieldToEdit.buttonLabel,
      defaultPath: fieldToEdit.defaultPath,
      extensions: fieldToEdit.extensions,
      filterName: fieldToEdit.filterName,
      canCreateDirectories: fieldToEdit.canCreateDirectories,
    });
    if (result.canceled) return;
    update(
      fieldToEdit.id,
      fieldToEdit.type === 'files' ? result.paths : result.paths[0] || '',
    );
    if (editingId === fieldToEdit.id) closeEditor();
  }

  async function open(fieldToEdit: FormField) {
    if (fieldToEdit.disabled) return;
    if (fieldToEdit.type === 'checkbox') {
      update(fieldToEdit.id, !Boolean(formFieldValue(fieldToEdit, values)));
      return;
    }
    if (['file', 'files', 'folder'].includes(fieldToEdit.type || '')) {
      const current = formFieldValue(fieldToEdit, values);
      if (Array.isArray(current) ? !current.length : !current) {
        await choosePaths(fieldToEdit);
        return;
      }
    }
    setEditingId(fieldToEdit.id);
    setQuery(
      ['dropdown', 'select', 'multiselect', 'file', 'files', 'folder'].includes(
        fieldToEdit.type || '',
      )
        ? ''
        : String(formFieldValue(fieldToEdit, values)),
    );
  }

  function commit(value: FormValue) {
    if (!field) return;
    update(field.id, value);
    closeEditor();
  }

  function rows(): ActionPanelRow[] {
    if (!field || editor) return [];
    const current = formFieldValue(field, values);
    if (['file', 'files', 'folder'].includes(field.type || '')) {
      const options = [
        {
          value: 'form:choose-path',
          icon: <FileUp size={18} />,
          title: field.buttonLabel || `Choose ${field.label || field.id}`,
          onSelect: () => void choosePaths(field),
        },
        {
          value: 'form:clear-path',
          icon: <ChevronRight size={18} />,
          title: `Clear ${field.label || field.id}`,
          onSelect: () => commit(field.type === 'files' ? [] : ''),
        },
      ];
      return options.filter((option) =>
        option.title.toLowerCase().includes(query.toLowerCase()),
      );
    }
    if (
      field.type === 'dropdown' ||
      field.type === 'select' ||
      field.type === 'multiselect'
    ) {
      const selected = Array.isArray(current) ? current : [];
      const options = (field.options || [])
        .filter((option) =>
          `${option.title} ${option.value}`
            .toLowerCase()
            .includes(query.toLowerCase()),
        )
        .sort((left, right) =>
          field.type === 'multiselect'
            ? 0
            : Number(right.value === current) - Number(left.value === current),
        )
        .map((option) => ({
          value: `form:option:${option.value}`,
          icon: (
            <Check
              size={18}
              opacity={
                selected.includes(option.value) || current === option.value
                  ? 1
                  : 0
              }
            />
          ),
          title: option.title,
          onSelect: () => {
            if (field.type !== 'multiselect') return commit(option.value);
            update(
              field.id,
              selected.includes(option.value)
                ? selected.filter((value) => value !== option.value)
                : [...selected, option.value],
            );
          },
        }));
      if (field.type === 'multiselect')
        options.unshift({
          value: 'form:done',
          icon: <ChevronRight size={18} />,
          title: 'Done',
          onSelect: closeEditor,
        });
      return options;
    }
    const value = query.trim();
    return [
      {
        value: 'form:use-value',
        icon: <Check size={18} />,
        title: value
          ? `Set ${field.label || field.id}`
          : `Clear ${field.label || field.id}`,
        subtitle: field.type === 'password' ? undefined : value,
        onSelect: () =>
          commit(field.type === 'password' ? query : query.trim()),
      },
    ];
  }

  return {
    active,
    editing,
    editor,
    choices,
    field,
    query,
    setQuery,
    errors,
    rows: rows(),
    open,
    save,
    closeEditor,
    commit,
    placeholder:
      field?.placeholder ||
      (field?.type === 'date'
        ? 'YYYY-MM-DD'
        : `Enter ${field?.label || field?.id}`),
    concealed: field?.type === 'password',
    selectionKey: `${editingId || ''}:${query}`,
  };
}
