import assert from 'node:assert/strict';
import test from 'node:test';
import { applicationPathsFromSpotlight } from './app-uninstall-search';

test('ignores discarded and nested app bundles in Spotlight results', () => {
  assert.deepEqual(
    applicationPathsFromSpotlight(
      [
        '/Applications/Example.app',
        '/Users/test/.Trash/Example.app',
        '/Volumes/Data/.Trashes/501/Example.app',
        '/Applications/Example.app/Contents/Helper.app',
      ].join('\n'),
    ),
    ['/Applications/Example.app'],
  );
});
