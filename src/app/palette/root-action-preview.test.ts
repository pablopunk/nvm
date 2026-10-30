import assert from 'node:assert/strict';
import test from 'node:test';
import { previewActionFlags, rootPreviewAction } from './root-action-preview';

test('declared content preview is used instead of a synthetic path preview', () => {
  const preview = {
    type: 'runExtensionAction' as const,
    title: 'Preview Contents',
    previewIntent: true,
  };
  const panel = { sections: [{ actions: [preview] }] };
  assert.equal(rootPreviewAction({ actionPanel: panel }), preview);
  assert.deepEqual(previewActionFlags(panel), {
    hasPreview: true,
    hasQuickLook: false,
  });
});

test('quick look remains distinct from a declared item preview', () => {
  const panel = {
    sections: [
      { actions: [{ type: 'quickLook' as const, title: 'Quick Look' }] },
    ],
  };
  assert.deepEqual(previewActionFlags(panel), {
    hasPreview: false,
    hasQuickLook: true,
  });
  assert.equal(rootPreviewAction({ actionPanel: panel }), undefined);
});
