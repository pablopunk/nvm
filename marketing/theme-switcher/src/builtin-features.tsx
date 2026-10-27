import React from 'react';
import { Command } from 'cmdk';
import { Clipboard, FileText, Folder, Zap } from 'lucide-react';
import { AbsoluteFill, Img, staticFile, useCurrentFrame } from 'remotion';
import { CommandRow, ListView, PreviewView } from '../../../src/app/palette/ui';

const APPS = [
  { id: 'calendar', title: 'Calendar' },
  { id: 'calculator', title: 'Calculator' },
  { id: 'notes', title: 'Notes' },
  { id: 'finder', title: 'Finder' },
  { id: 'photos', title: 'Photos' },
  { id: 'terminal', title: 'Terminal' },
  { id: 'settings', title: 'System Settings' },
];
const CLIPS = [
  {
    id: 'forest',
    title: 'Misty forest.jpg',
    subtitle: 'Image · just now',
    image: 'forest',
  },
  {
    id: 'inspiration',
    title: 'Design inspiration.png',
    subtitle: 'Image · 1 minute ago',
    image: 'pink',
  },
  {
    id: 'checklist',
    title: 'Launch checklist',
    subtitle: 'Text · 2 minutes ago',
    image: null,
  },
  {
    id: 'link',
    title: 'https://github.com/pablopunk/nvm',
    subtitle: 'Text · 3 minutes ago',
    image: null,
  },
  {
    id: 'color',
    title: '#e6bd32',
    subtitle: 'Text · 5 minutes ago',
    image: null,
  },
  {
    id: 'message',
    title: 'Looks great. Let’s ship it.',
    subtitle: 'Text · 12 minutes ago',
    image: null,
  },
  {
    id: 'meadow',
    title: 'Summer wallpaper.png',
    subtitle: 'Image · 18 minutes ago',
    image: 'meadow',
  },
];

const FILES = [
  {
    id: 'brief',
    title: 'Design brief.pdf',
    path: '~/Documents/Projects',
    folder: false,
  },
  {
    id: 'system',
    title: 'Design system.fig',
    path: '~/Documents/Projects',
    folder: false,
  },
  {
    id: 'homepage',
    title: 'Homepage design.png',
    path: '~/Pictures',
    folder: false,
  },
  { id: 'notes', title: 'Design notes.md', path: '~/Documents', folder: false },
  { id: 'assets', title: 'Design assets', path: '~/Downloads', folder: true },
  {
    id: 'review',
    title: 'Design review.pdf',
    path: '~/Documents/Reviews',
    folder: false,
  },
  {
    id: 'icons',
    title: 'Design icons.svg',
    path: '~/Documents/Projects/Assets',
    folder: false,
  },
];

const DICTATIONS = [
  {
    id: 'idea',
    title: 'I have an idea for the next release.',
    date: '10/27/2026, 10:42 AM',
  },
  {
    id: 'reply',
    title: 'Thanks for the feedback. I’ll take a look today.',
    date: '10/27/2026, 10:35 AM',
  },
  {
    id: 'notes',
    title: 'Let’s keep the first version simple.',
    date: '10/27/2026, 10:18 AM',
  },
  {
    id: 'plan',
    title: 'The new demo is ready to share.',
    date: '10/27/2026, 9:56 AM',
  },
  {
    id: 'meeting',
    title: 'Can we move our meeting to tomorrow?',
    date: '10/27/2026, 9:32 AM',
  },
  {
    id: 'reminder',
    title: 'Remember to update the launch notes.',
    date: '10/27/2026, 9:14 AM',
  },
  {
    id: 'summary',
    title: 'Everything we need, one shortcut away.',
    date: '10/27/2026, 9:02 AM',
  },
];

function ignore() {}

export function BuiltinFeatures() {
  const frame = useCurrentFrame();
  const clipboard = frame === 1;
  const files = frame === 2;
  const dictation = frame === 3;
  const rows = clipboard
    ? CLIPS
    : files
      ? FILES
      : dictation
        ? DICTATIONS
        : APPS;
  return (
    <AbsoluteFill className="videoCanvas builtinFeature">
      <div className="productWindow">
        <Command
          className="palette"
          label="Nevermind"
          shouldFilter={false}
          value={rows[0].id}
        >
          <div className="searchRow card searchCard">
            <Zap className="brandIcon" size={22} />
            <Command.Input
              readOnly
              value={files ? 'design' : ''}
              placeholder={
                clipboard
                  ? 'Search Clipboard History'
                  : dictation
                    ? 'Search dictation history'
                    : 'Search anything…'
              }
              onValueChange={ignore}
            />
          </div>
          <Command.List
            className={`results card resultsCard ${clipboard ? 'sidePreviewCard' : ''}`}
          >
            {clipboard ? (
              <div className="extensionListWithDetail">
                <div className="extensionListPane">
                  <ListView
                    items={CLIPS}
                    renderItem={function renderClip(clip) {
                      return (
                        <CommandRow
                          key={clip.id}
                          value={clip.id}
                          icon={
                            clip.image ? (
                              <Img
                                src={staticFile(`${clip.image}.jpg`)}
                                style={{
                                  width: 28,
                                  height: 28,
                                  objectFit: 'cover',
                                  borderRadius: 4,
                                }}
                              />
                            ) : (
                              <Clipboard size={18} />
                            )
                          }
                          title={clip.title}
                          subtitle={clip.subtitle}
                          appearance={{ foreground: 'purple' }}
                          onSelect={ignore}
                        />
                      );
                    }}
                  />
                </div>
                <div className="extensionDetailPane sidePreviewDetailPane">
                  <PreviewView image={staticFile('forest.jpg')} />
                </div>
              </div>
            ) : dictation ? (
              <ListView
                items={DICTATIONS}
                renderItem={function renderTranscript(entry) {
                  return (
                    <CommandRow
                      key={entry.id}
                      value={entry.id}
                      icon={<FileText size={20} />}
                      title={entry.title}
                      subtitle={entry.date}
                      onSelect={ignore}
                    />
                  );
                }}
              />
            ) : files ? (
              <ListView
                items={FILES}
                renderItem={function renderFile(file) {
                  return (
                    <CommandRow
                      key={file.id}
                      value={file.id}
                      icon={
                        file.folder ? (
                          <Folder size={20} />
                        ) : (
                          <FileText size={20} />
                        )
                      }
                      title={file.title}
                      subtitle={file.path}
                      onSelect={ignore}
                    />
                  );
                }}
              />
            ) : (
              <ListView
                items={APPS}
                renderItem={function renderApp(app) {
                  return (
                    <CommandRow
                      key={app.id}
                      value={app.id}
                      icon={
                        <Img
                          src={staticFile(`app-${app.id}.png`)}
                          style={{ width: 32, height: 32 }}
                        />
                      }
                      title={app.title}
                      subtitle="Launch application"
                      onSelect={ignore}
                    />
                  );
                }}
              />
            )}
          </Command.List>
        </Command>
      </div>
    </AbsoluteFill>
  );
}
