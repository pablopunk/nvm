import React from 'react';
import { Composition, registerRoot } from 'remotion';
import { ThemeSwitcher } from './theme-switcher';
import { BuiltinFeatures } from './builtin-features';
import { DURATION, FPS } from './timeline';
import '../../../src/app/palette/styles.css';
import './video.css';

function VideoRoot() {
  return (
    <>
      <Composition
        id="ThemeSwitcher"
        component={ThemeSwitcher}
        width={1920}
        height={1080}
        fps={FPS}
        durationInFrames={DURATION}
      />
      <Composition
        id="BuiltinFeatures"
        component={BuiltinFeatures}
        width={1060}
        height={560}
        fps={30}
        durationInFrames={3}
      />
    </>
  );
}

registerRoot(VideoRoot);
