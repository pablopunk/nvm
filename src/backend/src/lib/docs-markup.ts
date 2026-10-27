function escapeHtml(text: string) {
  return text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}

function renderInline(text: string) {
  return escapeHtml(text)
    .replace(/\{@link\s+([^}\s]+)[^}]*\}/g, '<code>$1</code>')
    .replace(/`([^`]+)`/g, '<code>$1</code>')
    .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
}

/**
 * Renders the small Markdown subset TSDoc comments use: paragraphs, bullet
 * lists, inline code, bold, and fenced code. Input is escaped before any markup
 * is added, so comment text can never inject HTML.
 */
export function renderDocMarkup(markdown: string) {
  const blocks: string[] = [];
  const fenced = markdown.split(/```[a-z]*\n?/);
  fenced.forEach(function renderSegment(segment, index) {
    if (index % 2 === 1) {
      blocks.push(`<pre><code>${escapeHtml(segment.trimEnd())}</code></pre>`);
      return;
    }
    for (const paragraph of segment.split(/\n{2,}/)) {
      const trimmed = paragraph.trim();
      if (!trimmed) continue;
      const lines = trimmed.split('\n');
      if (lines.every((line) => /^\s*[-*]\s+/.test(line))) {
        const items = lines.map(
          (line) => `<li>${renderInline(line.replace(/^\s*[-*]\s+/, ''))}</li>`,
        );
        blocks.push(`<ul>${items.join('')}</ul>`);
      } else {
        blocks.push(`<p>${renderInline(lines.join(' '))}</p>`);
      }
    }
  });
  return blocks.join('');
}

/** First sentence of a doc comment, for compact summaries. */
export function summarizeDoc(markdown: string) {
  const firstParagraph = markdown.split(/\n{2,}/)[0] ?? '';
  const sentence = firstParagraph.replace(/\s+/g, ' ').match(/^.*?[.!?](\s|$)/);
  return (sentence?.[0] ?? firstParagraph).trim();
}
