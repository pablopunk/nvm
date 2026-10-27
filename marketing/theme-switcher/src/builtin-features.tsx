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
];

function ignore() {}

export function BuiltinFeatures() {
  const frame = useCurrentFrame();
  const clipboard = frame === 1;
  const files = frame === 2;
  const rows = clipboard ? CLIPS : files ? FILES : APPS;
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
                clipboard ? 'Search Clipboard History' : 'Search anything…'
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
