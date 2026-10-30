import { showExtensionFeedback } from '../extensions/feedback.ts';
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

const TRIAGE_RESULT_INDICATOR: ExtensionIndicatorInput = {
  id: INDICATOR_ID,
  title: 'Triage Selected Text with AI',
  subtitle: 'question · urgency 4/5 · needs reply',
  status: 'success',
};

const TRIAGE_ERROR_INDICATOR: ExtensionIndicatorInput = {
  id: INDICATOR_ID,
  title: 'Triage Selected Text with AI',
  subtitle: 'The decision model is unavailable. Try again shortly.',
  status: 'error',
};

const LONG_ERROR_INDICATOR: ExtensionIndicatorInput = {
  id: INDICATOR_ID,
  title: 'Triage Selected Text with AI',
  subtitle:
    'The decision model returned an unexpected response after the provider timed out twice. Check your connection, then run Triage Selected Text with AI again.',
  status: 'error',
  durationMs: 4000,
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
  const triageResult = indicatorAction(
    'Show Triage Result',
    TRIAGE_RESULT_INDICATOR,
    true,
  )(ctx);
  const triageError = indicatorAction(
    'Show Triage Error',
    TRIAGE_ERROR_INDICATOR,
    true,
  )(ctx);
  const longError = indicatorAction(
    'Show Long Error',
    LONG_ERROR_INDICATOR,
    true,
  )(ctx);
  const hide = ctx.actions.background('Hide Indicator', (innerCtx) => {
    innerCtx.ui.indicator.hide(INDICATOR_ID);
    showExtensionFeedback(innerCtx, 'Indicator Fixture', 'Indicator hidden');
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
        id: 'triage-result',
        title: 'Show Triage Result',
        subtitle: 'Success state with a decision summary',
        icon: 'list-checks',
        primaryAction: triageResult,
      },
      {
        id: 'triage-error',
        title: 'Show Triage Error',
        subtitle: 'Error state with a decision failure message',
        icon: 'triangle-alert',
        primaryAction: triageError,
      },
      {
        id: 'long-error',
        title: 'Show Long Error',
        subtitle: 'Timed error that wraps to bounded lines and stays readable',
        icon: 'triangle-alert',
        primaryAction: longError,
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
