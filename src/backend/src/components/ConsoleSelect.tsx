import type { ComponentProps } from 'react';
import { NativeSelect, NativeSelectOption } from './ui/native-select';

type Option = { value: string; label: string; disabled?: boolean };
type Props = Omit<ComponentProps<typeof NativeSelect>, 'children'> & { options: Option[] };

export function ConsoleSelect({ options, ...props }: Props) {
  return (
    <NativeSelect {...props}>
      {options.map((option) => <NativeSelectOption key={option.value} value={option.value} disabled={option.disabled}>{option.label}</NativeSelectOption>)}
    </NativeSelect>
  );
}
