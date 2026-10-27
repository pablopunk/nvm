import { useEffect, useRef, useState } from 'react';
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
type Props = { options: Option[]; defaultValue?: string; label: string };

export function ConsoleCombobox({ options, defaultValue, label }: Props) {
  const initial =
    defaultValue ?? options.find((option) => !option.disabled)?.value ?? '';
  const [value, setValue] = useState(initial);
  const [open, setOpen] = useState(false);
  const [hydrated, setHydrated] = useState(false);
  const select = useRef<HTMLSelectElement>(null);

  useEffect(
    function activateSearch() {
      setValue(select.current?.value ?? initial);
      setHydrated(true);
    },
    [initial],
  );

  function choose(next: string) {
    setValue(next);
    setOpen(false);
    // Keep the host-owned save handler on the native select as the single writer.
    if (select.current && select.current.value !== next) {
      select.current.value = next;
      select.current.dispatchEvent(new Event('change', { bubbles: true }));
    }
  }

  return (
    <div className="min-w-0 w-full">
      <select
        ref={select}
        defaultValue={initial}
        hidden={hydrated}
        className="model-select console-fallback-select"
        aria-label={label}
      >
        {options.map(function renderNativeOption(option) {
          return (
            <option
              key={option.value}
              value={option.value}
              disabled={option.disabled}
            >
              {option.label}
            </option>
          );
        })}
      </select>
      {hydrated && (
        <Popover open={open} onOpenChange={setOpen}>
          <PopoverTrigger asChild>
            <Button
              variant="outline"
              role="combobox"
              aria-expanded={open}
              aria-label={label}
              disabled={options.length === 0}
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
      )}
    </div>
  );
}
