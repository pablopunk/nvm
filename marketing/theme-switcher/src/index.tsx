import React from 'react';
import { Composition, registerRoot } from 'remotion';
import { BuilderPreview } from './builder-preview';
import { BuiltinFeatures } from './builtin-features';
import { DICTATION_DURATION, DictationDemo } from './dictation-demo';
import { ThemeSwitcher } from './theme-switcher';
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
        id="BuilderPreview"
        component={BuilderPreview}
        width={1060}
        height={680}
        fps={30}
        durationInFrames={1}
      />
      <Composition
        id="BuiltinFeatures"
        component={BuiltinFeatures}
        width={1060}
        height={560}
        fps={30}
        durationInFrames={4}
      />
      <Composition
        id="DictationDemo"
        component={DictationDemo}
        width={1060}
        height={560}
        fps={30}
        durationInFrames={DICTATION_DURATION}
      />
    </>
  );
}

registerRoot(VideoRoot);
