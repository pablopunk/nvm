import type { PaletteItem } from './palette-types';

/**
 * Real built-in commands, extracted from the desktop app by
 * `scripts/generate-palette-items.ts`. The glob keeps the page building if the
 * generated file is missing.
 */
const generatedModules = import.meta.glob<PaletteItem[]>(
  '../../generated/palette-items.json',
  { eager: true, import: 'default' },
);
const builtinCommands: PaletteItem[] =
  Object.values(generatedModules)[0] ?? [];

function sample(
  kind: PaletteItem['kind'],
  title: string,
  subtitle: string,
  icon: string,
  keywords: string[] = [],
  image?: string,
): PaletteItem {
  return {
    id: `${kind}:${title}`,
    title,
    subtitle,
    icon,
    aliases: [],
    kind,
    keywords,
    image,
  };
}

const LAUNCH_SUBTITLE = 'Launch application';
const apps: PaletteItem[] = [
  sample('app', 'Helium', LAUNCH_SUBTITLE, 'app-window', ['browser', 'web'], '/apps/helium.webp'),
  sample('app', 'T3 Code', LAUNCH_SUBTITLE, 'app-window', ['code', 'agent', 'ai'], '/apps/t3-code.webp'),
  sample('app', 'Spotifin', LAUNCH_SUBTITLE, 'app-window', ['music', 'player'], '/apps/spotifin.webp'),
  sample('app', 'Telegram', LAUNCH_SUBTITLE, 'app-window', ['chat', 'messages'], '/apps/telegram.webp'),
  sample('app', 'Nevermind', LAUNCH_SUBTITLE, 'app-window', ['palette', 'launcher'], '/apps/nevermind.webp'),
  sample('app', 'Calendar', LAUNCH_SUBTITLE, 'calendar'),
  sample('app', 'Calculator', LAUNCH_SUBTITLE, 'calculator'),
  sample('app', 'Notes', LAUNCH_SUBTITLE, 'sticky-note'),
  sample('app', 'Finder', LAUNCH_SUBTITLE, 'folder-open'),
  sample('app', 'Photos', LAUNCH_SUBTITLE, 'images'),
  sample('app', 'Terminal', LAUNCH_SUBTITLE, 'terminal'),
  sample('app', 'System Settings', LAUNCH_SUBTITLE, 'settings'),
];

const files: PaletteItem[] = [
  sample('file', 'Brand guidelines.pdf', '~/Documents/Design/Brand guidelines.pdf', 'file-text', ['design']),
  sample('file', 'Design system.fig', '~/Documents/Design/Design system.fig', 'palette', ['design']),
  sample('file', 'Pricing page mockup.png', '~/Pictures/Design/Pricing page mockup.png', 'image', ['design']),
  sample('file', 'Design review notes.md', '~/Documents/Notes/Design review notes.md', 'file-code', ['design']),
  sample('file', 'Roadmap Q4.pdf', '~/Documents/Planning/Roadmap Q4.pdf', 'file-text'),
  sample('file', 'Invoices', '~/Documents/Finance/Invoices', 'folder'),
];

const clipboard: PaletteItem[] = [
  sample('clipboard', 'https://nvm.fyi/pricing', 'Copied 2 minutes ago', 'link', ['clipboard', 'copied', 'paste']),
  sample('clipboard', 'ssh pol@192.168.1.153', 'Copied 14 minutes ago', 'terminal', ['clipboard', 'copied', 'paste']),
  sample('clipboard', 'Meeting at 3pm, bring the roadmap', 'Copied 1 hour ago', 'type', ['clipboard', 'copied', 'paste']),
  sample('clipboard', '#f4cf86', 'Copied yesterday', 'palette', ['clipboard', 'copied', 'paste']),
];

const voice: PaletteItem[] = [
  sample('command', 'Start Dictation', 'Speak and Nevermind pastes the transcript', 'mic', ['dictate', 'voice', 'speech', 'transcribe']),
  sample('command', 'Dictation History', 'Browse past transcripts', 'history', ['dictate', 'voice', 'speech']),
];

const clipboardHistory = sample(
  'command',
  'Clipboard History',
  'Search everything you copied',
  'clipboard',
  ['clipboard', 'copied', 'paste'],
);

/** Empty-query order mirrors a fresh palette: apps first, then commands. */
export const paletteItems: PaletteItem[] = [
  ...apps,
  clipboardHistory,
  ...voice,
  ...builtinCommands,
  ...clipboard,
  ...files,
];

export interface PaletteTab {
  id: string;
  label: string;
  query: string;
}

export const paletteTabs: PaletteTab[] = [
  { id: 'apps', label: 'App launcher', query: '' },
  { id: 'clipboard', label: 'Clipboard history', query: 'clipboard' },
  { id: 'files', label: 'File finder', query: 'design' },
  { id: 'dictation', label: 'Dictation', query: 'dictate' },
];
