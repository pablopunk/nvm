#!/usr/bin/env node
// Lists every member of the frozen 1.x extension API, one path per member, for the parity ledger.
//   node src/native/scripts/parity/members-1x.cjs         writes testdata/parity/api-1x-members.json
//   node src/native/scripts/parity/members-1x.cjs --check fails when that file is out of date
const fs = require('node:fs');
const path = require('node:path');

const parity = path.resolve(__dirname, '../../testdata/parity');
const source = path.join(parity, 'nevermind-extension-api.d.ts');
const output = path.join(parity, 'api-1x-members.json');

const OPENERS = '{([<';
const CLOSERS = '})]>';

function stripComments(text) {
  return text.replace(/\/\*[\s\S]*?\*\//g, '').replace(/^\s*\/\/.*$/gm, '');
}

function isCloser(text, index) {
  return CLOSERS.includes(text[index]) && !(text[index] === '>' && text[index - 1] === '=');
}

// splitTopLevel splits text on separator where no bracket or string is open. An arrow's ">" closes nothing.
function splitTopLevel(text, separator) {
  const parts = [];
  let depth = 0;
  let quote = '';
  let current = '';
  for (let i = 0; i < text.length; i += 1) {
    const char = text[i];
    if (quote) {
      if (char === quote) quote = '';
    } else if (char === "'" || char === '"' || char === '`') {
      quote = char;
    } else if (OPENERS.includes(char)) {
      depth += 1;
    } else if (isCloser(text, i)) {
      depth -= 1;
    } else if (char === separator && depth === 0) {
      parts.push(current);
      current = '';
      continue;
    }
    current += char;
  }
  parts.push(current);
  return parts.map((part) => part.trim()).filter(Boolean);
}

function closesAtEnd(text) {
  let depth = 0;
  for (let i = 0; i < text.length; i += 1) {
    if (OPENERS.includes(text[i])) depth += 1;
    else if (isCloser(text, i)) depth -= 1;
    if (depth === 0 && i < text.length - 1) return false;
  }
  return depth === 0;
}

function wrappedBy(text, open, close) {
  return text.startsWith(open) && text.endsWith(close) && closesAtEnd(text);
}

function discriminantOf(literal) {
  for (const member of splitTopLevel(literal.slice(1, -1), ';')) {
    const match = member.match(/^(kind|type)\??\s*:\s*'([^']+)'$/);
    if (match) return `${match[1]}=${match[2]}`;
  }
  return '';
}

function walkMembers(body, prefix, add) {
  for (const raw of splitTopLevel(body, ';')) {
    const match = raw.match(/^(?:readonly\s+)?(\w+|'[^']+')\??\s*([:(<])/);
    if (!match) continue;
    const memberPath = `${prefix}.${match[1].replace(/'/g, '')}`;
    add(memberPath);
    if (match[2] === ':') walkType(raw.slice(match[0].length).trim(), memberPath, add);
  }
}

function walkUnion(parts, prefix, add) {
  for (const part of parts) {
    const literal = part.match(/^'([^']*)'$/);
    if (literal) add(`${prefix}='${literal[1]}'`);
    else if (wrappedBy(part, '{', '}')) {
      const tag = discriminantOf(part);
      walkType(part, tag ? `${prefix}#${tag}` : prefix, add);
    }
  }
}

function walkType(raw, prefix, add) {
  const text = raw.trim().replace(/^\|\s*/, '');
  const union = splitTopLevel(text, '|');
  if (union.length > 1) {
    walkUnion(union, prefix, add);
    return;
  }
  const intersection = splitTopLevel(text, '&');
  if (intersection.length > 1) {
    intersection.forEach((part) => walkType(part, prefix, add));
    return;
  }
  if (wrappedBy(text, '(', ')')) walkType(text.slice(1, -1), prefix, add);
  else if (text.startsWith('Array<') && wrappedBy(text.slice(5), '<', '>')) walkType(text.slice(6, -1), prefix, add);
  else if (wrappedBy(text, '{', '}')) walkMembers(text.slice(1, -1), prefix, add);
  else walkUnion([text], prefix, add);
}

function closingBrace(text) {
  let depth = 0;
  for (let i = 0; i < text.length; i += 1) {
    if (text[i] === '{') depth += 1;
    else if (text[i] === '}') {
      depth -= 1;
      if (depth === 0) return i;
    }
  }
  return text.length - 1;
}

function listMembers() {
  const text = stripComments(fs.readFileSync(source, 'utf8'));
  const members = new Set();
  const add = (member) => members.add(member);
  const declaration = /^export\s+(type|interface)\s+(\w+)(?:<[^=\n{]*>)?\s*(=|extends[^{]*\{|\{)/gm;
  for (let match = declaration.exec(text); match; match = declaration.exec(text)) {
    const start = match.index + match[0].length - (match[3] === '=' ? 0 : 1);
    const rest = text.slice(start);
    const body = match[1] === 'type' ? splitTopLevel(rest, ';')[0] : rest.slice(0, closingBrace(rest) + 1);
    walkType(body, match[2], add);
  }
  return [...members].sort();
}

function main() {
  const text = `${JSON.stringify(listMembers(), null, 1)}\n`;
  if (process.argv.includes('--check')) {
    if (fs.readFileSync(output, 'utf8') !== text) {
      console.error('api-1x-members.json is out of date; rerun members-1x.cjs');
      process.exit(1);
    }
    return;
  }
  fs.writeFileSync(output, text);
  console.log(`${JSON.parse(text).length} members`);
}

main();
