import assert from 'node:assert/strict';
import test from 'node:test';
import {
  EMPTY_AI_CHAT_ACTIVITY,
  MINIMUM_TOOL_ACTIVITY_MS,
  toolActivityLabel,
  transitionAiChatActivity,
} from './ai-chat-activity';

const FIRST_TOOL_STARTED_AT = 10;
const FIRST_TOOL_ENDED_AT = 20;
const DELTA_RECEIVED_AT = 100;
const SECOND_TOOL_ENDED_AT = 200;
const SECOND_TOOL_STARTED_AT = 50;
const TURN_ENDED_AT = 60;

test('moves from thinking to a tool and keeps fast tools visible', () => {
  const thinking = transitionAiChatActivity(
    EMPTY_AI_CHAT_ACTIVITY,
    { type: 'start' },
    0,
  );
  const tool = transitionAiChatActivity(
    thinking,
    { type: 'tool_start', toolName: 'web_search' },
    FIRST_TOOL_STARTED_AT,
  );
  const ended = transitionAiChatActivity(
    tool,
    { type: 'tool_end', toolName: 'web_search' },
    FIRST_TOOL_ENDED_AT,
  );

  assert.equal(thinking.label, 'Thinking');
  assert.equal(tool.label, 'Searching the web');
  assert.equal(ended.label, 'Searching the web');
  assert.equal(ended.activeToolName, null);
  assert.equal(
    ended.toolVisibleUntil,
    FIRST_TOOL_ENDED_AT + MINIMUM_TOOL_ACTIVITY_MS,
  );
});

test('does not let deltas erase a retained tool label', () => {
  const tool = transitionAiChatActivity(
    EMPTY_AI_CHAT_ACTIVITY,
    { type: 'tool_start', toolName: 'read' },
    0,
  );
  const streaming = transitionAiChatActivity(
    tool,
    { type: 'delta' },
    DELTA_RECEIVED_AT,
  );
  const ended = transitionAiChatActivity(
    streaming,
    { type: 'tool_end', toolName: 'read' },
    SECOND_TOOL_ENDED_AT,
  );
  const settled = transitionAiChatActivity(
    ended,
    { type: 'settle' },
    SECOND_TOOL_ENDED_AT + MINIMUM_TOOL_ACTIVITY_MS,
  );

  assert.equal(streaming.label, 'Reading a file');
  assert.equal(ended.label, 'Reading a file');
  assert.equal(settled.label, null);
});

test('returns to thinking after a tool when no response is streaming', () => {
  const tool = transitionAiChatActivity(
    EMPTY_AI_CHAT_ACTIVITY,
    { type: 'tool_start', toolName: 'web_search' },
    0,
  );
  const ended = transitionAiChatActivity(
    tool,
    { type: 'tool_end', toolName: 'web_search' },
    FIRST_TOOL_ENDED_AT,
  );
  const settled = transitionAiChatActivity(
    ended,
    { type: 'settle' },
    FIRST_TOOL_ENDED_AT + MINIMUM_TOOL_ACTIVITY_MS,
  );

  assert.equal(settled.label, 'Thinking');
});

test('replaces a retained tool and lets it paint before clearing', () => {
  const firstTool = transitionAiChatActivity(
    EMPTY_AI_CHAT_ACTIVITY,
    { type: 'tool_start', toolName: 'web_search' },
    0,
  );
  const secondTool = transitionAiChatActivity(
    firstTool,
    { type: 'tool_start', toolName: 'fetch_content' },
    SECOND_TOOL_STARTED_AT,
  );
  const terminal = transitionAiChatActivity(
    secondTool,
    { type: 'terminal' },
    TURN_ENDED_AT,
  );
  const settled = transitionAiChatActivity(
    terminal,
    { type: 'settle' },
    TURN_ENDED_AT + MINIMUM_TOOL_ACTIVITY_MS,
  );

  assert.equal(secondTool.label, 'Reading a page');
  assert.equal(terminal.label, 'Reading a page');
  assert.equal(
    terminal.toolVisibleUntil,
    TURN_ENDED_AT + MINIMUM_TOOL_ACTIVITY_MS,
  );
  assert.deepEqual(settled, EMPTY_AI_CHAT_ACTIVITY);
});

test('clears thinking immediately when a turn ends without a tool', () => {
  const thinking = transitionAiChatActivity(
    EMPTY_AI_CHAT_ACTIVITY,
    { type: 'start' },
    0,
  );

  assert.deepEqual(
    transitionAiChatActivity(thinking, { type: 'terminal' }, TURN_ENDED_AT),
    EMPTY_AI_CHAT_ACTIVITY,
  );
});

test('uses concise fallback labels without decorative dots', () => {
  assert.equal(toolActivityLabel('grep'), 'Searching files');
  assert.equal(toolActivityLabel('custom_tool'), 'Calling custom tool');
  assert.equal(toolActivityLabel('web_search').includes('.'), false);
  assert.equal(toolActivityLabel('web_search').includes('…'), false);
});
