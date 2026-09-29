import type { FormField, FormValue } from './ui';

export function formRowId(id: string) {
  return `form:field:${id}`;
}

export const FORM_SAVE_ROW_ID = 'form:save';

export function formApplyRowId(id: string) {
  return `form:apply:${id}`;
}

export function formFieldValue(
  field: FormField,
  values: Record<string, FormValue>,
) {
  return values[field.id] ?? field.value ?? '';
}

export function formValueLabel(field: FormField, value: FormValue) {
  if (field.type === 'checkbox') return value ? 'On' : 'Off';
  if (field.type === 'password') return value ? '••••••' : 'Not set';
  if (Array.isArray(value))
    return value.length ? `${value.length} selected` : 'None';
  if (!value) return 'Not set';
  if (field.type === 'textarea') {
    const lines = String(value).split('\n').filter(Boolean).length;
    return lines ? `${lines} ${lines === 1 ? 'line' : 'lines'}` : 'Not set';
  }
  return (
    field.options?.find((option) => option.value === value)?.title ||
    String(value)
  );
}

export function formFieldMatches(
  field: FormField,
  value: FormValue,
  query: string,
) {
  return `${field.label || field.id} ${field.description || ''} ${formValueLabel(field, value)}`
    .toLowerCase()
    .includes(query.trim().toLowerCase());
}

export function invalidFormFields(
  fields: FormField[],
  values: Record<string, FormValue>,
) {
  const errors: Record<string, string> = {};
  for (const field of fields) {
    if (
      field.disabled ||
      field.type === 'description' ||
      field.type === 'separator'
    )
      continue;
    const value = formFieldValue(field, values);
    if (
      field.required &&
      (Array.isArray(value) ? !value.length : !value || !String(value).trim())
    ) {
      errors[field.id] = `${field.label || field.id} is required`;
      continue;
    }
    if (!value || Array.isArray(value) || typeof value !== 'string') continue;
    if (!['email', 'url', 'number', 'date'].includes(field.type || ''))
      continue;
    const input = document.createElement('input');
    input.type = field.type || 'text';
    input.value = value;
    if (!input.checkValidity() || input.value !== value)
      errors[field.id] = `Enter a valid ${field.label || field.id}`;
  }
  return errors;
}
