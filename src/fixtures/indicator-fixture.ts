import type {
  ExtensionContext,
  ExtensionIndicatorInput,
  NevermindExtension,
} from '../resources/nevermind-extension-api';

const INDICATOR_ID = 'dev-ui-indicator';

const LISTENING_INDICATOR: ExtensionIndicatorInput = {
  id: INDICATOR_ID,
  title: 'Dictation Fixture',
  subtitle: 'Listening',
  status: 'recording',
};

const DOWNLOADING_INDICATOR: ExtensionIndicatorInput = {
  id: INDICATOR_ID,
  title: 'Dictation Fixture',
  subtitle: 'Downloading speech model...',
  status: 'loading',
};

const TRANSCRIBING_INDICATOR: ExtensionIndicatorInput = {
  id: INDICATOR_ID,
  title: 'Dictation Fixture',
  subtitle: 'Transcribing',
  status: 'transcribing',
};

function indicatorAction(
  title: string,
  input: ExtensionIndicatorInput,
  update: boolean,
) {
  return (ctx: ExtensionContext) =>
    ctx.actions.background(title, (innerCtx) => {
      if (update) innerCtx.ui.indicator.update(input);
      else innerCtx.ui.indicator.show(input);
      return innerCtx.ui.toast({
        message: `${input.subtitle} indicator shown`,
      });
    });
}

function indicatorView(ctx: ExtensionContext) {
  const listening = indicatorAction(
    'Show Listening Indicator',
    LISTENING_INDICATOR,
    false,
  )(ctx);
  const downloading = indicatorAction(
    'Show Long Loading Indicator',
    DOWNLOADING_INDICATOR,
    true,
  )(ctx);
  const transcribing = indicatorAction(
    'Show Transcribing Indicator',
    TRANSCRIBING_INDICATOR,
    true,
  )(ctx);
  const hide = ctx.actions.background('Hide Indicator', (innerCtx) => {
    innerCtx.ui.indicator.hide(INDICATOR_ID);
    return innerCtx.ui.toast({ message: 'Indicator hidden' });
  });
  const stackedLifecycle = ctx.actions.background(
    'Show Stacked Lifecycle',
    (innerCtx) => {
      innerCtx.ui.indicator.show(DOWNLOADING_INDICATOR);
      setTimeout(() => innerCtx.ui.indicator.update(LISTENING_INDICATOR), 300);
      setTimeout(
        () => innerCtx.ui.indicator.update(TRANSCRIBING_INDICATOR),
        900,
      );
      setTimeout(() => innerCtx.ui.indicator.hide(INDICATOR_ID), 1_500);
      return innerCtx.ui.toast({ message: 'Indicator lifecycle started' });
    },
  );

  return ctx.ui.list({
    id: 'dev-ui-indicator',
    title: 'Dev UI · Passive Indicator',
    subtitle: 'Exercises ctx.ui.indicator through the regular Extension API',
    items: [
      {
        id: 'stacked-lifecycle',
        title: 'Show Stacked Lifecycle',
        subtitle: 'Fast transitions remain readable as stable pills',
        icon: 'layers',
        primaryAction: stackedLifecycle,
      },
      {
        id: 'listening',
        title: 'Show Listening',
        subtitle: 'Short content-sized state',
        icon: 'mic',
        primaryAction: listening,
      },
      {
        id: 'downloading',
        title: 'Show Long Loading',
        subtitle: 'Long label creates a new fixed-width snapshot',
        icon: 'download',
        primaryAction: downloading,
      },
      {
        id: 'transcribing',
        title: 'Show Transcribing',
        subtitle: 'Newest state stacks above prior snapshots',
        icon: 'audio-lines',
        primaryAction: transcribing,
      },
      {
        id: 'hide',
        title: 'Hide Indicator',
        subtitle: 'Retire the active pill after its reading delay',
        icon: 'x',
        primaryAction: hide,
      },
    ],
  });
}

const extension: NevermindExtension = {
  id: 'dev.ui-indicator-fixture',
  title: 'Dev UI Indicator Fixture',
  subtitle: 'Dev-only Extension API passive indicator fixture',
  commands: [
    {
      id: 'indicator',
      title: 'Dev UI: Passive Indicator',
      icon: 'radio',
      run: (ctx) => indicatorView(ctx),
    },
  ],
};

export default extension;
