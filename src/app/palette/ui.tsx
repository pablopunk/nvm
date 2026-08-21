// biome-ignore-all lint: This legacy shared UI module retains established renderer conventions.
import { Command } from 'cmdk';
import { Folder } from 'lucide-react';
import React, { type ReactNode, useId, useLayoutEffect, useRef } from 'react';
import ReactMarkdown, { defaultUrlTransform } from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { formKeyboardActionForEvent } from './form-keyboard';
import { MarkdownEditor } from './markdown-editor';
import type { CommandImage } from './model';

export const EMPTY_ROOT_TITLE = 'Type anything';
export const EMPTY_ROOT_SUBTITLE =
  'Nevermind starts with local actions; AI planning comes next.';
export const EMPTY_RESULTS_TITLE = 'No results';
export const EMPTY_ITEMS_TITLE = 'No items';
export const EMPTY_ACTIONS_TITLE = 'No actions';
export const EMPTY_SHORTCUTS_TITLE = 'No keyboard shortcuts';

export interface KeyHintsProps {
  shortcut?: string;
  extras?: string[];
  showEnter?: boolean;
}
export interface ItemAppearance {
  foreground?: string;
  background?: 'accent';
}
export interface CommandRowProps {
  value: string;
  icon: ReactNode;
  title: string;
  subtitle?: string;
  accessories?: {
    text?: string;
    icon?: ReactNode;
    tone?: string;
    tooltip?: string;
  }[];
  shortcut?: string;
  extras?: string[];
  className?: string;
  appearance?: ItemAppearance;
  selectedOnlyShortcut?: boolean;
  disabled?: boolean;
  onSelect: () => void;
}
export interface CommandTileProps {
  value: string;
  title: string;
  subtitle?: string;
  glyph?: string;
  image?: CommandImage;
  video?: string;
  appearance?: ItemAppearance;
  draggable?: boolean;
  onDragStart?: (event: React.DragEvent) => void;
  onSelect: () => void;
}
export interface EmptyStateProps {
  icon: ReactNode;
  title: string;
  subtitle?: string;
  action?: ActionPanelRow;
}
export type ToastTone = 'default' | 'info' | 'success' | 'error';
export interface ToastProps {
  message: string;
  tone?: ToastTone;
}
export interface PreviewViewProps {
  content?: ReactNode;
  image?: CommandImage;
  video?: string;
  poster?: string;
  actions?: ReactNode;
}
export interface ProgressViewProps {
  steps: { title: string; status?: string }[];
  value?: number;
  total?: number;
  label?: string;
  status?: string;
}
export type FormValue = string | boolean | string[];
export interface FormField {
  id: string;
  label?: string;
  type?: string;
  value?: FormValue;
  placeholder?: string;
  required?: boolean;
  options?: { title: string; value: string }[];
  description?: string;
  error?: string;
  rows?: number;
  extensions?: string[];
  filterName?: string;
  buttonLabel?: string;
  defaultPath?: string;
  canCreateDirectories?: boolean;
}
export interface FormViewProps {
  fields: FormField[];
  values?: Record<string, FormValue>;
  onChange?: (id: string, value: FormValue) => void;
  onSubmit?: () => void;
  submitTitle?: string;
  autoFocus?: boolean;
  autoFocusKey?: string | number;
}
export interface EditorViewProps {
  value: string;
  title?: string;
  subtitle?: string;
  format?: 'text' | 'markdown';
  language?: string;
  placeholder?: string;
  readOnly?: boolean;
  autoFocus?: boolean;
  preview?: ReactNode;
  actions?: ReactNode;
  submitTitle?: string;
  onChange?: (value: string) => void;
  onFlush?: (value: string) => void;
  onSubmit?: () => void;
}
export interface ItemSection<T> {
  title?: string;
  subtitle?: string;
  items: T[];
}
export interface ListViewProps<T> {
  items?: T[];
  sections?: ItemSection<T>[];
  renderItem: (item: T) => ReactNode;
  empty?: ReactNode;
  subtitle?: string;
  isLoading?: boolean;
  pagination?: ReactNode;
}
export interface GridViewProps<T> {
  items?: T[];
  sections?: ItemSection<T>[];
  renderItem: (item: T) => ReactNode;
  empty?: ReactNode;
  subtitle?: string;
  layout?: string;
  style?: React.CSSProperties;
  isLoading?: boolean;
  pagination?: ReactNode;
}
export interface ChatViewProps {
  messages: {
    role: string;
    content: ReactNode;
    images?: { url: string; alt?: string }[];
  }[];
  isBusy?: boolean;
  input?: ReactNode;
  messagesRef?: React.RefObject<HTMLDivElement | null>;
  banner?: ReactNode;
}
export interface ActionPanelRow {
  value: string;
  icon?: ReactNode;
  title: string;
  subtitle?: string;
  shortcut?: string;
  className?: string;
  sectionHeader?: boolean;
  disabled?: boolean;
  onSelect: () => void;
}
export interface ActionPanelViewProps {
  rows: ActionPanelRow[];
  renderEmpty: () => ReactNode;
}
export interface SearchAccessoryProps {
  tooltip?: string;
  value?: string;
  items: { title: string; value: string }[];
  onChange?: (value: string) => void;
}
export interface MarkdownContentProps {
  content: string;
}

let shortcutLabelHyperKey = 'Command+Control+Alt+Shift';

export function setShortcutLabelHyperKey(shortcut: unknown) {
  shortcutLabelHyperKey = String(shortcut || '').trim();
}

function shortcutLabelParts(shortcut?: string) {
  const parts = String(shortcut || '')
    .split('+')
    .filter(Boolean);
  const hyperParts = shortcutLabelHyperKey.split('+').filter(Boolean);
  const startsWithHyper =
    hyperParts.length > 0 &&
    hyperParts.every((part, index) => parts[index] === part);
  return startsWithHyper ? ['✦', ...parts.slice(hyperParts.length)] : parts;
}

export function shortcutLabel(shortcut?: string) {
  return shortcutLabelParts(shortcut)
    .map(
      (part) =>
        ({
          Command: '⌘',
          Cmd: '⌘',
          Control: '⌃',
          Ctrl: '⌃',
          Alt: '⌥',
          Option: '⌥',
          Shift: '⇧',
          Enter: '↵',
          Return: '↵',
          Backspace: '⌫',
          Escape: 'Esc',
          Tab: 'Tab',
        })[part] || part,
    )
    .join('');
}

export function KeyHints({
  shortcut,
  extras = [],
  showEnter = true,
}: KeyHintsProps) {
  return (
    <span className="keyHints">
      {extras.map((extra) => (
        <span key={extra} className="shortcutHint selectedOnlyEnter">
          {extra}
        </span>
      ))}
      {shortcut ? (
        <span className="shortcutHint">{shortcutLabel(shortcut)}</span>
      ) : null}
      {showEnter ? (
        <span className="enterHint selectedOnlyEnter" aria-label="Enter">
          <span aria-hidden="true">↵</span>
        </span>
      ) : null}
    </span>
  );
}

const MAX_VISIBLE_ACCESSORIES = 3;

function imageProps(image?: CommandImage) {
  if (!image) return null;
  if (typeof image === 'string')
    return { src: image, alt: '', fit: undefined, shape: undefined };
  return {
    src: image.dark || image.src || image.light || image.fallback || '',
    alt: image.alt || '',
    fit: image.fit,
    shape: image.shape || image.mask,
  };
}

export function CommandRow({
  value,
  icon,
  title,
  subtitle,
  accessories = [],
  shortcut,
  extras,
  className,
  appearance,
  selectedOnlyShortcut = false,
  disabled,
  onSelect,
}: CommandRowProps) {
  const keyHints = selectedOnlyShortcut ? (
    shortcut ? (
      <span className="keyHints selectedOnlyEnter">
        <span className="shortcutHint">{shortcutLabel(shortcut)}</span>
        <span className="enterHint" aria-label="Enter">
          <span aria-hidden="true">↵</span>
        </span>
      </span>
    ) : null
  ) : (
    <KeyHints shortcut={shortcut} extras={extras} />
  );
  const itemClassName = className ? `result ${className}` : 'result';
  const visibleAccessories = accessories.slice(0, MAX_VISIBLE_ACCESSORIES);
  const overflowAccessories = accessories.slice(MAX_VISIBLE_ACCESSORIES);
  const overflowTitle = overflowAccessories
    .map((accessory) => accessory.text)
    .filter(Boolean)
    .join(', ');
  return (
    <Command.Item
      value={value}
      className={itemClassName}
      data-foreground={appearance?.foreground}
      data-background={appearance?.background}
      disabled={disabled}
      data-disabled={disabled ? 'true' : undefined}
      aria-disabled={disabled ? 'true' : undefined}
      onSelect={() => {
        if (!disabled) onSelect();
      }}
    >
      <span className="resultIcon">{icon}</span>
      <span className="resultText">
        {disabled ? title : <strong>{title}</strong>}
        <small>{subtitle}</small>
      </span>
      <span className="resultTrailing">
        {accessories.length ? (
          <span className="accessories">
            {visibleAccessories.map((accessory, index) => (
              <span
                key={index}
                className="accessory"
                data-tone={accessory.tone || 'default'}
                title={accessory.tooltip || accessory.text}
              >
                {accessory.icon}
                {accessory.text ? (
                  <span className="accessoryText">{accessory.text}</span>
                ) : null}
              </span>
            ))}
            {overflowAccessories.length ? (
              <span className="accessoryOverflow" title={overflowTitle}>
                +{overflowAccessories.length}
              </span>
            ) : null}
          </span>
        ) : null}
        {keyHints}
      </span>
    </Command.Item>
  );
}

export function CommandTile({
  value,
  title,
  subtitle,
  glyph,
  image,
  video,
  appearance,
  draggable,
  onDragStart,
  onSelect,
}: CommandTileProps) {
  const media = imageProps(image);
  const visual = glyph ? (
    <span className="tileIcon tileGlyph" aria-hidden="true">
      {glyph}
    </span>
  ) : media?.src ? (
    <img
      src={media.src}
      alt={media.alt}
      draggable={false}
      loading="lazy"
      decoding="async"
    />
  ) : video ? (
    <video
      src={video}
      draggable={false}
      muted={true}
      loop={true}
      playsInline={true}
      preload="none"
      onMouseEnter={(event) => event.currentTarget.play().catch(() => {})}
      onMouseLeave={(event) => event.currentTarget.pause()}
    />
  ) : (
    <span className="tileIcon">
      <Folder size={20} />
    </span>
  );
  return (
    <Command.Item
      value={value}
      className="extensionTile"
      data-extension-item-id={value}
      data-foreground={appearance?.foreground}
      data-background={appearance?.background}
      draggable={draggable}
      onDragStart={onDragStart}
      onSelect={onSelect}
    >
      <span
        className="tileMedia"
        data-fit={media?.fit}
        data-shape={media?.shape}
      >
        {visual}
      </span>
      <strong>{title}</strong>
      {subtitle ? <small>{subtitle}</small> : null}
    </Command.Item>
  );
}

export function EmptyState({ icon, title, subtitle, action }: EmptyStateProps) {
  return (
    <div className="empty" role="status">
      {icon}
      <strong>{title}</strong>
      {subtitle ? <span>{subtitle}</span> : null}
      {action ? (
        <CommandRow
          value={action.value}
          icon={action.icon}
          title={action.title}
          onSelect={action.onSelect}
        />
      ) : null}
    </div>
  );
}

export function Toast({ message, tone }: ToastProps) {
  const resolvedTone = tone || 'default';
  return (
    <div
      className={`toast toast-${resolvedTone}`}
      role={resolvedTone === 'error' ? 'alert' : 'status'}
    >
      {message}
    </div>
  );
}

export function SearchAccessory({
  tooltip,
  value,
  items,
  onChange,
}: SearchAccessoryProps) {
  return (
    <select
      className="searchAccessory"
      aria-label={tooltip || 'View filter'}
      value={value || items[0]?.value || ''}
      onChange={(event) => onChange?.(event.target.value)}
    >
      {items.map((item) => (
        <option key={item.value} value={item.value}>
          {item.title}
        </option>
      ))}
    </select>
  );
}

export function MarkdownContent({ content }: MarkdownContentProps) {
  return (
    <div className="markdownContent">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        urlTransform={defaultUrlTransform}
        components={{
          h1: ({ children }) => (
            <h1 className="markdownHeading markdownHeading1">{children}</h1>
          ),
          h2: ({ children }) => (
            <h2 className="markdownHeading markdownHeading2">{children}</h2>
          ),
          h3: ({ children }) => (
            <h3 className="markdownHeading markdownHeading3">{children}</h3>
          ),
          h4: ({ children }) => (
            <h4 className="markdownHeading markdownHeading4">{children}</h4>
          ),
          h5: ({ children }) => (
            <h5 className="markdownHeading markdownHeading5">{children}</h5>
          ),
          h6: ({ children }) => (
            <h6 className="markdownHeading markdownHeading6">{children}</h6>
          ),
          a: ({ children, href }) => (
            <a href={href} target="_blank" rel="noreferrer">
              {children}
            </a>
          ),
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  );
}

export function PreviewView({
  content,
  image,
  video,
  poster,
  actions,
}: PreviewViewProps) {
  const media = imageProps(image);
  return (
    <div className="extensionView previewView">
      {video ? (
        <video
          className="previewMedia"
          src={video}
          poster={poster || media?.src}
          controls={true}
          autoPlay={true}
          muted={true}
          loop={true}
          playsInline={true}
        />
      ) : null}
      {!video && media?.src ? (
        <img
          className="previewMedia"
          src={media.src}
          alt={media.alt}
          data-fit={media.fit}
          data-shape={media.shape}
        />
      ) : null}
      <div className="previewText">{content}</div>
      {actions}
    </div>
  );
}

function normalizedProgressStatus(status?: string) {
  const value = String(status || '').toLowerCase();
  if (['done', 'complete', 'completed', 'success'].includes(value))
    return 'done';
  if (
    ['active', 'running', 'loading', 'in progress', 'progress'].includes(value)
  )
    return 'active';
  if (['error', 'failed', 'failure'].includes(value)) return 'error';
  return 'pending';
}

export function ProgressView({
  steps,
  value,
  total,
  label,
  status,
}: ProgressViewProps) {
  const hasProgress =
    typeof value === 'number' && typeof total === 'number' && total > 0;
  const ratio = hasProgress ? Math.max(0, Math.min(1, value / total)) : 0;
  const percent = Math.round(ratio * 100);
  const showSummary = Boolean(label || status) || hasProgress;
  return (
    <div className="extensionView progressView">
      {showSummary ? (
        <div className="progressOverview">
          <div>
            <strong>{label || status || 'Working…'}</strong>
            {hasProgress ? (
              <small>
                {value} of {total} · {percent}%
              </small>
            ) : null}
          </div>
          <div
            className="progressBar"
            role="progressbar"
            aria-valuenow={hasProgress ? value : undefined}
            aria-valuemin={hasProgress ? 0 : undefined}
            aria-valuemax={hasProgress ? total : undefined}
            aria-label={status || 'Progress'}
          >
            <span style={{ width: hasProgress ? `${percent}%` : undefined }} />
          </div>
        </div>
      ) : null}
      {steps.map((step, index) => (
        <div
          key={index}
          className="progressStep"
          data-status={normalizedProgressStatus(step.status)}
        >
          <span className="progressStepMarker" aria-hidden="true" />
          <div>
            <strong>{step.title}</strong>
            <small>{step.status || 'Pending'}</small>
          </div>
        </div>
      ))}
    </div>
  );
}

function normalizedFormValue(value: FormValue | undefined) {
  return value === undefined ? '' : value;
}

function formFieldErrorId(formId: string, field: FormField) {
  return `${formId}-form-field-error-${field.id}`;
}

function formFieldDescriptionId(formId: string, field: FormField) {
  return `${formId}-form-field-description-${field.id}`;
}

function formFieldControlId(formId: string, field: FormField) {
  return `${formId}-form-field-control-${field.id}`;
}

function formFieldDescribedBy(formId: string, field: FormField) {
  return [
    field.description ? formFieldDescriptionId(formId, field) : '',
    field.error ? formFieldErrorId(formId, field) : '',
  ]
    .filter(Boolean)
    .join(' ');
}

function formFieldControl(
  field: FormField,
  value: FormValue,
  formId: string,
  onChange?: FormViewProps['onChange'],
) {
  const type = field.type || 'text';
  const controlId = formFieldControlId(formId, field);
  const describedBy = formFieldDescribedBy(formId, field) || undefined;
  if (type === 'file' || type === 'files' || type === 'folder') {
    const values = Array.isArray(value)
      ? value
      : String(value || '')
          .split('\n')
          .map((item) => item.trim())
          .filter(Boolean);
    const label = values.length ? values.join('\n') : '';
    const placeholder =
      field.placeholder ||
      (type === 'folder'
        ? 'No folder selected'
        : type === 'files'
          ? 'No files selected'
          : 'No file selected');
    async function choosePath() {
      const result = await window.nvm.pickFormFieldPaths({
        type: type as 'file' | 'files' | 'folder',
        title: field.label,
        buttonLabel: field.buttonLabel,
        defaultPath: field.defaultPath,
        extensions: field.extensions,
        filterName: field.filterName,
        canCreateDirectories: field.canCreateDirectories,
      });
      if (result.canceled) return;
      onChange?.(
        field.id,
        type === 'files' ? result.paths : result.paths[0] || '',
      );
    }
    function clearPath() {
      onChange?.(field.id, type === 'files' ? [] : '');
    }
    return (
      <div className="formPickerControl">
        <pre title={label || placeholder}>{label || placeholder}</pre>
        <button
          type="button"
          aria-label={`${field.buttonLabel || 'Choose'} ${field.label || field.id}`}
          onClick={choosePath}
        >
          {field.buttonLabel || 'Choose…'}
        </button>
        {values.length ? (
          <button
            type="button"
            className="formPickerClear"
            aria-label={`Clear ${field.label || field.id}`}
            onClick={clearPath}
          >
            Clear
          </button>
        ) : null}
      </div>
    );
  }
  if (type === 'description')
    return (
      <p className="formDescription">{field.description || field.label}</p>
    );
  if (type === 'separator') return <hr className="formSeparator" />;
  if (type === 'textarea')
    return (
      <textarea
        id={controlId}
        value={String(value)}
        placeholder={field.placeholder}
        required={field.required}
        aria-invalid={field.error ? true : undefined}
        aria-describedby={describedBy}
        rows={field.rows || 4}
        onChange={(event) => onChange?.(field.id, event.currentTarget.value)}
      />
    );
  if (type === 'checkbox')
    return (
      <label className="formCheckbox">
        <input
          id={controlId}
          checked={Boolean(value)}
          required={field.required}
          type="checkbox"
          aria-invalid={field.error ? true : undefined}
          aria-describedby={describedBy}
          onChange={(event) =>
            onChange?.(field.id, event.currentTarget.checked)
          }
        />
        <span>{field.label}</span>
      </label>
    );
  if (type === 'dropdown' || type === 'select')
    return (
      <select
        id={controlId}
        value={String(value)}
        required={field.required}
        aria-invalid={field.error ? true : undefined}
        aria-describedby={describedBy}
        onChange={(event) => onChange?.(field.id, event.currentTarget.value)}
      >
        {field.placeholder ? (
          <option value="">{field.placeholder}</option>
        ) : null}
        {(field.options || []).map((option) => (
          <option key={option.value} value={option.value}>
            {option.title}
          </option>
        ))}
      </select>
    );
  if (type === 'multiselect') {
    const selected = Array.isArray(value)
      ? value
      : String(value || '')
          .split(',')
          .map((item) => item.trim())
          .filter(Boolean);
    return (
      <fieldset
        id={controlId}
        className="formMultiselect"
        aria-invalid={field.error ? true : undefined}
        aria-describedby={describedBy}
      >
        <legend>{field.label || field.id}</legend>
        {(field.options || []).map((option, index) => (
          <label key={option.value}>
            <input
              type="checkbox"
              checked={selected.includes(option.value)}
              required={field.required && selected.length === 0 && index === 0}
              onChange={(event) =>
                onChange?.(
                  field.id,
                  event.currentTarget.checked
                    ? [...selected, option.value]
                    : selected.filter((value) => value !== option.value),
                )
              }
            />
            <span>{option.title}</span>
          </label>
        ))}
      </fieldset>
    );
  }
  return (
    <input
      id={controlId}
      value={String(value)}
      placeholder={field.placeholder}
      required={field.required}
      type={type}
      aria-invalid={field.error ? true : undefined}
      aria-describedby={describedBy}
      onChange={(event) => onChange?.(field.id, event.currentTarget.value)}
    />
  );
}

export function FormView({
  fields,
  values = {},
  onChange,
  onSubmit,
  submitTitle = 'Submit',
  autoFocus = true,
  autoFocusKey = 0,
}: FormViewProps) {
  const formRef = useRef<HTMLFormElement>(null);
  const hasAutoFocusedRef = useRef(false);
  const previousAutoFocusKeyRef = useRef(autoFocusKey);
  const formId = useId();
  useLayoutEffect(() => {
    if (previousAutoFocusKeyRef.current !== autoFocusKey) {
      previousAutoFocusKeyRef.current = autoFocusKey;
      hasAutoFocusedRef.current = false;
    }
    if (!autoFocus || hasAutoFocusedRef.current) return;
    const frame = requestAnimationFrame(() => {
      hasAutoFocusedRef.current = true;
      formRef.current
        ?.querySelector<HTMLElement>(
          'input:not(:disabled), textarea:not(:disabled), select:not(:disabled), button:not(:disabled)',
        )
        ?.focus();
    });
    return () => cancelAnimationFrame(frame);
  }, [autoFocus, autoFocusKey]);

  return (
    <form
      ref={formRef}
      className="extensionView formView"
      aria-keyshortcuts="Meta+Enter Control+Enter"
      onKeyDown={(event) => {
        const control = event.target;
        const action = formKeyboardActionForEvent({
          key: event.key,
          metaKey: event.metaKey,
          ctrlKey: event.ctrlKey,
          altKey: event.altKey,
          shiftKey: event.shiftKey,
          targetTag:
            control instanceof HTMLElement ? control.tagName : undefined,
          inputType:
            control instanceof HTMLInputElement ? control.type : undefined,
        });
        if (action === 'host') return;
        if (action === 'submit') {
          event.preventDefault();
          event.stopPropagation();
          event.currentTarget.requestSubmit();
          return;
        }
        event.stopPropagation();
        if (action === 'field' || !(control instanceof HTMLInputElement))
          return;
        if (action === 'toggle') {
          event.preventDefault();
          control.click();
          return;
        }
        event.preventDefault();
        const controls = Array.from(
          event.currentTarget.querySelectorAll<HTMLElement>(
            'input:not(:disabled), textarea:not(:disabled), select:not(:disabled), button:not(:disabled)',
          ),
        );
        controls[controls.indexOf(control) + 1]?.focus();
      }}
      onSubmit={(event) => {
        event.preventDefault();
        onSubmit?.();
      }}
    >
      <div className="formFields">
        {fields.map((field) => {
          const type = field.type || 'text';
          const value = normalizedFormValue(values[field.id] ?? field.value);
          if (type === 'description' || type === 'separator')
            return (
              <div
                key={field.id}
                className={`formStaticField formStaticField-${type}`}
              >
                {formFieldControl(field, value, formId, onChange)}
              </div>
            );
          return (
            <div key={field.id} className={`formField formField-${type}`}>
              {type !== 'checkbox' &&
              type !== 'multiselect' &&
              type !== 'file' &&
              type !== 'files' &&
              type !== 'folder' ? (
                <label
                  className="formFieldLabel"
                  htmlFor={formFieldControlId(formId, field)}
                >
                  {field.label || field.id}
                </label>
              ) : type !== 'checkbox' ? (
                <span className="formFieldLabel">
                  {field.label || field.id}
                </span>
              ) : null}
              {formFieldControl(field, value, formId, onChange)}
              {field.description ? (
                <small id={formFieldDescriptionId(formId, field)}>
                  {field.description}
                </small>
              ) : null}
              {field.error ? (
                <small
                  id={formFieldErrorId(formId, field)}
                  className="formFieldError"
                >
                  {field.error}
                </small>
              ) : null}
            </div>
          );
        })}
      </div>
      {onSubmit ? (
        <footer className="formFooter">
          <span className="formKeyboardHint">
            <kbd>Tab</kbd> Move between fields
          </span>
          <button
            className="formSubmitButton"
            type="submit"
            title="Command+Enter"
          >
            <span>{submitTitle}</span>
            <kbd>{shortcutLabel('Command+Enter')}</kbd>
          </button>
        </footer>
      ) : null}
    </form>
  );
}

export function EditorView({
  value,
  title,
  subtitle,
  format = 'text',
  language,
  placeholder,
  readOnly,
  autoFocus,
  preview,
  actions,
  submitTitle = 'Save',
  onChange,
  onFlush,
  onSubmit,
}: EditorViewProps) {
  const showsPreview = format === 'markdown' && Boolean(preview);
  return (
    <div
      className={`extensionView editorView ${showsPreview ? 'editorViewSplit' : ''} ${format === 'markdown' ? 'editorViewMarkdown' : ''}`}
    >
      {title || subtitle ? (
        <header className="editorHeader">
          {title ? <span className="editorHeaderTitle">{title}</span> : null}
          {subtitle ? (
            <span className="editorHeaderSubtitle">{subtitle}</span>
          ) : null}
        </header>
      ) : null}
      <div className="editorPane">
        {showsPreview || language ? (
          <div className="editorToolbar">
            <span>{format === 'markdown' ? 'Markdown' : 'Plain text'}</span>
            {language ? <small>{language}</small> : null}
          </div>
        ) : null}
        {format === 'markdown' ? (
          <MarkdownEditor
            value={value}
            placeholder={placeholder}
            readOnly={readOnly}
            autoFocus={autoFocus}
            onChange={onChange}
            onFlush={onFlush}
          />
        ) : (
          <textarea
            className="editorTextarea"
            value={value}
            placeholder={placeholder}
            readOnly={readOnly}
            autoFocus={autoFocus}
            spellCheck={true}
            onKeyDown={(event) => {
              if (event.key === 'Escape') return;
              if (event.metaKey || event.ctrlKey || event.altKey) return;
              event.stopPropagation();
            }}
            onChange={(event) => onChange?.(event.currentTarget.value)}
          />
        )}
        {onSubmit ? (
          <button
            className="formSubmitButton editorSubmitButton"
            type="button"
            onClick={onSubmit}
          >
            {submitTitle}
          </button>
        ) : null}
      </div>
      {showsPreview ? (
        <div className="editorPreviewPane">
          <div className="editorToolbar">
            <span>Preview</span>
          </div>
          <div className="previewText">{preview}</div>
        </div>
      ) : null}
      {actions ? <div className="editorActions">{actions}</div> : null}
    </div>
  );
}

function normalizedSections<T>(items?: T[], sections?: ItemSection<T>[]) {
  return sections?.length ? sections : [{ items: items || [] }];
}

export function ListView<T>({
  items,
  sections,
  renderItem,
  empty,
  subtitle,
  isLoading,
  pagination,
}: ListViewProps<T>) {
  const visibleSections = normalizedSections(items, sections).filter(
    (section) => section.items.length > 0,
  );
  const hasItems = visibleSections.length > 0;
  return (
    <>
      {subtitle ? <div className="extensionSubtitle">{subtitle}</div> : null}
      {hasItems
        ? visibleSections.map((section, index) => (
            <div key={index} className="itemSection">
              {section.title ? (
                <div className="actionSectionHeader">{section.title}</div>
              ) : null}
              {section.subtitle ? (
                <div className="actionSectionSubtitle">{section.subtitle}</div>
              ) : null}
              {section.items.map(renderItem)}
            </div>
          ))
        : isLoading
          ? null
          : empty}
      {pagination}
    </>
  );
}

export function GridView<T>({
  items,
  sections,
  renderItem,
  empty,
  subtitle,
  layout = 'square',
  style,
  isLoading,
  pagination,
}: GridViewProps<T>) {
  const visibleSections = normalizedSections(items, sections).filter(
    (section) => section.items.length > 0,
  );
  const hasItems = visibleSections.length > 0;
  return (
    <div className="extensionView">
      {subtitle ? <div className="extensionSubtitle">{subtitle}</div> : null}
      {hasItems
        ? visibleSections.map((section, index) => (
            <div key={index} className="itemSection">
              {section.title ? (
                <div className="actionSectionHeader">{section.title}</div>
              ) : null}
              {section.subtitle ? (
                <div className="actionSectionSubtitle">{section.subtitle}</div>
              ) : null}
              <div
                className={`extensionGrid extensionGrid-${layout}`}
                style={style}
              >
                {section.items.map(renderItem)}
              </div>
            </div>
          ))
        : isLoading
          ? null
          : empty}
      {pagination}
    </div>
  );
}

export function ChatView({
  messages,
  isBusy,
  input,
  messagesRef,
  banner,
}: ChatViewProps) {
  return (
    <div className="extensionView chatView">
      {banner ? <div className="chatBanner">{banner}</div> : null}
      <div className="chatMessages" ref={messagesRef}>
        {messages.map((message, index) => (
          <div key={index} className={`chatBubble ${message.role}`}>
            {message.images?.length ? (
              <div className="chatMessageImages">
                {message.images.map((image, imageIndex) => (
                  <img
                    key={`${image.url}:${imageIndex}`}
                    src={image.url}
                    alt={image.alt || 'Attached image'}
                  />
                ))}
              </div>
            ) : null}
            {message.content || null}
          </div>
        ))}
        {isBusy ? <div className="chatBubble system">Thinking…</div> : null}
      </div>
      {input}
    </div>
  );
}

export function ActionPanelView({ rows, renderEmpty }: ActionPanelViewProps) {
  if (rows.length === 0) return <>{renderEmpty()}</>;
  return (
    <>
      {rows.map((row) =>
        row.sectionHeader ? (
          <div key={row.value} className="actionSectionHeader">
            {row.title}
          </div>
        ) : (
          <CommandRow
            key={row.value}
            value={row.value}
            icon={row.icon}
            title={row.title}
            subtitle={row.subtitle}
            shortcut={row.shortcut}
            className={row.className}
            disabled={row.disabled}
            onSelect={row.onSelect}
          />
        ),
      )}
    </>
  );
}
