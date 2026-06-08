import assert from 'node:assert/strict'
import test from 'node:test'
import { buildShortcutByAiChatIdMap } from './shortcut-ownership'

test('buildShortcutByAiChatIdMap maps chats with exactly one shortcut and one generated file', () => {
  const map = buildShortcutByAiChatIdMap(
    {
      actionA: { aiChatId: 'chat-a' },
      actionB: { aiChatId: 'chat-b' },
      actionC: { aiChatId: 'chat-c' },
      actionD: { aiChatId: 'chat-c' },
    },
    { actionA: 'Cmd+1', actionB: 'Cmd+2', actionC: 'Cmd+3', actionD: 'Cmd+4' },
    {
      'chat-a': { files: ['a.ts'] },
      'chat-b': { files: ['b.ts', 'other.ts'] },
      'chat-c': { files: ['c.ts'] },
    },
    (chat) => (chat as { files?: string[] } | undefined)?.files || [],
  )

  assert.deepEqual(Array.from(map.entries()), [['chat-a', 'Cmd+1']])
})

test('buildShortcutByAiChatIdMap ignores missing shortcut bindings and missing chats', () => {
  const map = buildShortcutByAiChatIdMap(
    { actionA: { aiChatId: 'chat-a' }, actionB: { aiChatId: 'chat-b' } },
    { actionA: '' },
    {},
    () => ['file.ts'],
  )

  assert.deepEqual(Array.from(map.entries()), [])
})
