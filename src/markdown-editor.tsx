import { CodeHighlightNode, CodeNode } from '@lexical/code';
import { AutoLinkNode, LinkNode } from '@lexical/link';
import { ListItemNode, ListNode } from '@lexical/list';
import {
  $convertFromMarkdownString,
  $convertToMarkdownString,
  TRANSFORMERS,
} from '@lexical/markdown';
import { AutoFocusPlugin } from '@lexical/react/LexicalAutoFocusPlugin';
import { CheckListPlugin } from '@lexical/react/LexicalCheckListPlugin';
import { ContentEditable } from '@lexical/react/LexicalContentEditable';
import { HistoryPlugin } from '@lexical/react/LexicalHistoryPlugin';
import { LexicalComposer } from '@lexical/react/LexicalComposer';
import { useLexicalComposerContext } from '@lexical/react/LexicalComposerContext';
import { LexicalErrorBoundary } from '@lexical/react/LexicalErrorBoundary';
import { ListPlugin } from '@lexical/react/LexicalListPlugin';
import { MarkdownShortcutPlugin } from '@lexical/react/LexicalMarkdownShortcutPlugin';
import { OnChangePlugin } from '@lexical/react/LexicalOnChangePlugin';
import { RichTextPlugin } from '@lexical/react/LexicalRichTextPlugin';
import { TabIndentationPlugin } from '@lexical/react/LexicalTabIndentationPlugin';
import { HeadingNode, QuoteNode } from '@lexical/rich-text';
import type { EditorState } from 'lexical';
import { useEffect, useRef } from 'react';

const markdownEditorTheme = {
  code: 'markdownEditorCodeBlock',
  heading: {
    h1: 'markdownEditorHeading markdownEditorHeading1',
    h2: 'markdownEditorHeading markdownEditorHeading2',
    h3: 'markdownEditorHeading markdownEditorHeading3',
    h4: 'markdownEditorHeading markdownEditorHeading4',
    h5: 'markdownEditorHeading markdownEditorHeading5',
    h6: 'markdownEditorHeading markdownEditorHeading6',
  },
  link: 'markdownEditorLink',
  list: {
    checklist: 'markdownEditorChecklist',
    listitem: 'markdownEditorListItem',
    listitemChecked: 'markdownEditorListItemChecked',
    listitemUnchecked: 'markdownEditorListItemUnchecked',
    nested: { listitem: 'markdownEditorNestedListItem' },
    ol: 'markdownEditorOrderedList',
    ul: 'markdownEditorUnorderedList',
  },
  paragraph: 'markdownEditorParagraph',
  quote: 'markdownEditorQuote',
  text: {
    bold: 'markdownEditorBold',
    code: 'markdownEditorInlineCode',
    italic: 'markdownEditorItalic',
    strikethrough: 'markdownEditorStrikethrough',
    underline: 'markdownEditorUnderline',
  },
};

function MarkdownValuePlugin({
  value,
  onChange,
  onFlush,
}: {
  value: string;
  onChange?: (value: string) => void;
  onFlush?: (value: string) => void;
}) {
  const [editor] = useLexicalComposerContext();
  const currentMarkdownRef = useRef(value);
  const pendingEditorStateRef = useRef<EditorState | null>(null);
  const exportTimerRef = useRef<number | null>(null);

  useEffect(() => {
    if (value === currentMarkdownRef.current) return;
    currentMarkdownRef.current = value;
    editor.update(() => $convertFromMarkdownString(value, TRANSFORMERS));
  }, [editor, value]);

  function emitMarkdown(editorState: EditorState, flush = false) {
    editorState.read(() => {
      const markdown = $convertToMarkdownString(TRANSFORMERS);
      if (markdown === currentMarkdownRef.current) return;
      currentMarkdownRef.current = markdown;
      if (flush) onFlush?.(markdown);
      else onChange?.(markdown);
    });
  }

  function flushMarkdown(persist = false) {
    if (exportTimerRef.current !== null)
      window.clearTimeout(exportTimerRef.current);
    exportTimerRef.current = null;
    const editorState = pendingEditorStateRef.current;
    pendingEditorStateRef.current = null;
    if (editorState) emitMarkdown(editorState, persist);
  }

  function scheduleMarkdown(editorState: EditorState) {
    pendingEditorStateRef.current = editorState;
    if (exportTimerRef.current !== null)
      window.clearTimeout(exportTimerRef.current);
    exportTimerRef.current = window.setTimeout(flushMarkdown, 80);
  }

  useEffect(
    () => () => {
      flushMarkdown(true);
    },
    [],
  );

  return (
    <OnChangePlugin ignoreSelectionChange={true} onChange={scheduleMarkdown} />
  );
}

export function MarkdownEditor({
  value,
  placeholder,
  readOnly,
  autoFocus,
  onChange,
  onFlush,
}: {
  value: string;
  placeholder?: string;
  readOnly?: boolean;
  autoFocus?: boolean;
  onChange?: (value: string) => void;
  onFlush?: (value: string) => void;
}) {
  return (
    <LexicalComposer
      initialConfig={{
        namespace: 'NevermindMarkdownEditor',
        editable: !readOnly,
        editorState: () => $convertFromMarkdownString(value, TRANSFORMERS),
        nodes: [
          AutoLinkNode,
          CodeHighlightNode,
          CodeNode,
          HeadingNode,
          LinkNode,
          ListItemNode,
          ListNode,
          QuoteNode,
        ],
        onError: (error) => {
          throw error;
        },
        theme: markdownEditorTheme,
      }}
    >
      <div className="markdownEditor">
        <RichTextPlugin
          contentEditable={
            <ContentEditable
              className="markdownEditorContent"
              aria-placeholder={placeholder || ''}
              placeholder={
                <div className="markdownEditorPlaceholder">{placeholder}</div>
              }
              spellCheck={true}
              onKeyDown={(event) => {
                if (event.key === 'Escape') return;
                if (event.metaKey || event.ctrlKey || event.altKey) return;
                event.stopPropagation();
              }}
            />
          }
          ErrorBoundary={LexicalErrorBoundary}
        />
        <HistoryPlugin />
        <ListPlugin />
        <CheckListPlugin />
        <TabIndentationPlugin />
        <MarkdownShortcutPlugin transformers={TRANSFORMERS} />
        <MarkdownValuePlugin
          value={value}
          onChange={onChange}
          onFlush={onFlush}
        />
        {autoFocus ? <AutoFocusPlugin /> : null}
      </div>
    </LexicalComposer>
  );
}
