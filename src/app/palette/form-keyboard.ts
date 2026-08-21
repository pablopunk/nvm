export type FormKeyboardAction =
  | 'advance'
  | 'field'
  | 'host'
  | 'submit'
  | 'toggle';

export function formKeyboardActionForEvent(input: {
  key: string;
  metaKey?: boolean;
  ctrlKey?: boolean;
  altKey?: boolean;
  shiftKey?: boolean;
  targetTag?: string;
  inputType?: string;
}): FormKeyboardAction {
  const commandKey = input.metaKey || input.ctrlKey;
  if (input.key === 'Escape' || (commandKey && input.key.toLowerCase() === 'k'))
    return 'host';
  if (commandKey && input.key === 'Enter') return 'submit';
  if (commandKey || input.altKey) return 'host';
  if (input.key !== 'Enter' || input.shiftKey || input.targetTag !== 'INPUT')
    return 'field';
  return input.inputType === 'checkbox' ? 'toggle' : 'advance';
}
