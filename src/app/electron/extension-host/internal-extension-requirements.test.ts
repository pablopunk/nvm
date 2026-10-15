import assert from 'node:assert/strict';
import test from 'node:test';
import { missingRequiredInternalCommands } from './internal-extension-requirements';

const requiredCommands = [
  { extensionId: 'nevermind.ai-builder', commandId: 'ai-chats' },
  {
    extensionId: 'nevermind.ai-commands',
    commandId: 'fix-selected-text-with-ai',
    capability: 'selected-text',
  },
] as const;

test('does not require unavailable capability-gated commands', () => {
  assert.deepEqual(
    missingRequiredInternalCommands(requiredCommands, new Map(), () => false),
    [requiredCommands[0]],
  );
});

test('still requires capability-gated commands when the host supports them', () => {
  assert.deepEqual(
    missingRequiredInternalCommands(requiredCommands, new Map(), () => true),
    requiredCommands,
  );
});

test('accepts every registered internal command', () => {
  const registeredCommands = new Map([
    ['nevermind.ai-builder:ai-chats', {}],
    ['nevermind.ai-commands:fix-selected-text-with-ai', {}],
  ]);

  assert.deepEqual(
    missingRequiredInternalCommands(
      requiredCommands,
      registeredCommands,
      () => true,
    ),
    [],
  );
});
