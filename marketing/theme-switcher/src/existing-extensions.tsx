import React from 'react';
import { Command } from 'cmdk';
import { CalendarDays, GitPullRequest, Video, Zap } from 'lucide-react';
import {
  CommandRow,
  ListView,
  SearchAccessory,
} from '../../../src/app/palette/ui';
import { CALENDAR_EVENTS, PULL_REQUESTS } from './extension-demo-data';
import { existingExtensionAt } from './extension-scenes';
import { tween } from './timeline';
import { VideoPipeline } from './video-pipeline';

function ignore() {}

export function ExistingExtensions({ frame }: { frame: number }) {
  const scene = existingExtensionAt(frame);
  if (!scene) return null;
  if (scene.id === 'pipeline')
    return <VideoPipeline frame={frame} scene={scene} />;
  const list = frame >= scene.listStart;
  const github = scene.id === 'github';
  const query = scene.query.slice(
    0,
    Math.floor(Math.max(0, frame - scene.typingStart) / 1.2),
  );
  const selected = list
    ? github
      ? PULL_REQUESTS[0].id
      : CALENDAR_EVENTS[0].id
    : scene.id;
  const enter = tween(
    frame,
    list ? scene.listStart : scene.searchStart,
    (list ? scene.listStart : scene.searchStart) + 8,
  );
  const searchIcon = github ? (
    <GitPullRequest size={20} />
  ) : (
    <CalendarDays size={20} />
  );

  return (
    <div
      className="productWindow existingExtensionWindow"
      style={{ opacity: enter, transform: `translateY(${(1 - enter) * 16}px)` }}
    >
      <Command
        key={`${scene.id}:${list ? 'list' : 'search'}`}
        className="palette"
        label="Nevermind"
        shouldFilter={false}
        value={selected}
      >
        <div className="searchRow card searchCard">
          <Zap className="brandIcon" size={22} />
          <Command.Input
            value={list ? scene.title : query}
            onValueChange={ignore}
            placeholder="Search anything…"
            readOnly
            spellCheck={false}
          />
          {github && list ? (
            <SearchAccessory
              value="authored"
              items={[{ value: 'authored', title: 'Authored by me' }]}
              onChange={ignore}
            />
          ) : null}
        </div>
        <Command.List className="results card resultsCard">
          {!list ? (
            <CommandRow
              value={scene.id}
              icon={searchIcon}
              title={scene.command}
              subtitle={scene.subtitle}
              onSelect={ignore}
            />
          ) : github ? (
            <ListView
              items={PULL_REQUESTS}
              renderItem={function renderPullRequest(pr) {
                return (
                  <CommandRow
                    key={pr.id}
                    value={pr.id}
                    icon={<GitPullRequest size={18} />}
                    title={pr.title}
                    subtitle={pr.subtitle}
                    accessories={[{ text: '10/27/2026' }, { text: pr.label }]}
                    appearance={{ foreground: 'green' }}
                    onSelect={ignore}
                  />
                );
              }}
            />
          ) : (
            <ListView
              items={CALENDAR_EVENTS}
              renderItem={function renderEvent(event) {
                return (
                  <CommandRow
                    key={event.id}
                    value={event.id}
                    icon={
                      event.meeting ? (
                        <Video size={18} />
                      ) : (
                        <CalendarDays size={18} />
                      )
                    }
                    title={event.title}
                    subtitle={event.subtitle}
                    accessories={[{ text: event.calendar }]}
                    appearance={{
                      foreground: event.ongoing ? 'green' : 'blue',
                    }}
                    onSelect={ignore}
                  />
                );
              }}
            />
          )}
        </Command.List>
      </Command>
    </div>
  );
}
