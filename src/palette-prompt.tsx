import { Check, ChevronRight, FileUp } from 'lucide-react';
import { useEffect, useState } from 'react';
import {
  type CommandAction,
  type CommandFormField,
  type CommandFormValue,
  type CommandView,
} from './model';
import type { ActionPanelRow } from './ui';

type PromptSubmit = (action: CommandAction) => void | Promise<void>;

function interactiveFields(view: CommandView | null) {
  return (view?.fields || []).filter(
    (field) => field.type !== 'description' && field.type !== 'separator',
  );
}

function seededValues(fields: CommandFormField[]) {
  return Object.fromEntries(
    fields.map((field) => {
      if (field.type === 'checkbox') return [field.id, Boolean(field.value)];
      if (field.type === 'multiselect' || field.type === 'files')
        return [field.id, Array.isArray(field.value) ? field.value : []];
      return [field.id, field.value || ''];
    }),
  ) as Record<string, CommandFormValue>;
}

function textValue(value: CommandFormValue | undefined) {
  return Array.isArray(value) ? value.join(', ') : String(value ?? '');
}

function fieldLabel(field: CommandFormField | undefined) {
  return field?.label || field?.id || 'value';
}

function initialQuery(
  field: CommandFormField | undefined,
  values: Record<string, CommandFormValue>,
) {
  if (!field) return '';
  if (
    field.type === 'dropdown' ||
    field.type === 'select' ||
    field.type === 'multiselect' ||
    field.type === 'checkbox' ||
    field.type === 'file' ||
    field.type === 'files' ||
    field.type === 'folder'
  )
    return '';
  return textValue(values[field.id]);
}

export function usePalettePrompt(
  view: CommandView | null,
  submit: PromptSubmit,
) {
  const active = view?.type === 'prompt';
  const fields = interactiveFields(active ? view : null);
  const resetKey = active
    ? `${view?.id || ''}:${view?.type}:${view?.title || ''}`
    : '';
  const [fieldIndex, setFieldIndex] = useState(0);
  const [values, setValues] = useState<Record<string, CommandFormValue>>({});
  const [query, setQuery] = useState('');
  const field = fields[fieldIndex];

  useEffect(() => {
    const nextValues = seededValues(fields);
    setFieldIndex(0);
    setValues(nextValues);
    setQuery(initialQuery(fields[0], nextValues));
  }, [resetKey]);

  function advance(value: CommandFormValue) {
    if (!(view && field)) return;
    const nextValues = { ...values, [field.id]: value };
    setValues(nextValues);
    const nextField = fields[fieldIndex + 1];
    if (nextField) {
      setFieldIndex((index) => index + 1);
      setQuery(initialQuery(nextField, nextValues));
      return;
    }
    if (view.submitAction)
      void submit({ ...view.submitAction, formValues: nextValues });
  }

  async function choosePaths() {
    if (!field) return;
    const type = field.type as 'file' | 'files' | 'folder';
    const result = await window.nvm.pickFormFieldPaths({
      type,
      title: field.label,
      buttonLabel: field.buttonLabel,
      defaultPath: field.defaultPath,
      extensions: field.extensions,
      filterName: field.filterName,
      canCreateDirectories: field.canCreateDirectories,
    });
    if (!result.canceled)
      advance(type === 'files' ? result.paths : result.paths[0] || '');
  }

  function textRows(): ActionPanelRow[] {
    if (!field) return [];
    const value = query.trim();
    const displayedValue = field.type === 'password' ? '******' : value;
    const missing = field.required && !value;
    return [
      {
        value: `prompt:${fieldIndex}:text`,
        icon: missing ? <ChevronRight size={18} /> : <Check size={18} />,
        title: missing
          ? `Enter ${fieldLabel(field)}`
          : fieldIndex === fields.length - 1
            ? view?.submitAction?.title || 'Submit'
            : 'Continue',
        subtitle: missing
          ? field.error || 'Type a value above'
          : `Use "${displayedValue}" for ${fieldLabel(field)}`,
        onSelect: () => {
          if (!missing) advance(value);
        },
      },
    ];
  }

  function choiceRows(): ActionPanelRow[] {
    if (!field) return [];
    const selected = Array.isArray(values[field.id])
      ? (values[field.id] as string[])
      : [];
    const selectedValue = textValue(values[field.id]);
    const options = (field.options || [])
      .filter((option) =>
        `${option.title} ${option.value}`
          .toLowerCase()
          .includes(query.toLowerCase()),
      )
      .sort((left, right) =>
        left.value === selectedValue
          ? -1
          : right.value === selectedValue
            ? 1
            : 0,
      );
    const rows = options.map((option) => ({
      value: `prompt:${fieldIndex}:option:${option.value}`,
      icon: (
        <Check
          size={18}
          opacity={
            selected.includes(option.value) || option.value === selectedValue
              ? 1
              : 0
          }
        />
      ),
      title: option.title,
      subtitle: option.value,
      onSelect: () => {
        if (field.type !== 'multiselect') {
          advance(option.value);
          return;
        }
        setValues((current) => ({
          ...current,
          [field.id]: selected.includes(option.value)
            ? selected.filter((value) => value !== option.value)
            : [...selected, option.value],
        }));
      },
    }));
    if (field.type === 'multiselect')
      rows.unshift({
        value: `prompt:${fieldIndex}:continue`,
        icon: <ChevronRight size={18} />,
        title: fieldIndex === fields.length - 1 ? 'Submit' : 'Continue',
        subtitle: `${selected.length} selected`,
        onSelect: () => advance(selected),
      });
    return rows;
  }

  function rows(): ActionPanelRow[] {
    if (!active || !field) return [];
    if (
      field.type === 'dropdown' ||
      field.type === 'select' ||
      field.type === 'multiselect'
    )
      return choiceRows();
    if (field.type === 'checkbox')
      return [
        {
          value: `prompt:${fieldIndex}:yes`,
          icon: <Check size={18} />,
          title: 'Yes',
          subtitle: fieldLabel(field),
          onSelect: () => advance(true),
        },
        {
          value: `prompt:${fieldIndex}:no`,
          icon: <ChevronRight size={18} />,
          title: 'No',
          subtitle: fieldLabel(field),
          onSelect: () => advance(false),
        },
      ].sort((left) =>
        Boolean(values[field.id]) === left.value.endsWith(':yes') ? -1 : 1,
      );
    if (
      field.type === 'file' ||
      field.type === 'files' ||
      field.type === 'folder'
    )
      return [
        {
          value: `prompt:${fieldIndex}:pick`,
          icon: <FileUp size={18} />,
          title: field.buttonLabel || `Choose ${fieldLabel(field)}`,
          subtitle: field.description || field.placeholder,
          onSelect: () => void choosePaths(),
        },
      ];
    return textRows();
  }

  const promptRows = rows();
  return {
    active,
    query,
    setQuery,
    rows: promptRows,
    fieldIndex,
    resetKey,
    selectionKey: promptRows.map((row) => row.value).join(':'),
    placeholder: field?.placeholder || `Enter ${fieldLabel(field)}`,
    progress:
      fields.length > 1 ? `${fieldIndex + 1} of ${fields.length}` : undefined,
    concealed: field?.type === 'password',
  };
}
