const MINIMUM_TOOL_ACTIVITY_MS = 1800;

interface AiChatActivityState {
  label: string | null;
  activeToolName: string | null;
  toolVisibleUntil: number | null;
  labelAfterTool: string | null;
}

type AiChatActivityEvent =
  | { type: 'start' }
  | { type: 'delta' }
  | { type: 'tool_start'; toolName: string }
  | { type: 'tool_end'; toolName?: string }
  | { type: 'settle' }
  | { type: 'terminal' };

const EMPTY_AI_CHAT_ACTIVITY: AiChatActivityState = {
  label: null,
  activeToolName: null,
  toolVisibleUntil: null,
  labelAfterTool: null,
};

const TOOL_ACTIVITY_LABELS = new Map([
  ['web_search', 'Searching the web'],
  ['code_search', 'Searching code'],
  ['fetch_content', 'Reading a page'],
  ['get_search_content', 'Reading a search result'],
  ['read', 'Reading a file'],
  ['grep', 'Searching files'],
  ['find', 'Finding files'],
  ['ls', 'Listing files'],
  ['read_extension_api', 'Reading the extension API'],
  ['list_capabilities', 'Listing capabilities'],
  ['list_shortcuts', 'Listing shortcuts'],
  ['read_app_logs', 'Reading app logs'],
  ['list_extensions', 'Listing extensions'],
  ['read_extension', 'Reading an extension'],
  ['read_current_extension', 'Reading the current extension'],
  ['write_extension', 'Writing the extension'],
  ['remove_extension', 'Removing the extension'],
  ['validate_extension', 'Validating the extension'],
  ['install_extension', 'Installing the extension'],
]);

function toolActivityLabel(toolName: string) {
  const label = TOOL_ACTIVITY_LABELS.get(toolName);
  if (label) {
    return label;
  }
  const readableName = toolName.replaceAll('_', ' ').trim() || 'tool';
  return `Using ${readableName}`;
}

function aiChatDeltaHasVisibleText(text?: string) {
  return Boolean(text?.trim());
}

function toolStarted(toolName: string, now: number): AiChatActivityState {
  return {
    label: toolActivityLabel(toolName),
    activeToolName: toolName,
    toolVisibleUntil: now + MINIMUM_TOOL_ACTIVITY_MS,
    labelAfterTool: 'Thinking',
  };
}

function deltaReceived(state: AiChatActivityState, now: number) {
  if (state.activeToolName || (state.toolVisibleUntil || 0) > now) {
    return { ...state, labelAfterTool: null };
  }
  return EMPTY_AI_CHAT_ACTIVITY;
}

function toolEnded(
  state: AiChatActivityState,
  toolName: string | undefined,
  now: number,
) {
  if (!state.activeToolName) {
    return state;
  }
  if (toolName && toolName !== state.activeToolName) {
    return state;
  }
  return {
    ...state,
    activeToolName: null,
    toolVisibleUntil: now + MINIMUM_TOOL_ACTIVITY_MS,
  };
}

function settledActivity(state: AiChatActivityState, now: number) {
  if (state.activeToolName || (state.toolVisibleUntil || 0) > now) {
    return state;
  }
  return { ...EMPTY_AI_CHAT_ACTIVITY, label: state.labelAfterTool };
}

function transitionAiChatActivity(
  state: AiChatActivityState,
  event: AiChatActivityEvent,
  now: number,
): AiChatActivityState {
  switch (event.type) {
    case 'terminal':
      if (state.activeToolName || state.toolVisibleUntil) {
        return {
          ...state,
          activeToolName: null,
          toolVisibleUntil: Math.max(
            state.toolVisibleUntil || 0,
            now + MINIMUM_TOOL_ACTIVITY_MS,
          ),
          labelAfterTool: null,
        };
      }
      return EMPTY_AI_CHAT_ACTIVITY;
    case 'start':
      return { ...EMPTY_AI_CHAT_ACTIVITY, label: 'Thinking' };
    case 'tool_start':
      return toolStarted(event.toolName, now);
    case 'delta':
      return deltaReceived(state, now);
    case 'tool_end':
      return toolEnded(state, event.toolName, now);
    case 'settle':
      return settledActivity(state, now);
    default:
      return state;
  }
}

export type { AiChatActivityEvent, AiChatActivityState };
export {
  aiChatDeltaHasVisibleText,
  EMPTY_AI_CHAT_ACTIVITY,
  MINIMUM_TOOL_ACTIVITY_MS,
  toolActivityLabel,
  transitionAiChatActivity,
};
