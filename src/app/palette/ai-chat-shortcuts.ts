interface ConversationShortcutInput {
  key: string;
  query: string;
  isChildOpen: boolean;
  altKey: boolean;
  ctrlKey: boolean;
  metaKey: boolean;
  shiftKey: boolean;
}

export function shouldStartConversationFromTab(
  input: ConversationShortcutInput,
) {
  return (
    !input.isChildOpen &&
    input.key === 'Tab' &&
    !(input.altKey || input.ctrlKey || input.metaKey || input.shiftKey) &&
    Boolean(input.query.trim())
  );
}
