import type { ModelCapability } from './criteria';
import { type ProbeReply, type ProbeTarget, sendProbe } from './probe-request';

export type ProbeOutcome = { passed: boolean; detail: string; conclusive: boolean };

const RED_SQUARE_PNG =
  'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAEAAAABACAIAAAAlC+aJAAAAS0lEQVR42u3PQQkAAAgAsetfWiP4FgYrsKZeS0BAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEDgsqnc8OJg6Ln3AAAAAElFTkSuQmCC';

const MULTIPLY_TOOL = {
  type: 'function',
  function: {
    name: 'multiply',
    description: 'Multiply two integers.',
    parameters: {
      type: 'object',
      properties: { a: { type: 'integer' }, b: { type: 'integer' } },
      required: ['a', 'b'],
    },
  },
};

const REASONING_PUZZLE =
  'A farmer has 17 sheep. All but 9 run away, then he buys 3 times as many as remain, and sells 5. How many sheep does he have now? Answer with the number only.';

function failed(reply: Extract<ProbeReply, { ok: false }>): ProbeOutcome {
  return { passed: false, detail: reply.detail, conclusive: !reply.temporary };
}

function outcome(passed: boolean, detail: string): ProbeOutcome {
  return { passed, detail, conclusive: true };
}

function reasoningObserved(reply: Extract<ProbeReply, { ok: true }>): boolean {
  return (reply.reasoningTokens ?? 0) > 0 || reply.reasoningCharacters > 0;
}

async function probeThinkingOff(target: ProbeTarget): Promise<ProbeOutcome> {
  const reply = await sendProbe(target, {
    messages: [{ role: 'user', content: REASONING_PUZZLE }],
    thinkingLevel: 'off',
    reasoning: 'route',
    maxOutputTokens: 256,
  });
  if (!reply.ok) return failed(reply);
  if (!reply.text.trim()) return outcome(false, 'empty reply with thinking off');
  if (reasoningObserved(reply))
    return outcome(false, `thought anyway: ${reply.reasoningTokens ?? 0} reasoning tokens`);
  return outcome(true, 'answered without reasoning');
}

async function probeReasoning(target: ProbeTarget): Promise<ProbeOutcome> {
  const reply = await sendProbe(target, {
    messages: [{ role: 'user', content: REASONING_PUZZLE }],
    thinkingLevel: 'medium',
    reasoning: 'route',
    maxOutputTokens: 4096,
  });
  if (!reply.ok) return failed(reply);
  if (!reasoningObserved(reply)) return outcome(false, 'no reasoning at medium thinking');
  return outcome(true, `reasoned: ${reply.reasoningTokens ?? reply.reasoningCharacters} reasoning units`);
}

function isMultiplyOfSeventeenAndTwentyThree(argumentsText: string): boolean {
  try {
    const parsed = JSON.parse(argumentsText);
    return [parsed?.a, parsed?.b].sort().join(',') === '17,23';
  } catch {
    return false;
  }
}

async function probeToolCalls(target: ProbeTarget): Promise<ProbeOutcome> {
  const reply = await sendProbe(target, {
    messages: [{ role: 'user', content: 'Use the multiply tool to compute 17 times 23.' }],
    tools: [MULTIPLY_TOOL],
    thinkingLevel: 'off',
    reasoning: 'providerDefault',
    maxOutputTokens: 4096,
  });
  if (!reply.ok) return failed(reply);
  const call = reply.toolCalls.find((toolCall) => toolCall.name === 'multiply');
  if (!call) return outcome(false, 'no multiply tool call');
  if (!isMultiplyOfSeventeenAndTwentyThree(call.arguments))
    return outcome(false, `unexpected tool arguments: ${call.arguments.slice(0, 80)}`);
  return outcome(true, 'called multiply(17, 23)');
}

async function probeStreamUsage(target: ProbeTarget): Promise<ProbeOutcome> {
  const reply = await sendProbe(target, {
    messages: [{ role: 'user', content: 'Reply with the word ok.' }],
    thinkingLevel: 'off',
    reasoning: 'providerDefault',
    maxOutputTokens: 4096,
  });
  if (!reply.ok) return failed(reply);
  if (!reply.text.trim()) return outcome(false, 'empty streamed reply');
  if (reply.promptTokens <= 0 || reply.completionTokens <= 0) return outcome(false, 'stream ended without token usage');
  return outcome(true, `usage ${reply.promptTokens}/${reply.completionTokens}`);
}

async function probeImageInput(target: ProbeTarget): Promise<ProbeOutcome> {
  const reply = await sendProbe(target, {
    messages: [
      {
        role: 'user',
        content: [
          { type: 'text', text: 'What color is this image? Answer with one word.' },
          { type: 'image_url', image_url: { url: RED_SQUARE_PNG } },
        ],
      },
    ],
    thinkingLevel: 'off',
    reasoning: 'providerDefault',
    maxOutputTokens: 4096,
  });
  if (!reply.ok) return failed(reply);
  if (!/red/i.test(reply.text)) return outcome(false, `did not see red: ${reply.text.trim().slice(0, 40)}`);
  return outcome(true, 'saw the red image');
}

const PROBES: Record<ModelCapability, (target: ProbeTarget) => Promise<ProbeOutcome>> = {
  thinking_off: probeThinkingOff,
  reasoning: probeReasoning,
  tool_calls: probeToolCalls,
  stream_usage: probeStreamUsage,
  image_input: probeImageInput,
};

export function probeCapability(target: ProbeTarget, capability: ModelCapability): Promise<ProbeOutcome> {
  return PROBES[capability](target);
}
