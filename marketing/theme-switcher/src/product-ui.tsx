import React from 'react';
import { Command } from 'cmdk';
import { Folder, Image, Palette, Search, Sparkles, Zap } from 'lucide-react';
import { staticFile } from 'remotion';
import {
  ChatView,
  CommandRow,
  CommandTile,
  GridView,
} from '../../../src/app/palette/ui';
import { REQUEST, tween } from './timeline';

export const WALLPAPERS = [
  { id: 'mountain', title: 'Moonlit Mountains' },
  { id: 'pink', title: 'Neon Interior' },
  { id: 'forest', title: 'Misty Forest' },
  { id: 'range-dark', title: 'Midnight Ranges' },
  { id: 'range-light', title: 'Morning Ranges' },
  { id: 'meadow', title: 'Blue Meadow' },
  { id: 'kanagawa', title: 'The Great Wave' },
  { id: 'fuji', title: 'Mount Fuji' },
  { id: 'audi', title: 'Desert Drive' },
];

function ignore() {}

export function ProductUi({ frame, scene }: { frame: number; scene: string }) {
  const query = REQUEST.slice(0, Math.floor(Math.max(0, frame - 37) / 2.05));
  const grid = scene === 'grid';
  const chat = scene === 'chat';
  const selection = grid
    ? frame >= 504
      ? 'meadow'
      : frame >= 414
        ? 'forest'
        : 'pink'
    : query
      ? 'create'
      : 'apps';
  const slide = chat ? 1 - tween(frame, 174, 192) : 0;

  return (
    <div
      className={`productWindow ${chat ? 'chatWindow' : ''}`}
      style={{ transform: `translateX(${slide * 300}px)`, opacity: 1 - slide }}
    >
      <Command
        className="palette"
        label="Nevermind"
        shouldFilter={false}
        value={selection}
      >
        <div className="searchRow card searchCard">
          <Zap className="brandIcon" size={22} />
          <Command.Input
            value={grid ? '' : chat ? 'Theme Switcher' : query}
            onValueChange={ignore}
            placeholder={grid ? 'Search wallpapers…' : 'Search anything…'}
            readOnly
            spellCheck={false}
          />
          {!chat && !grid && query ? (
            <div className="tabHint">
              <Sparkles size={12} />
              <kbd>Tab</kbd> to ask AI
            </div>
          ) : null}
        </div>
        <Command.List
          className={`results card resultsCard ${chat ? 'aiChatResultsCard' : ''} ${grid ? 'largeResultsCard' : ''}`}
        >
          {grid ? (
            <GridView
              items={WALLPAPERS}
              layout="wide"
              renderItem={function renderWallpaper(item) {
                return (
                  <CommandTile
                    key={item.id}
                    value={item.id}
                    title={item.title}
                    subtitle="~/.config/wallpapers"
                    image={staticFile(`${item.id}.jpg`)}
                    onSelect={ignore}
                  />
                );
              }}
            />
          ) : chat ? (
            <ChatView
              messages={[{ role: 'user', content: REQUEST }]}
              isBusy
              activity="Building your extension..."
              input={
                <div className="chatInputRow">
                  <div className="chatInputShell">
                    <textarea readOnly placeholder="Thinking" rows={1} />
                    <div className="chatComposerFooter">
                      <span />
                      <button
                        className="chatIconButton chatStopButton"
                        type="button"
                        aria-label="Stop"
                      >
                        ■
                      </button>
                    </div>
                  </div>
                </div>
              }
            />
          ) : query ? (
            <CommandRow
              value="create"
              icon={<Sparkles size={20} />}
              title="Create with AI"
              subtitle="Build a reusable command with AI"
              appearance={{ foreground: 'yellow', background: 'accent' }}
              onSelect={ignore}
            />
          ) : (
            <>
              <CommandRow
                value="apps"
                icon={<Search size={20} />}
                title="Applications"
                subtitle="Find and open your apps"
                onSelect={ignore}
              />
              <CommandRow
                value="files"
                icon={<Folder size={20} />}
                title="Files"
                subtitle="Find something on your Mac"
                onSelect={ignore}
              />
              <CommandRow
                value="wallpapers"
                icon={<Image size={20} />}
                title="Wallpapers"
                subtitle="Desktop background and matching colors"
                onSelect={ignore}
              />
              <CommandRow
                value="themes"
                icon={<Palette size={20} />}
                title="Settings"
                subtitle="Make Nevermind yours"
                onSelect={ignore}
              />
            </>
          )}
        </Command.List>
      </Command>
    </div>
  );
}
