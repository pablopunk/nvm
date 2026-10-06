// biome-ignore-all lint: This legacy shared UI module retains established renderer conventions.
import { Command } from 'cmdk';
import { Check, ChevronRight, Folder, Search } from 'lucide-react';
import React, { type ReactNode, useLayoutEffect, useRef } from 'react';
import ReactMarkdown, { defaultUrlTransform } from 'react-markdown';
import remarkGfm from 'remark-gfm';
import {
  formatShortcut,
  normalizeAccelerator,
  type ShortcutPlatform,
} from '../shared/shortcut-utils';
import {
  FORM_SAVE_ROW_ID,
  formFieldMatches,
  formFieldValue,
  formRowId,
  formValueLabel,
} from './form-fields';
import { MarkdownEditor } from './markdown-editor';
import type { CommandImage } from './model';
import { MorphingActivityText, MorphingIndicatorText } from './morphing-text';
import {
  type SystemTheme,
  themedImageSource,
  useSystemTheme,
} from './system-theme';

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
  prominence?: 'primary';
}
export interface CommandRowProps {
  value: string;
  icon: ReactNode;
  title: string;
  subtitle?: string;
  tooltip?: string;
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
  draggable?: boolean;
  onDragStart?: (event: React.DragEvent) => void;
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
  animateSummaryText?: boolean;
}
export type FormValue = string | boolean | string[];
export interface FormField {
  id: string;
  label?: string;
  type?: string;
  value?: FormValue;
  placeholder?: string;
  required?: boolean;
  disabled?: boolean;
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
  onEdit?: (field: FormField) => void;
  onSubmit?: () => void;
  submitTitle?: string;
  query?: string;
  errors?: Record<string, string>;
}
export interface EditorViewProps {
  className?: string;
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
    streaming?: boolean;
  }[];
  isBusy?: boolean;
  activity?: string | null;
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
  const parts = normalizeAccelerator(shortcut).split('+').filter(Boolean);
  const hyperParts = normalizeAccelerator(shortcutLabelHyperKey)
    .split('+')
    .filter(Boolean);
  const startsWithHyper =
    hyperParts.length > 0 &&
    hyperParts.every((part, index) => parts[index] === part);
  return startsWithHyper ? ['✦', ...parts.slice(hyperParts.length)] : parts;
}

export function shortcutLabel(
  shortcut?: string,
  platform: ShortcutPlatform = globalThis.window?.nvm?.platform ?? 'darwin',
) {
  const parts = shortcutLabelParts(shortcut);
  const startsWithHyper = parts[0] === '✦';
  const formattedParts = startsWithHyper ? parts.slice(1) : parts;
  const label = formatShortcut(formattedParts.join('+'), platform);
  return startsWithHyper ? `✦${label}` : label;
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

function imageProps(image: CommandImage | undefined, theme: SystemTheme) {
  if (!image) return null;
  if (typeof image === 'string')
    return { src: image, alt: '', fit: undefined, shape: undefined };
  return {
    src: themedImageSource(image, theme),
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
  tooltip,
  accessories = [],
  shortcut,
  extras,
  className,
  appearance,
  selectedOnlyShortcut = false,
  disabled,
  draggable,
  onDragStart,
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
      title={tooltip}
      data-foreground={appearance?.foreground}
      data-background={appearance?.background}
      data-prominence={appearance?.prominence}
      disabled={disabled}
      data-disabled={disabled ? 'true' : undefined}
      aria-disabled={disabled ? 'true' : undefined}
      draggable={draggable}
      onDragStart={onDragStart}
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
  const theme = useSystemTheme();
  const media = imageProps(image, theme);
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
      data-prominence={appearance?.prominence}
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
  const theme = useSystemTheme();
  const media = imageProps(image, theme);
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
  animateSummaryText = false,
}: ProgressViewProps) {
  const normalizedStatus = normalizedProgressStatus(status);
  const hasProgress =
    typeof value === 'number' && typeof total === 'number' && total > 0;
  const ratio = hasProgress ? Math.max(0, Math.min(1, value / total)) : 0;
  const percent = Math.round(ratio * 100);
  const showSummary = Boolean(label || status) || hasProgress;
  const summaryLabel = label || status || 'Working…';
  const progressLabel = `${value} of ${total} · ${percent}%`;
  const orbState = animateSummaryText ? indicatorOrbState(status) : undefined;
  return (
    <div className="extensionView progressView">
      {showSummary ? (
        <div
          className="progressOverview"
          data-status={normalizedStatus}
          data-orb={orbState ? 'true' : undefined}
          role={normalizedStatus === 'error' ? 'alert' : 'status'}
        >
          {orbState ? <IndicatorOrb state={orbState} /> : null}
          <div>
            <strong>
              {animateSummaryText ? (
                <MorphingIndicatorText value={summaryLabel} />
              ) : (
                summaryLabel
              )}
            </strong>
            {hasProgress ? (
              <small>
                {animateSummaryText ? (
                  <MorphingIndicatorText value={progressLabel} />
                ) : (
                  progressLabel
                )}
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

export function FormView({
  fields,
  values = {},
  onEdit,
  onSubmit,
  submitTitle = 'Submit',
  query = '',
  errors = {},
}: FormViewProps) {
  const filter = query.trim().toLowerCase();
  const matchingFields = fields.filter((field) => {
    if (field.type === 'description' || field.type === 'separator')
      return !filter;
    const value = formFieldValue(field, values);
    return formFieldMatches(field, value, filter);
  });
  const showSubmit =
    onSubmit && (!filter || submitTitle.toLowerCase().includes(filter));
  return (
    <div className="extensionView formView">
      {matchingFields.map((field) => {
        if (field.type === 'separator')
          return <hr key={field.id} className="formSeparator" />;
        if (field.type === 'description')
          return (
            <p key={field.id} className="formDescription">
              {field.description || field.label}
            </p>
          );
        const value = formFieldValue(field, values);
        const error =
          errors[field.id] || (value === field.value ? field.error : undefined);
        return (
          <CommandRow
            key={field.id}
            value={formRowId(field.id)}
            icon={
              field.type === 'checkbox' && value ? (
                <Check size={18} />
              ) : (
                <ChevronRight size={18} />
              )
            }
            title={field.label || field.id}
            subtitle={error || field.description}
            tooltip={error || field.description}
            accessories={[
              {
                text: formValueLabel(field, value),
                tone: error ? 'danger' : 'default',
              },
            ]}
            className={`formListRow${error ? ' formListRowError' : ''}`}
            disabled={field.disabled}
            onSelect={() => onEdit?.(field)}
          />
        );
      })}
      {showSubmit ? (
        <CommandRow
          value={FORM_SAVE_ROW_ID}
          icon={<Check size={18} />}
          title={submitTitle}
          shortcut="Command+Enter"
          className="formSaveRow"
          appearance={{ prominence: 'primary' }}
          onSelect={() => onSubmit?.()}
        />
      ) : null}
      {!matchingFields.length && !showSubmit ? (
        <EmptyState icon={<Search size={20} />} title="No matching fields" />
      ) : null}
    </div>
  );
}

export function EditorView({
  className,
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
      className={[
        'extensionView',
        'editorView',
        className,
        showsPreview && 'editorViewSplit',
        format === 'markdown' && 'editorViewMarkdown',
      ]
        .filter(Boolean)
        .join(' ')}
    >
      {title || subtitle ? (
        <header className="editorHeader">
          {title ? <span className="editorHeaderTitle">{title}</span> : null}
          {subtitle ? (
            <span className="editorHeaderSubtitle" title={subtitle}>
              {subtitle}
            </span>
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
  activity,
  input,
  messagesRef,
  banner,
}: ChatViewProps) {
  const statusRef = useRef<HTMLDivElement>(null);
  const latestMessage = messages[messages.length - 1];
  const activityIsVisible = Boolean(activity || isBusy);

  useLayoutEffect(() => {
    if (activityIsVisible) {
      statusRef.current?.scrollIntoView({ block: 'nearest' });
    }
  }, [activityIsVisible, activity, latestMessage?.content]);

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
            {message.streaming && typeof message.content === 'string' ? (
              <span className="chatStreamingText">{message.content}</span>
            ) : (
              message.content || null
            )}
          </div>
        ))}
        <div
          ref={statusRef}
          className="chatBubble system chatActivity"
          data-status={activityIsVisible ? 'active' : 'inactive'}
          role="status"
          aria-hidden={!activityIsVisible}
        >
          <MorphingActivityText value={activity || 'Thinking'} />
        </div>
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
