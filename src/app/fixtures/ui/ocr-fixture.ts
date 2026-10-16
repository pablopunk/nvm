// biome-ignore-all lint: Dev-only UI fixtures intentionally exercise dynamic extension payload shapes.
import type {
  ExtensionContext,
  ExtensionFile,
  ExtensionOcrResult,
} from '../../resources/nevermind-extension-api';

function ocrResultMarkdown(result: ExtensionOcrResult, source: string) {
  const text = result.text?.trim() || '_No text recognized._';
  const confidence =
    typeof result.confidence === 'number'
      ? `${Math.round(result.confidence * 100)}%`
      : 'unknown';
  return [
    '# OCR Result',
    '',
    `- Source: ${source}`,
    `- Confidence: ${confidence}`,
    `- Blocks: ${result.blocks?.length || 0}`,
    '',
    '## Text',
    '',
    text,
  ].join('\n');
}

function ocrUnavailableView(ctx: ExtensionContext) {
  return ctx.ui.preview({
    title: 'OCR Unavailable',
    content:
      '# OCR Unavailable\n\nDeclare the `ocr` capability for review and check `ctx.system.capabilities.has("ocr")` before using OCR helpers.',
  });
}

function ocrErrorView(ctx: ExtensionContext, error: unknown) {
  return ctx.ui.preview({
    title: 'OCR Error',
    content: `# OCR Error\n\n${error instanceof Error ? error.message : String(error)}`,
  });
}

async function runOcrImage(
  ctx: ExtensionContext,
  input: string | ExtensionFile,
  source: string,
) {
  if (!(ctx.ocr && ctx.system.capabilities.has('ocr')))
    return ocrUnavailableView(ctx);
  try {
    const result = await ctx.ocr.image(input);
    const image =
      typeof input === 'string'
        ? ctx.desktop.files?.thumbnail(input) ||
          ctx.desktop.files?.toFileUrl(input)
        : input.thumbnailUrl || input.url;
    return ctx.ui.preview({
      title: 'OCR Result',
      content: ocrResultMarkdown(result, source),
      image,
    });
  } catch (error) {
    return ocrErrorView(ctx, error);
  }
}

export function ocrFixtureView(ctx: ExtensionContext) {
  const chooseImage = ctx.input.prompt({
    title: 'Choose Image for OCR',
    fields: [
      {
        id: 'imagePath',
        label: 'Image',
        type: 'file',
        extensions: ['png', 'jpg', 'jpeg', 'webp', 'tiff', 'heic'],
        filterName: 'Images',
      },
    ],
    action: ctx.actions.run('OCR Chosen Image', (innerCtx, action) =>
      runOcrImage(
        innerCtx,
        String(action.formValues?.imagePath || ''),
        'chosen image',
      ),
    ),
    submitTitle: 'Recognize Text',
  });
  const recentImage = ctx.actions.run('OCR Recent Image', async (innerCtx) => {
    const images =
      (await innerCtx.desktop.files?.findImages(
        ['~/Downloads', '~/Desktop', '~/Pictures'],
        { limit: 1, depth: 2, sortBy: 'recent' },
      )) || [];
    if (!images.length)
      return innerCtx.ui.preview({
        title: 'No Image Found',
        content:
          '# No Image Found\n\nAdd an image to Downloads, Desktop, or Pictures, or use **Choose Image for OCR**.',
      });
    const file = await innerCtx.desktop.files!.metadata(images[0].path);
    return runOcrImage(innerCtx, file, file.displayPath || file.path);
  });
  const screenOcr = ctx.actions.run('OCR Screen', async (innerCtx) => {
    if (!(innerCtx.ocr && innerCtx.system.capabilities.has('ocr')))
      return ocrUnavailableView(innerCtx);
    try {
      const result = await innerCtx.ocr.screen();
      return innerCtx.ui.preview({
        title: 'Screen OCR Result',
        content: ocrResultMarkdown(result, 'screen capture'),
      });
    } catch (error) {
      return ocrErrorView(innerCtx, error);
    }
  });
  const regionOcr = ctx.actions.run('OCR Top-left Region', async (innerCtx) => {
    if (!(innerCtx.ocr && innerCtx.system.capabilities.has('ocr')))
      return ocrUnavailableView(innerCtx);
    try {
      const result = await innerCtx.ocr.region({
        x: 0,
        y: 0,
        width: 900,
        height: 600,
      });
      return innerCtx.ui.preview({
        title: 'Region OCR Result',
        content: ocrResultMarkdown(result, 'screen region 0,0,900×600'),
      });
    } catch (error) {
      return ocrErrorView(innerCtx, error);
    }
  });
  return ctx.ui.list({
    id: 'dev-ui-ocr',
    title: 'Dev UI · OCR',
    subtitle: 'Generic OCR helpers for images, screenshots, and regions',
    items: [
      ctx.ui.item({
        id: 'recent-image',
        title: 'OCR Recent Image',
        subtitle: 'Recognize text in the newest local image',
        icon: 'scan-text',
        primaryAction: recentImage,
        actions: [recentImage, chooseImage],
      }),
      ctx.ui.item({
        id: 'choose-image',
        title: 'Choose Image for OCR',
        subtitle: 'Uses a file picker plus ctx.ocr.image(...)',
        icon: 'file-image',
        primaryAction: chooseImage,
        actions: [chooseImage],
      }),
      ctx.ui.item({
        id: 'screen',
        title: 'OCR Screen',
        subtitle: 'Captures the screen, then recognizes visible text',
        icon: 'monitor',
        primaryAction: screenOcr,
        actions: [screenOcr],
      }),
      ctx.ui.item({
        id: 'region',
        title: 'OCR Top-left Region',
        subtitle:
          'Captures a fixed 900×600 region for dogfooding ctx.ocr.region(...)',
        icon: 'scan-line',
        primaryAction: regionOcr,
        actions: [regionOcr],
      }),
    ],
  });
}
