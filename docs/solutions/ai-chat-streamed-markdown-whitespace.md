# AI Chat Streamed Markdown Whitespace

**Status: Fixed, awaiting CI and packaged-app validation** (2026-10-31)

## Problem

A streamed AI answer showed a Markdown list as one bullet (`Apple- Banana- Orange- Mango`). Reopening the conversation rendered the same saved answer correctly.

## Root cause

Providers stream the newline between list items as its own whitespace-only delta. The renderer dropped every whitespace-only delta to keep the activity indicator mounted, so the live message lost its line breaks while the main process saved the full text.

## Contract

- Whitespace in a delta is answer content. Only the activity indicator ignores whitespace-only deltas.
- The live message must equal the saved message. Only leading whitespace before the first visible text is dropped, so no empty assistant bubble appears.
- `appendAssistantDelta` owns this rule; `ai-chat-delta.test.ts` covers it.
