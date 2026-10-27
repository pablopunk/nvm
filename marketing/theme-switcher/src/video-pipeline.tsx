import React from 'react';
import { Command } from 'cmdk';
import {
  CheckCircle2,
  ChevronLeft,
  ChevronRight,
  Film,
  Folder,
  Play,
  Search,
  Zap,
} from 'lucide-react';
import { staticFile } from 'remotion';
import {
  CommandRow,
  ListView,
  ProgressView,
} from '../../../src/app/palette/ui';
import type { EXTENSION_SCENES } from './extension-scenes';
import { tween } from './timeline';

const CLIPS = [
  { name: 'coast.mov', image: 'meadow', duration: '00:12' },
  { name: 'forest.mov', image: 'forest', duration: '00:18' },
  { name: 'city.mov', image: 'pink', duration: '00:09' },
];
const STEPS = [
  'Converting to MP4...',
  'Resizing to 1080p...',
  'Speeding up to 1.5x...',
  'Merging all videos...',
];

function ignore() {}

function SelectedVideos() {
  return (
    <div className="finderWindow">
      <div className="finderToolbar">
        <div className="finderTraffic">
          <i />
          <i />
          <i />
        </div>
        <ChevronLeft size={18} />
        <ChevronRight size={18} />
        <strong>Video clips</strong>
        <Search size={18} />
      </div>
      <div className="finderBody">
        <aside className="finderSidebar">
          <small>Favorites</small>
          <span>Recents</span>
          <span>Applications</span>
          <span>Desktop</span>
          <span>Documents</span>
          <span>Downloads</span>
          <span className="finderFolder">
            <Folder size={16} /> Video clips
          </span>
        </aside>
        <div className="finderFiles">
          {CLIPS.map(function renderClip(clip) {
            return (
              <div className="finderSelectedFile" key={clip.name}>
                <div className="finderThumbnail">
                  <img src={staticFile(`${clip.image}.jpg`)} alt="" />
                  <Play size={22} fill="currentColor" />
                  <small>{clip.duration}</small>
                </div>
                <span>{clip.name}</span>
              </div>
            );
          })}
        </div>
      </div>
      <div className="finderStatus">3 of 3 selected</div>
    </div>
  );
}

export function VideoPipeline({
  frame,
  scene,
}: {
  frame: number;
  scene: (typeof EXTENSION_SCENES)[number];
}) {
  const running = frame >= scene.listStart;
  const activeStep = Math.floor((frame - scene.listStart) / 32);
  const complete = activeStep >= STEPS.length;
  const query = scene.query.slice(
    0,
    Math.floor(Math.max(0, frame - scene.typingStart) / 1.2),
  );
  const enter = tween(frame, 708, 716);
  const steps = STEPS.map(function step(title, index) {
    return {
      title,
      status:
        index < activeStep
          ? 'Done'
          : index === activeStep
            ? 'Running'
            : 'Pending',
    };
  });
  return (
    <>
      <div
        style={{
          opacity: tween(frame, scene.searchStart, scene.searchStart + 8),
        }}
      >
        <SelectedVideos />
      </div>
      <div
        className="productWindow pipelineWindow"
        style={{
          opacity: enter,
          transform: `translateY(${(1 - enter) * 16}px)`,
        }}
      >
        <Command
          key={complete ? 'complete' : running ? 'running' : 'search'}
          className="palette"
          label="Nevermind"
          shouldFilter={false}
          value={complete ? 'final-video' : 'pipeline'}
        >
          <div className="searchRow card searchCard">
            <Zap className="brandIcon" size={22} />
            <Command.Input
              value={running ? scene.title : query}
              onValueChange={ignore}
              readOnly
              placeholder="Search anything…"
            />
          </div>
          <Command.List className="results card resultsCard">
            {!running ? (
              <CommandRow
                value="pipeline"
                icon={<Film size={20} />}
                title="Video pipeline"
                subtitle="Convert, resize, speed up and merge 3 selected videos"
                onSelect={ignore}
              />
            ) : complete ? (
              <>
                <div className="pipelineSuccess">
                  <CheckCircle2 size={20} />
                  <div>
                    <strong>Your video is ready</strong>
                    <small>
                      3 clips converted, resized, sped up and merged
                    </small>
                  </div>
                </div>
                <ListView
                  items={[{ id: 'final-video' }]}
                  renderItem={function renderOutput(item) {
                    return (
                      <CommandRow
                        key={item.id}
                        value={item.id}
                        icon={<Film size={20} />}
                        title="final-video.mp4"
                        subtitle="~/Movies/Video clips"
                        accessories={[
                          { text: '1080p' },
                          { text: '00:26' },
                          { text: 'MP4' },
                        ]}
                        onSelect={ignore}
                      />
                    );
                  }}
                />
              </>
            ) : (
              <ProgressView
                steps={steps}
                value={activeStep}
                total={4}
                label={STEPS[activeStep]}
                status="running"
              />
            )}
          </Command.List>
        </Command>
      </div>
    </>
  );
}
