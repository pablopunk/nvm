import { Command } from 'cmdk';
import { ArrowUp, Zap } from 'lucide-react';
import React from 'react';
import { AbsoluteFill, staticFile } from 'remotion';
import { ChatView, CommandTile, GridView } from '../../../src/app/palette/ui';
import { WALLPAPERS } from './product-ui';

function ignore() {}

export function BuilderPreview() {
  return (
    <AbsoluteFill className="videoCanvas builderFeature">
      <div className="productWindow">
        <Command
          className="palette"
          label="Nevermind"
          shouldFilter={false}
          value="mountain"
        >
          <div className="searchRow card searchCard">
            <Zap className="brandIcon" size={22} />
            <Command.Input
              value="Theme Switcher"
              readOnly={true}
              onValueChange={ignore}
            />
          </div>
          <Command.List className="results card resultsCard builderResultsCard">
            <div className="builderWorkspace">
              <div className="builderChatPane">
                <ChatView
                  messages={[
                    {
                      role: 'user',
                      content:
                        'Create a theme switcher for my Mac. Show my wallpapers in a grid and match the colors when I pick one.',
                    },
                    {
                      role: 'assistant',
                      content:
                        'Your theme switcher is ready. Pick a wallpaper in the preview to try it.',
                    },
                    {
                      role: 'user',
                      content: 'Can I use this again without asking you?',
                    },
                    {
                      role: 'assistant',
                      content:
                        'Yes. It’s saved as a command. Open Theme Switcher from the palette any time. The code handles the rest.',
                    },
                  ]}
                  input={
                    <div className="chatInputRow">
                      <div className="chatInputShell">
                        <textarea
                          readOnly={true}
                          placeholder="Ask for changes…"
                          rows={1}
                        />
                        <div className="chatComposerFooter">
                          <span />
                          <button
                            className="chatIconButton"
                            type="button"
                            aria-label="Send"
                          >
                            <ArrowUp size={16} />
                          </button>
                        </div>
                      </div>
                    </div>
                  }
                />
              </div>
              <section className="builderPreviewPane">
                <header className="builderPreviewHeader">
                  <span>theme-switcher</span>
                </header>
                <div className="builderPreviewBody">
                  <GridView
                    items={WALLPAPERS.slice(0, 6)}
                    layout="wide"
                    renderItem={function renderWallpaper(item) {
                      return (
                        <CommandTile
                          key={item.id}
                          value={item.id}
                          title={item.title}
                          image={staticFile(`${item.id}.jpg`)}
                          onSelect={ignore}
                        />
                      );
                    }}
                  />
                </div>
              </section>
            </div>
          </Command.List>
        </Command>
      </div>
    </AbsoluteFill>
  );
}
