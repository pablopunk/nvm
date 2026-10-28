import { inspectExtensionManifest } from '../extension-host/extension-manifest';

export function validateExtensionCapabilities(source: string) {
  const manifest = inspectExtensionManifest(source);
  if (!(manifest.id && manifest.title))
    throw new Error(
      'Extension must statically declare literal id and title values',
    );
  const capabilities = new Set(manifest.capabilities);
  const declarations = [
    [
      'system',
      /ctx\.desktop\.shell\b|ctx\.actions\.(?:shellExec|shellScript|system)\b/,
    ],
    ['desktop.files', /ctx\.desktop\.files\b|ctx\.desktop\.selection\.files\b/],
    ['desktop.apps', /ctx\.desktop\.apps\b/],
    ['desktop.window-management', /ctx\.desktop\.windowManagement\b/],
    [
      'clipboard.history',
      /ctx\.clipboard\.history\b|ctx\.desktop\.clipboard\b/,
    ],
    ['ocr', /ctx\.ocr\b/],
    ['ai', /ctx\.ai\b/],
    ['settings.write', /ctx\.settings\.(?:set|toggle)\b/],
    [
      'shortcuts',
      /ctx\.shortcuts\b|ctx\.actions\.(?:recordShortcut|removeShortcut|setPaletteShortcut)\b/,
    ],
    ['updates', /ctx\.updates\b|ctx\.actions\.updates\b/],
    ['extensions.ownership', /ctx\.extensions\.ownership\b/],
  ] as const;
  for (const [capability, pattern] of declarations) {
    if (pattern.test(source) && !capabilities.has(capability)) {
      throw new Error(
        `Extension uses ${capability} helpers but does not declare the '${capability}' capability. Add it to capabilities so reviewers understand its intent.`,
      );
    }
  }
}
