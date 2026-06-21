import { Command } from 'cmdk';
import { Folder, Loader2 } from 'lucide-react';
import React, { type ReactNode, useEffect, useState } from 'react';
import ReactMarkdown, { defaultUrlTransform } from 'react-markdown';
import remarkGfm from 'remark-gfm';
import type { CommandImage } from './model';

export const EMPTY_ROOT_TITLE = 'Type anything';
export const EMPTY_ROOT_SUBTITLE =
  'Nevermind starts with local actions; AI planning comes next.';
export const EMPTY_RESULTS_TITLE = 'No results';
export const EMPTY_ITEMS_TITLE = 'No items';
export const EMPTY_ACTIONS_TITLE = 'No actions';
export const EMPTY_SHORTCUTS_TITLE = 'No keyboard shortcuts';

export type KeyHintsProps = {
  shortcut?: string;
  extras?: string[];
  showEnter?: boolean;
};
export type ItemAppearance = { foreground?: string };
export type CommandRowProps = {
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
  onSelect: () => void;
};
export type CommandTileProps = {
  value: string;
  title: string;
  subtitle?: string;
  image?: CommandImage;
  video?: string;
  actionHint?: ReactNode;
  appearance?: ItemAppearance;
  draggable?: boolean;
  onDragStart?: (event: React.DragEvent) => void;
  onSelect: () => void;
};
export type EmptyStateProps = {
  icon: ReactNode;
  title: string;
  subtitle?: string;
  action?: ActionPanelRow;
};
export type ToastProps = { message: string; tone?: 'default' | 'error' };
export type PreviewViewProps = {
  content?: ReactNode;
  image?: CommandImage;
  video?: string;
  poster?: string;
  actions?: ReactNode;
};
export type ProgressViewProps = {
  steps: { title: string; status?: string }[];
  value?: number;
  total?: number;
  status?: string;
};
export type FormValue = string | boolean | string[];
export type FormField = {
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
};
export type FormViewProps = {
  fields: FormField[];
  values?: Record<string, FormValue>;
  onChange?: (id: string, value: FormValue) => void;
  onSubmit?: () => void;
  submitTitle?: string;
};
export type EditorViewProps = {
  value: string;
  format?: 'text' | 'markdown';
  language?: string;
  placeholder?: string;
  readOnly?: boolean;
  preview?: ReactNode;
  actions?: ReactNode;
  submitTitle?: string;
  onChange?: (value: string) => void;
  onSubmit?: () => void;
};
export type ItemSection<T> = { title?: string; subtitle?: string; items: T[] };
export type ListViewProps<T> = {
  items?: T[];
  sections?: ItemSection<T>[];
  renderItem: (item: T) => ReactNode;
  empty?: ReactNode;
  subtitle?: string;
  isLoading?: boolean;
  pagination?: ReactNode;
};
export type GridViewProps<T> = {
  items?: T[];
  sections?: ItemSection<T>[];
  renderItem: (item: T) => ReactNode;
  empty?: ReactNode;
  subtitle?: string;
  layout?: string;
  style?: React.CSSProperties;
  isLoading?: boolean;
  pagination?: ReactNode;
};
export type ChatViewProps = {
  messages: { role: string; content: ReactNode }[];
  isBusy?: boolean;
  input?: ReactNode;
  messagesRef?: React.RefObject<HTMLDivElement | null>;
  banner?: ReactNode;
};
export type ActionPanelRow = {
  value: string;
  icon?: ReactNode;
  title: string;
  subtitle?: string;
  shortcut?: string;
  className?: string;
  sectionHeader?: boolean;
  onSelect: () => void;
};
export type ActionPanelViewProps = {
  rows: ActionPanelRow[];
  renderEmpty: () => ReactNode;
};
export type SearchAccessoryProps = {
  tooltip?: string;
  value?: string;
  items: { title: string; value: string }[];
  onChange?: (value: string) => void;
};
export type MarkdownContentProps = { content: string };

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
      onSelect={onSelect}
    >
      <span className="resultIcon">{icon}</span>
      <span className="resultText">
        <strong>{title}</strong>
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
  image,
  video,
  actionHint,
  appearance,
  draggable,
  onDragStart,
  onSelect,
}: CommandTileProps) {
  const media = imageProps(image);
  const visual = media?.src ? (
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
        {actionHint}
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
  return (
    <div
      className={`toast ${tone === 'error' ? 'toastError' : ''}`}
      role="status"
      aria-live={tone === 'error' ? 'assertive' : 'polite'}
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
  status,
}: ProgressViewProps) {
  const hasProgress =
    typeof value === 'number' && typeof total === 'number' && total > 0;
  const ratio = hasProgress ? Math.max(0, Math.min(1, value / total)) : 0;
  const percent = Math.round(ratio * 100);
  const showSummary = Boolean(status) || hasProgress;
  return (
    <div className="extensionView progressView">
      {showSummary ? (
        <div className="progressOverview">
          <div>
            <strong>{status || 'Working…'}</strong>
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

function formFieldControl(
  field: FormField,
  value: FormValue,
  onChange?: FormViewProps['onChange'],
) {
  const type = field.type || 'text';
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
        value={String(value)}
        placeholder={field.placeholder}
        required={field.required}
        rows={field.rows || 4}
        onChange={(event) => onChange?.(field.id, event.currentTarget.value)}
      />
    );
  if (type === 'checkbox')
    return (
      <label className="formCheckbox">
        <input
          checked={Boolean(value)}
          required={field.required}
          type="checkbox"
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
        value={String(value)}
        required={field.required}
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
      <select
        multiple={true}
        value={selected}
        required={field.required}
        onChange={(event) =>
          onChange?.(
            field.id,
            Array.from(event.currentTarget.selectedOptions).map(
              (option) => option.value,
            ),
          )
        }
      >
        {(field.options || []).map((option) => (
          <option key={option.value} value={option.value}>
            {option.title}
          </option>
        ))}
      </select>
    );
  }
  return (
    <input
      value={String(value)}
      placeholder={field.placeholder}
      required={field.required}
      type={type}
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
}: FormViewProps) {
  return (
    <form
      className="extensionView formView"
      onSubmit={(event) => {
        event.preventDefault();
        onSubmit?.();
      }}
    >
      {fields.map((field) => {
        const type = field.type || 'text';
        const value = normalizedFormValue(values[field.id] ?? field.value);
        if (type === 'description' || type === 'separator')
          return (
            <div
              key={field.id}
              className={`formStaticField formStaticField-${type}`}
            >
              {formFieldControl(field, value, onChange)}
            </div>
          );
        if (type === 'checkbox')
          return (
            <div key={field.id} className="formField formField-checkbox">
              {formFieldControl(field, value, onChange)}
              {field.description ? <small>{field.description}</small> : null}
              {field.error ? (
                <small className="formFieldError">{field.error}</small>
              ) : null}
            </div>
          );
        if (type === 'file' || type === 'files' || type === 'folder')
          return (
            <div key={field.id} className="formField">
              <span>{field.label}</span>
              {formFieldControl(field, value, onChange)}
              {field.description ? <small>{field.description}</small> : null}
              {field.error ? (
                <small className="formFieldError">{field.error}</small>
              ) : null}
            </div>
          );
        return (
          <label key={field.id} className="formField">
            <span>{field.label}</span>
            {formFieldControl(field, value, onChange)}
            {field.description ? <small>{field.description}</small> : null}
            {field.error ? (
              <small className="formFieldError">{field.error}</small>
            ) : null}
          </label>
        );
      })}
      {onSubmit ? (
        <button className="formSubmitButton" type="submit">
          {submitTitle}
        </button>
      ) : null}
    </form>
  );
}

export function EditorView({
  value,
  format = 'text',
  language,
  placeholder,
  readOnly,
  preview,
  actions,
  submitTitle = 'Save',
  onChange,
  onSubmit,
}: EditorViewProps) {
  const showsPreview = format === 'markdown' && Boolean(preview);
  return (
    <div
      className={`extensionView editorView ${showsPreview ? 'editorViewSplit' : ''}`}
    >
      <div className="editorPane">
        <div className="editorToolbar">
          <span>{format === 'markdown' ? 'Markdown' : 'Plain text'}</span>
          {language ? <small>{language}</small> : null}
        </div>
        <textarea
          className="editorTextarea"
          value={value}
          placeholder={placeholder}
          readOnly={readOnly}
          spellCheck={format !== 'markdown'}
          onKeyDown={(event) => {
            if (event.key === 'Escape') return;
            if (event.metaKey || event.ctrlKey || event.altKey) return;
            event.stopPropagation();
          }}
          onChange={(event) => onChange?.(event.currentTarget.value)}
        />
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

function LoadingSpinner({ delayMs = 200 }: { delayMs?: number }) {
  const [visible, setVisible] = useState(false);
  useEffect(() => {
    const timer = setTimeout(() => setVisible(true), delayMs);
    return () => clearTimeout(timer);
  }, [delayMs]);
  if (!visible) return null;
  return (
    <div className="loadingSpinner">
      <Loader2 size={20} className="spinnerIcon" />
    </div>
  );
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
      {isLoading && hasItems && <div className="viewLoadingBar" />}

      {hasItems ? (
        visibleSections.map((section, index) => (
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
      ) : isLoading ? (
        <LoadingSpinner />
      ) : (
        empty
      )}
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
      {isLoading && hasItems && <div className="viewLoadingBar" />}

      {hasItems ? (
        visibleSections.map((section, index) => (
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
      ) : isLoading ? (
        <LoadingSpinner />
      ) : (
        empty
      )}
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
            {message.content}
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
            onSelect={row.onSelect}
          />
        ),
      )}
    </>
  );
}
