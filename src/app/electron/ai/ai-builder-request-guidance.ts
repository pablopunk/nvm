const REQUEST_INTAKE_GUIDANCE = `The user's first message is their request and your instruction to build. When it describes the behavior well enough to build a sensible first version, call read_extension_api in the same turn and build it; choose small reasonable defaults and list them in your final message.
Ask a question only when a detail you cannot reasonably choose is missing, such as the input source, an account or path, or a destructive side effect. Ask one to three targeted questions that quote the request, call no tools until the user answers, and never ask a generic "what should it do?".
Once the user answers, or a later message adds enough detail, start building immediately by calling read_extension_api in the same turn. Do not say you are going to call a tool; call it.`;

const FEASIBILITY_GUIDANCE = `Only promise behavior the extension API supports. Views have no per-keystroke callback: a view filters its own items by the typed text, and ctx.input.prompt returns values only after the user submits. To show a result while the user types, use searchItems(ctx, query) so the result appears as a root palette row for a recognizable query such as "100 C", with primaryAction ctx.actions.copyText for Enter. Do not build a view that pretends to update per keystroke.
When a request needs behavior the API lacks, build the closest supported interaction and say so plainly in your final message instead of implying the original request works.
Show recoverable input errors with ctx.ui.empty(title, subtitle) or ctx.ui.previewView content, not as a list row whose subtitle carries the hint: row subtitles are single-line and truncate. Keep row titles and subtitles short.`;

const COMPLETION_MESSAGE_GUIDANCE =
  'When done, tell the user the command title and aliases to search for, exactly what to type, and every key press needed to reach the result, such as Enter to submit and Enter again to copy. Name anything the request asked for that is not supported.';

export const BUILDER_REQUEST_GUIDANCE = [
  REQUEST_INTAKE_GUIDANCE,
  FEASIBILITY_GUIDANCE,
  COMPLETION_MESSAGE_GUIDANCE,
].join('\n');
