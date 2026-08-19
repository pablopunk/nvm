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

  return ctx.ui.list({
    id: 'dev-ui-indicator',
    title: 'Dev UI · Passive Indicator',
    subtitle: 'Exercises ctx.ui.indicator through the regular Extension API',
    items: [
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
        subtitle: 'Content width with a transparent shadow gutter',
        icon: 'download',
        primaryAction: downloading,
      },
      {
        id: 'transcribing',
        title: 'Show Transcribing',
        subtitle: 'In-place update state',
        icon: 'audio-lines',
        primaryAction: transcribing,
      },
      {
        id: 'hide',
        title: 'Hide Indicator',
        subtitle: 'Dismiss the passive window',
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
