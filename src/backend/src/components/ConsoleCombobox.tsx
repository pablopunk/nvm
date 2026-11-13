import { useState } from 'react';
import { Check, ChevronsUpDown } from 'lucide-react';
import { Button } from './ui/button';
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from './ui/command';
import { Popover, PopoverContent, PopoverTrigger } from './ui/popover';
import { cn } from '../lib/utils';

type Option = { value: string; label: string; disabled?: boolean };
type Props = {
  options: Option[];
  value: string;
  onValueChange: (value: string) => void;
  label: string;
  disabled?: boolean;
};

export function ConsoleCombobox({ options, value, onValueChange, label, disabled = false }: Props) {
  const [open, setOpen] = useState(false);

  function choose(next: string) {
    setOpen(false);
    if (next !== value) onValueChange(next);
  }

  return (
    <div className="min-w-0 w-full">
        <Popover open={open} onOpenChange={setOpen}>
          <PopoverTrigger asChild>
            <Button
              variant="outline"
              role="combobox"
              aria-expanded={open}
              aria-label={label}
              disabled={disabled || options.length === 0}
              className="w-full min-w-0 justify-between"
            >
              <span className="truncate">
                {options.find((option) => option.value === value)?.label ??
                  'No models available'}
              </span>
              <ChevronsUpDown className="size-4 shrink-0 opacity-50" />
            </Button>
          </PopoverTrigger>
          <PopoverContent
            className="w-[var(--radix-popover-trigger-width)] p-0"
            align="start"
          >
            <Command>
              <CommandInput
                placeholder="Search models…"
                aria-label={`Search ${label}`}
              />
              <CommandList>
                <CommandEmpty>No matching models.</CommandEmpty>
                <CommandGroup>
                  {options.map(function renderOption(option) {
                    return (
                      <CommandItem
                        key={option.value}
                        value={option.value}
                        keywords={[option.label]}
                        disabled={option.disabled}
                        onSelect={choose}
                      >
                        <Check
                          className={cn(
                            'size-4 shrink-0',
                            value === option.value
                              ? 'opacity-100'
                              : 'opacity-0',
                          )}
                        />
                        <span className="break-all">{option.label}</span>
                      </CommandItem>
                    );
                  })}
                </CommandGroup>
              </CommandList>
            </Command>
          </PopoverContent>
        </Popover>
    </div>
  );
}
