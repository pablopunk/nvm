#!/usr/bin/env node
// Builds testdata/parity/api-ledger.json: one row for every 1.x API member saying where it went in the Go API.
// Rows resolve by name, then testdata/parity/api-ledger-decisions.json overrides them with hand-made decisions.
//   node src/native/scripts/parity/ledger-1x.cjs
const fs = require('node:fs');
const path = require('node:path');

const native = path.resolve(__dirname, '../..');
const apiDir = path.join(native, 'internal/ext/api');
const parity = path.join(native, 'testdata/parity');

// The Go type each 1.x exported type became, where dropping the Extension prefix is not enough.
const TYPE_NAMES = {
  ExtensionContext: 'Context',
  NevermindExtension: 'Manifest',
  ExtensionFile: 'FileRef',
  ExtensionFormField: 'Field',
  ExtensionFormValue: 'FormValue',
  ExtensionActionSection: 'ActionSection',
  ExtensionActionPanel: 'ActionPanel',
  ExtensionActionContribution: 'Command',
  ExtensionCrudCollection: 'CollectionSpec',
  ExtensionBackgroundTrigger: 'Trigger',
  ExtensionFindFilesOptions: 'FindOptions',
  ExtensionFileIndexOptions: 'IndexOptions',
  ExtensionIndexedFile: 'IndexedFile',
  ExtensionShellOptions: 'ShellOptions',
  ExtensionShellResult: 'ShellResult',
  ExtensionTypeTextOptions: 'TypeOptions',
  ExtensionPasteTextOptions: 'PasteOptions',
  ExtensionPasteOptions: 'PasteOptions',
  ExtensionClipboardWriteOptions: 'ClipboardContent',
  ExtensionClipboardEntry: 'ClipboardEntry',
  ExtensionClipboardHistory: 'ClipboardHistory',
  ExtensionAiOptions: 'AIOptions',
  ExtensionAi: 'AI',
  ExtensionAiBuilder: 'AIBuilder',
  ExtensionDecisionOptions: 'DecisionRequest',
  ExtensionOcrOptions: 'OCROptions',
  ExtensionOcrResult: 'OCRResult',
  ExtensionOcrBlock: 'OCRBlock',
  ExtensionOcrRegion: 'Rect',
  ExtensionRuntimeCache: 'Cache',
  ExtensionTemplateOptions: 'TemplateOptions',
  ExtensionTemplateResult: 'TemplateResult',
  ExtensionWindowOptions: 'WindowOptions',
  ExtensionWindowInfo: 'WindowInfo',
  ExtensionDisplayInfo: 'DisplayInfo',
  ExtensionMenuBarRow: 'MenuRow',
  ExtensionMenuBar: 'MenuBar',
  ExtensionPreviewItemDescriptor: 'PreviewItem',
  LogEntry: 'LogEntry',
};

// The Go service behind each namespace of ExtensionContext.
const CONTEXT_NAMESPACES = {
  'desktop.keyboard': 'Keyboard',
  'desktop.clipboard': 'SystemClipboard',
  'desktop.selection': 'Selection',
  'desktop.apps': 'Apps',
  'desktop.menus': 'Menus',
  'desktop.windowManagement': 'WindowManagement',
  'desktop.files': 'Files',
  'desktop.shell': 'Shell',
  desktop: 'Desktop',
  windows: 'Windows',
  'clipboard.history': 'ClipboardHistory',
  ocr: 'OCR',
  navigation: 'Navigation',
  input: 'Input',
  drafts: 'Drafts',
  shortcuts: 'Shortcuts',
  views: 'Views',
  updates: 'Updates',
  storage: 'Storage',
  secrets: 'Secrets',
  settings: 'Settings',
  dictation: 'Dictation',
  logs: 'Logs',
  cache: 'Cache',
  data: 'Data',
  ai: 'AI',
  aiBuilder: 'AIBuilder',
  system: 'System',
  text: 'Text',
  'extensions.ownership': 'Ownership',
};

// Go types that also hold members of a 1.x type, because 2.x moved them into kind-specific or nested types.
const MOVED_TO = {
  ExtensionView: ['GridSpec', 'EditorSpec', 'FormSpec', 'ProgressSpec', 'CameraSpec', 'WebviewSpec', 'ChatSpec', 'PreviewSpec', 'Refresh', 'DetailPane', 'ChatMessage', 'CollectionSpec'],
  ExtensionAction: ['Confirmation', 'ActionRef', 'DraftConflict', 'Invocation', 'FormSpec', 'PromptSpec'],
  ExtensionActionContribution: ['Action', 'Item'],
  ExtensionCommand: ['Action', 'Item'],
  ExtensionItem: ['Action', 'Command', 'FileRef'],
  ExtensionCrudCollection: ['View', 'CrudItem', 'EmptyState'],
  ExtensionWindowLayoutSpec: ['WindowLayout'],
  ExtensionWindowApplyResult: ['ApplyResult', 'Rect'],
  ExtensionDisplayInfo: ['DisplayInfo', 'Rect'],
  ExtensionWindowInfo: ['WindowInfo', 'Rect'],
  ExtensionOcrBlock: ['OCRBlock', 'Rect'],
  ExtensionAiAttachment: ['Attachment'],
  ExtensionAiStreamEvent: ['AIStreamEvent'],
  ExtensionAiStream: ['AIStream'],
  ExtensionAi: ['AttachmentBuilder'],
  ExtensionMetadataItem: ['Metadata'],
  ExtensionSearchAccessory: ['ViewFilter', 'ViewFilterOption'],
  ExtensionDetailLink: ['LinkCard'],
  ExtensionEditorDraftOptions: ['DraftOptions'],
  ExtensionFormOption: ['Option'],
  ExtensionItemAccessory: ['Accessory'],
  ExtensionItemAppearance: ['Appearance'],
  ExtensionPaletteShortcut: ['ShortcutRecord'],
  ExtensionShortcutRecord: ['ShortcutRecord'],
  ExtensionMenuBar: ['Popover', 'MenuSection'],
  ExtensionMenuBarContribution: ['MenuBar'],
  ExtensionWindowManagementOptions: ['WindowManagementOptions', 'Insets'],
  ExtensionPreviewItemDescriptor: ['FilePreview', 'PreviewSpec', 'FileRef', 'Item'],
  ExtensionWindowRestoreDescriptor: ['WindowRestore'],
  ExtensionActionResult: ['Result'],
  RecentLogOptions: ['LogQuery'],
  NevermindExtension: ['Manifest', 'Identity'],
  ExtensionDraftResolutionFollowUp: ['DraftResolution', 'Result'],
  ExtensionDecisionAnswer: ['DecisionResult'],
  ExtensionSystem: ['SystemLabels'],
  ExtensionFindFilesOptions: ['FindOptions'],
  ExtensionPasteTextOptions: ['PasteOptions', 'Action'],
};

// Ways 2.x spells a 1.x member name: milliseconds suffixes, dropped "is" prefixes, Url and Id endings.
function spellings(name) {
  const stem = name.replace(/^is(?=[A-Z])/, '');
  return [name, stem, `${name}Ms`, `${name}At`, `${name}AtMs`, name.replace(/At$/, 'AtMs'), name.replace(/At$/, 'Ms'), name.replace(/Id$/, 'ID'), name.replace(/Url$/, ''), name.replace(/Path$/, '')];
}

// Files outside the API package that own the values of 1.x string unions.
const VALUE_SOURCES = ['internal/windowmgmt/failure.go', 'internal/platform/capabilities.go'];

function hasValue(corpus, value) {
  const escaped = value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  return new RegExp(`(^|[^\\w-])${escaped}($|[^\\w-])`, 'm').test(corpus);
}

function indexGoApi() {
  const types = new Map();
  const functions = new Set();
  const constants = new Set();
  let corpus = '';
  for (const name of fs.readdirSync(apiDir)) {
    if (!name.endsWith('.go') || name.endsWith('_test.go')) continue;
    const text = fs.readFileSync(path.join(apiDir, name), 'utf8');
    for (const match of text.matchAll(/^type (\w+) (struct|interface) \{([\s\S]*?)^\}/gm)) {
      types.set(match[1], membersOf(match[3]));
    }
    for (const match of text.matchAll(/^type ([A-Z]\w*) (?!struct|interface)\S/gm)) types.set(match[1], new Map());
    for (const match of text.matchAll(/^func ([A-Z]\w*)\(/gm)) functions.add(match[1]);
    for (const match of text.matchAll(/^\s*([A-Z]\w*)\s+\w+\s*=\s*"/gm)) constants.add(match[1]);
    corpus += text;
  }
  for (const extra of VALUE_SOURCES) corpus += fs.readFileSync(path.join(native, extra), 'utf8');
  return { types, functions, constants, corpus };
}

function membersOf(body) {
  const members = new Map();
  for (const line of body.split('\n')) {
    const trimmed = line.trim();
    if (trimmed.startsWith('//')) continue;
    const declared = trimmed.match(/^([A-Z]\w*)\b/);
    if (!declared) continue;
    members.set(declared[1].toLowerCase(), declared[1]);
    const tag = trimmed.match(/json:"(\w+)/);
    if (tag) members.set(tag[1].toLowerCase(), declared[1]);
  }
  return members;
}

function goTypeOf(root, types) {
  const direct = TYPE_NAMES[root] || root.replace(/^Extension/, '');
  return types.has(direct) ? direct : '';
}

function resolveInType(goType, name, types) {
  const members = types.get(goType);
  if (!members) return null;
  for (const spelling of spellings(name)) {
    const goName = members.get(spelling.toLowerCase());
    if (goName) return { target: `${goType}.${goName}`, renamed: goName.toLowerCase() !== name.toLowerCase() };
  }
  return null;
}

function resolveInMoved(root, name, types) {
  for (const goType of MOVED_TO[root] || []) {
    const found = resolveInType(goType, name, types);
    if (found) return { ...found, renamed: true };
  }
  return null;
}

function resolveContext(segments, index) {
  const joined = segments.join('.');
  if (CONTEXT_NAMESPACES[joined]) return { target: CONTEXT_NAMESPACES[joined], renamed: false };
  const parent = segments.slice(0, -1).join('.');
  const service = CONTEXT_NAMESPACES[parent];
  return service ? resolveInType(service, segments[segments.length - 1], index.types) : null;
}

function resolveValue(member, index) {
  const value = member.match(/='([^']*)'$/)[1];
  return hasValue(index.corpus, value) ? { target: `value:${value}`, renamed: false } : null;
}

function resolve(member, index) {
  if (member.endsWith("'")) return resolveValue(member, index);
  const segments = member.replace(/#[^.]+/, '').split('.');
  const root = segments.shift();
  if (root === 'ExtensionContext') return resolveContext(segments, index);
  const goType = goTypeOf(root, index.types);
  const name = segments[segments.length - 1];
  const direct = goType ? resolveInType(goType, name, index.types) : null;
  return direct || resolveInMoved(root, name, index.types);
}

function buildRow(member, index, decisions) {
  const decided = decisions[member];
  if (decided) return { member, verified: 'decision', ...decided };
  const found = resolve(member, index);
  if (!found) return { member, status: 'open', target: '', verified: 'none' };
  return { member, status: found.renamed ? 'renamed' : 'same', target: found.target, verified: 'name' };
}

function targetExists(target, index) {
  if (!target) return false;
  const [kind, name] = target.includes(':') ? target.split(/:(.*)/) : ['', target];
  if (kind === 'value') return hasValue(index.corpus, name);
  if (kind === 'func') return index.functions.has(name);
  if (kind === 'const') return index.constants.has(name);
  const [type, member] = name.split('.');
  if (!index.types.has(type)) return false;
  return !member || [...index.types.get(type).values()].includes(member);
}

// migrationText is what the builder reads to convert a 1.x extension: every 1.x name that is not the same in Go.
function migrationText(rows) {
  const lines = ['# 1.x extension API names and their Go replacements', '', 'Names not listed keep their name in Go.', ''];
  for (const row of rows) {
    if (row.status === 'same' || row.status === 'open') continue;
    const name = row.member.replace(/^Extension(?=[A-Z])/, '').replace(/^Context\./, 'ctx.');
    const target = row.status === 'dropped' ? 'gone' : row.target;
    lines.push(`- ${name} -> ${target}${row.note ? ` (${row.note})` : ''}`);
  }
  return `${lines.join('\n')}\n`;
}

function readJSON(file, fallback) {
  return fs.existsSync(file) ? JSON.parse(fs.readFileSync(file, 'utf8')) : fallback;
}

function main() {
  const members = readJSON(path.join(parity, 'api-1x-members.json'), []);
  const decisions = readJSON(path.join(parity, 'api-ledger-decisions.json'), {});
  const index = indexGoApi();
  const rows = members.map((member) => buildRow(member, index, decisions));
  fs.writeFileSync(path.join(parity, 'api-ledger.json'), `${JSON.stringify(rows, null, 1)}\n`);
  fs.writeFileSync(path.join(native, 'internal/ai/builder/migration_1x.txt'), migrationText(rows));
  for (const row of rows) {
    if (row.status !== 'open' && row.status !== 'dropped' && !targetExists(row.target, index)) console.log(`bad target: ${row.member} -> ${row.target}`);
  }
  const counts = {};
  for (const row of rows) counts[row.status] = (counts[row.status] || 0) + 1;
  console.log(JSON.stringify(counts));
}

main();
