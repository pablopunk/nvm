import assert from 'node:assert/strict';
import test from 'node:test';
import {
  declaredExtensionCapabilities,
  EXTENSION_TRUST_DISCLOSURE,
  filterWebviewPermissionsForExtension,
} from './extension-capabilities';

const FULL_ACCESS_PATTERN = /full access/i;
const NOT_RESTRICTIONS_PATTERN = /not security restrictions/i;

test('canonical capabilities win over legacy permissions, including an empty array', () => {
  assert.deepEqual(
    declaredExtensionCapabilities({
      capabilities: [],
      permissions: ['system'],
    }),
    { capabilities: [], provenance: 'capabilities' },
  );
  assert.deepEqual(declaredExtensionCapabilities({ permissions: ['system'] }), {
    capabilities: ['system'],
    provenance: 'legacy-permissions',
  });
  assert.deepEqual(declaredExtensionCapabilities({}), {
    capabilities: [],
    provenance: 'undeclared',
  });
});

test('webview iframe allowlisting is independent from declarations', () => {
  assert.deepEqual(
    filterWebviewPermissionsForExtension({ capabilities: [] }, [
      'autoplay',
      'camera',
      'camera',
      'clipboard-read',
      'unknown',
    ]),
    ['autoplay', 'camera', 'clipboard-read'],
  );
});

test('trust disclosure never describes capabilities as an enforcement boundary', () => {
  assert.match(EXTENSION_TRUST_DISCLOSURE, FULL_ACCESS_PATTERN);
  assert.match(EXTENSION_TRUST_DISCLOSURE, NOT_RESTRICTIONS_PATTERN);
});
