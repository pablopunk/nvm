import {
  ChevronDown,
  CornerDownLeft,
  Paperclip,
  Square,
  X,
} from 'lucide-react';
import React, { useRef } from 'react';
import { canSendAiChatMessage } from '../../shared/ai-chat-images';
import type { AiChatModel } from '../../shared/ai-chat-model';
import type { AiChatAttachment } from '../ai-chat/use-ai-chat';

export function ChatInputForm({
  active = true,
  value,
  onChange,
  onResize,
  onSubmit,
  busy,
  inputRef,
  placeholder,
  onAbort,
  chatId,
  attachments = [],
  attaching = false,
  attachmentError,
  onAttachImages,
  onRemoveAttachment,
  allowImagePicker = false,
  model,
  modelChanging = false,
  onModelChange,
}: {
  active?: boolean;
  value: string;
  onChange: (value: string) => void;
  onResize: (textarea?: HTMLTextAreaElement | null) => void;
  onSubmit: () => void;
  busy: boolean;
  inputRef: React.RefObject<HTMLTextAreaElement | null>;
  placeholder?: string;
  onAbort?: (chatId?: string) => void;
  chatId?: string;
  attachments?: AiChatAttachment[];
  attaching?: boolean;
  attachmentError?: string | null;
  onAttachImages?: (files: File[]) => void;
  onRemoveAttachment?: (id: string) => void;
  allowImagePicker?: boolean;
  model?: AiChatModel;
  modelChanging?: boolean;
  onModelChange?: (model: AiChatModel) => void;
}) {
  const imageInputRef = useRef<HTMLInputElement>(null);
  return (
    <form
      className="chatInputRow"
      onSubmit={(event) => {
        event.preventDefault();
        onSubmit();
      }}
    >
      <div className="chatInputShell">
        {attachments.length ? (
          <div className="chatAttachments" aria-label="Attached images">
            {attachments.map((attachment) => (
              <div className="chatAttachment" key={attachment.id}>
                <img
                  src={attachment.previewUrl}
                  alt={attachment.name || 'Pasted image'}
                />
                <button
                  type="button"
                  aria-label={`Remove ${attachment.name || 'image'}`}
                  title="Remove image"
                  onClick={() => onRemoveAttachment?.(attachment.id)}
                >
                  <X size={12} />
                </button>
              </div>
            ))}
          </div>
        ) : null}
        {attachmentError ? (
          <div className="chatAttachmentError" role="status">
            {attachmentError}
          </div>
        ) : null}
        <textarea
          ref={inputRef}
          autoFocus={active}
          rows={1}
          value={value}
          onChange={(event) => onChange(event.target.value)}
          onInput={(event) => onResize(event.currentTarget)}
          onPaste={(event) => {
            if (!onAttachImages) return;
            const files = Array.from(event.clipboardData.items).flatMap(
              (item) => {
                const file = item.kind === 'file' ? item.getAsFile() : null;
                return file ? [file] : [];
              },
            );
            if (!files.some((file) => file.type.startsWith('image/'))) return;
            event.preventDefault();
            onAttachImages(files);
          }}
          onKeyDown={(event) => {
            if (event.key !== 'Enter') return;
            event.stopPropagation();
            if (!event.shiftKey) {
              event.preventDefault();
              onSubmit();
            }
          }}
          placeholder={
            attaching
              ? 'Attaching image…'
              : placeholder || (busy ? 'Thinking' : 'Message AI')
          }
        />
        <div className="chatComposerFooter">
          <div className="chatComposerOptions">
            {model && onModelChange ? (
              <label className="chatModelPicker" title="AI model">
                <select
                  className="chatModelSelect"
                  aria-label="AI model"
                  value={model}
                  disabled={busy || modelChanging}
                  onKeyDown={(event) => {
                    if (
                      [
                        'ArrowDown',
                        'ArrowLeft',
                        'ArrowRight',
                        'ArrowUp',
                        'Enter',
                        ' ',
                      ].includes(event.key)
                    )
                      event.stopPropagation();
                  }}
                  onChange={(event) =>
                    onModelChange(event.target.value as AiChatModel)
                  }
                >
                  <option value="fast">Fast</option>
                  <option value="smart">Smart</option>
                </select>
                <ChevronDown size={12} aria-hidden="true" />
              </label>
            ) : null}
            {allowImagePicker && onAttachImages ? (
              <>
                <input
                  ref={imageInputRef}
                  type="file"
                  accept="image/png,image/jpeg,image/webp,image/gif"
                  multiple={true}
                  hidden={true}
                  onChange={(event) => {
                    const files = Array.from(event.currentTarget.files || []);
                    event.currentTarget.value = '';
                    if (files.length) onAttachImages(files);
                    inputRef.current?.focus();
                  }}
                />
                <button
                  className="chatIconButton chatAttachButton"
                  type="button"
                  aria-label="Attach images"
                  title="Attach images"
                  disabled={modelChanging}
                  onClick={() => imageInputRef.current?.click()}
                >
                  <Paperclip size={15} aria-hidden="true" />
                </button>
              </>
            ) : null}
          </div>
          {busy ? (
            <button
              className="chatIconButton chatStopButton"
              type="button"
              aria-label="Stop"
              title="Stop"
              onClick={() => onAbort?.(chatId)}
            >
              <Square size={12} fill="currentColor" />
            </button>
          ) : (
            <button
              className="chatIconButton chatEnterButton"
              type="submit"
              aria-label="Send message"
              title="Send message"
              disabled={
                attaching ||
                modelChanging ||
                !canSendAiChatMessage(value, attachments.length)
              }
            >
              <CornerDownLeft size={15} />
            </button>
          )}
        </div>
      </div>
    </form>
  );
}
