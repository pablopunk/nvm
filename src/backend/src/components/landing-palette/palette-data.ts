import type { PaletteItem, PaletteTone } from './palette-types';

import builtinCommandsJson from './palette-items.json';

/** Built-in commands of the desktop app, captured as static landing-page content. */
const builtinCommands = builtinCommandsJson as PaletteItem[];

interface SampleDetails {
  keywords?: string[];
  tone?: PaletteTone;
  primary?: string;
  image?: string;
  url?: string;
}

function sample(
  kind: PaletteItem['kind'],
  title: string,
  subtitle: string,
  icon: string,
  details: SampleDetails = {},
): PaletteItem {
  return { id: `${kind}:${title}`, title, subtitle, icon, aliases: [], kind, ...details };
}

export const RELEASES_URL = 'https://github.com/pablopunk/nvm/releases';
/** What the dictation demo "transcribes" and pastes into the palette. */
export const DICTATED_TEXT = 'Download nevermind';
/** The 2.x palette's first empty-query placeholder (src/native/internal/palette/render.go). */
export const ROOT_PLACEHOLDER = 'Watcha gonna do?';
export const DICTATE_ID = 'command:Dictate';

/** App rows carry no subtitle in 2.x; "launch" still finds them. */
function app(title: string, keywords: string[], image: string, url: string) {
  return sample('app', title, '', 'app-window', {
    keywords: [...keywords, 'app', 'launch'],
    image,
    url,
  });
}

const apps: PaletteItem[] = [
  app('Helium', ['browser', 'web'], '/apps/helium.webp', 'https://helium.computer'),
  app('T3 Code', ['code', 'agent', 'ai'], '/apps/t3-code.webp', 'https://t3.codes'),
  app('Spotifin', ['music', 'player'], '/apps/spotifin.webp', 'https://spotifin.app'),
  app('Telegram', ['chat', 'messages'], '/apps/telegram.webp', 'https://telegram.org'),
  app('Nevermind', ['palette', 'launcher'], '/apps/nevermind.webp', RELEASES_URL),
];

const design = { keywords: ['design'] };
const files: PaletteItem[] = [
  sample('file', 'Brand guidelines.pdf', '~/Documents/Design/Brand guidelines.pdf', 'file-text', design),
  sample('file', 'Design system.fig', '~/Documents/Design/Design system.fig', 'palette', design),
  sample('file', 'Pricing page mockup.png', '~/Pictures/Design/Pricing page mockup.png', 'image', design),
  sample('file', 'Design review notes.md', '~/Documents/Notes/Design review notes.md', 'file-code', design),
  sample('file', 'Roadmap Q4.pdf', '~/Documents/Planning/Roadmap Q4.pdf', 'file-text'),
  sample('file', 'Invoices', '~/Documents/Finance/Invoices', 'folder', { tone: 'blue' }),
];

const copied = { keywords: ['clipboard', 'copied', 'paste'] };
const clipboard: PaletteItem[] = [
  sample('clipboard', 'https://nvm.fyi/pricing', 'Copied 2 minutes ago', 'link', copied),
  sample('clipboard', 'git checkout -b feature/palette', 'Copied 14 minutes ago', 'terminal', copied),
  sample('clipboard', 'Meeting at 3pm, bring the roadmap', 'Copied 1 hour ago', 'type', copied),
  sample('clipboard', '#f4cf86', 'Copied yesterday', 'palette', copied),
];

const voiceKeywords = ['dictate', 'dictation', 'voice', 'speech', 'transcribe'];
const voice: PaletteItem[] = [
  sample('command', 'Dictate', 'Start or stop cloud voice dictation', 'mic', { keywords: voiceKeywords }),
  sample('command', 'Dictation History', 'Browse and copy previous dictations', 'history', {
    keywords: voiceKeywords,
    primary: 'Open Dictation History',
  }),
];

const downloadNevermind = sample(
  'command',
  'Download Nevermind',
  'Get the app for macOS, Windows, and Linux',
  'download',
  { keywords: ['install', 'get', 'app', 'releases'], tone: 'green', url: RELEASES_URL },
);

const clipboardHistory = sample(
  'command',
  'Clipboard History',
  'Browse and paste what you copied',
  'clipboard',
  { keywords: ['clipboard', 'copied', 'paste'], tone: 'blue', primary: 'Open Clipboard History' },
);

/** Empty-query order mirrors a fresh palette: apps first, then commands. */
export const paletteItems: PaletteItem[] = [
  ...apps,
  downloadNevermind,
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
  icon: string;
}

export const paletteTabs: PaletteTab[] = [
  { id: 'apps', label: 'App launcher', query: '', icon: 'app-window' },
  { id: 'clipboard', label: 'Clipboard history', query: 'clipboard', icon: 'clipboard' },
  { id: 'files', label: 'File finder', query: 'design', icon: 'file-search' },
  { id: 'dictation', label: 'Dictation', query: 'dictate', icon: 'mic' },
];
