type DictationSettings = {
  deviceId: string;
  keepAliveMs: number;
  dictionary: string;
  copyToClipboard: boolean;
};

const DEFAULT_SETTINGS: DictationSettings = {
  deviceId: 'default',
  keepAliveMs: 5 * 60 * 1000,
  dictionary: '',
  copyToClipboard: false,
};

function normalizedKeepAliveMs(value: unknown) {
  const number = Number(value);
  return [300_000, 1_800_000, -1].includes(number)
    ? number
    : DEFAULT_SETTINGS.keepAliveMs;
}

async function readSettings(ctx: any): Promise<DictationSettings> {
  const stored = await ctx.storage.get('settings', DEFAULT_SETTINGS);
  return {
    ...DEFAULT_SETTINGS,
    ...(stored && typeof stored === 'object' ? stored : {}),
    keepAliveMs: normalizedKeepAliveMs(stored?.keepAliveMs),
    copyToClipboard: stored?.copyToClipboard === true,
  };
}

async function cleanTranscript(
  ctx: any,
  transcript: string,
  dictionary: string,
) {
  if (!dictionary.trim() || !ctx.ai) return transcript;
  const dictionaryText = dictionary.trim().slice(0, 4_000);
  try {
    const cleaned = await ctx.ai.ask(
      `Correct this dictated text for punctuation, capitalization, and likely terminology. Preserve the meaning and wording. Apply these dictionary terms exactly where appropriate:\n${dictionaryText}\n\nDictated text:\n${transcript}`,
      {
        model: 'fast',
        system:
          'You clean speech-to-text output. Return only the corrected text, with no explanation or quotation marks.',
      },
    );
    return cleaned.trim() || transcript;
  } catch {
    return transcript;
  }
}

async function settingsView(ctx: any) {
  const settings = await readSettings(ctx);
  let devices = [{ id: 'default', title: 'System Default', isDefault: true }];
  try {
    devices = await ctx.dictation.devices();
  } catch {}

  const saveAction = ctx.actions.run(
    'Save Dictation Settings',
    async (innerCtx: any, action: any) => {
      const values = action?.formValues || {};
      await innerCtx.storage.set('settings', {
        deviceId: String(values.deviceId || 'default'),
        keepAliveMs: normalizedKeepAliveMs(values.keepAliveMs),
        dictionary: String(values.dictionary || ''),
        copyToClipboard: Boolean(values.copyToClipboard),
      });
      return innerCtx.navigation.pop();
    },
  );

  return ctx.ui.form({
    id: 'dictation-settings',
    title: 'Dictation Settings',
    subtitle: 'Local Parakeet speech recognition',
    fields: [
      {
        id: 'deviceId',
        label: 'Microphone',
        type: 'dropdown',
        value: settings.deviceId,
        options: devices.map((device: any) => ({
          title: device.title,
          value: device.id,
        })),
      },
      {
        id: 'keepAliveMs',
        label: 'Keep model loaded',
        type: 'dropdown',
        value: String(settings.keepAliveMs),
        options: [
          { title: '5 minutes', value: '300000' },
          { title: '30 minutes', value: '1800000' },
          { title: 'Until Nevermind quits', value: '-1' },
        ],
      },
      {
        id: 'dictionary',
        label: 'Custom dictionary',
        type: 'textarea',
        value: settings.dictionary,
        rows: 8,
        placeholder: 'One preferred term per line',
        description: 'Terms are applied during the optional cleanup pass.',
      },
      {
        id: 'copyToClipboard',
        label: 'Copy transcription to clipboard',
        type: 'checkbox',
        value: settings.copyToClipboard,
        description: 'Keep the transcription on the clipboard after pasting.',
      },
    ],
    submitAction: saveAction,
  });
}

async function runDictation(ctx: any) {
  if (!ctx.dictation) throw new Error('Dictation is unavailable');
  const settings = await readSettings(ctx);
  const status = await ctx.dictation.status();
  if (status === 'idle') {
    ctx.ui.indicator.show({
      id: 'dictation',
      title: 'Dictation',
      subtitle: 'Checking speech model...',
      status: 'loading',
    });
    try {
      const modelCacheStatus = await ctx.dictation.modelCacheStatus();
      if (modelCacheStatus === 'missing') {
        ctx.ui.indicator.update({
          id: 'dictation',
          title: 'Dictation',
          subtitle: 'Downloading speech model...',
          status: 'loading',
        });
        await ctx.dictation.prepareModel({
          modelKeepAliveMs: settings.keepAliveMs,
        });
      }
      await ctx.dictation.start({
        deviceId: settings.deviceId,
        modelKeepAliveMs: settings.keepAliveMs,
      });
      ctx.ui.indicator.update({
        id: 'dictation',
        title: 'Dictation',
        subtitle: 'Listening',
        status: 'recording',
      });
      return ctx.ui.toast({ message: 'Listening...', tone: 'info' });
    } catch (error) {
      ctx.ui.indicator.hide('dictation');
      return ctx.ui.toast({
        message: `Dictation unavailable: ${error instanceof Error ? error.message : String(error)}`,
        tone: 'error',
      });
    }
  }
  ctx.ui.indicator.update({
    id: 'dictation',
    title: 'Dictation',
    subtitle: 'Transcribing',
    status: 'transcribing',
  });
  try {
    const transcript = await ctx.dictation.stop();
    if (!transcript.trim())
      return ctx.ui.toast({
        message: 'No speech detected',
        tone: 'info',
      });
    const text = await cleanTranscript(ctx, transcript, settings.dictionary);
    return ctx.navigation.run(
      ctx.actions.pasteText(text, 'Paste Dictation', {
        concealed: !settings.copyToClipboard,
        restoreClipboard: !settings.copyToClipboard,
        dismissAfterRun: 'auto',
      }),
    );
  } finally {
    ctx.ui.indicator.hide('dictation');
  }
}

function dictationRootItem(ctx: any) {
  const dictateAction = ctx.actions.ref('dictate', 'Dictate');
  const settingsAction = ctx.actions.run(
    'Settings',
    async (innerCtx: any) => ({
      view: await settingsView(innerCtx),
      navigation: 'push',
    }),
    { icon: 'settings-2' },
  );
  return {
    id: 'dictation',
    title: 'Dictate',
    subtitle: 'Start or stop local voice dictation',
    icon: 'mic',
    aliases: ['dictate', 'dictation', 'voice dictation'],
    primaryAction: dictateAction,
    actionPanel: {
      title: 'Dictation',
      sections: [{ actions: [settingsAction] }],
    },
  };
}

function dictationActionContribution(ctx: any) {
  return ctx.action({
    id: 'dictate',
    actionId: 'dictation',
    title: 'Dictate',
    subtitle: 'Start or stop local voice dictation',
    icon: 'mic',
    aliases: ['dictate', 'dictation', 'voice dictation'],
    background: true,
    dismissAfterRun: 'auto',
    customizable: true,
    placement: ['root'],
    run: runDictation,
  });
}

export function createDictationExtension() {
  return {
    id: 'nevermind.dictation',
    title: 'Dictation',
    subtitle: 'Local Parakeet speech-to-text',
    capabilities: ['dictation', 'ai'] as const,
    actions(ctx: any) {
      return [dictationActionContribution(ctx)];
    },
    rootItems(ctx: any) {
      return [dictationRootItem(ctx)];
    },
    searchItems(ctx: any, _query: string) {
      return [dictationRootItem(ctx)];
    },
  };
}
