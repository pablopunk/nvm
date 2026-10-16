import assert from 'node:assert/strict';
import test from 'node:test';
import { appBundleIconFileNames, pngImagesFromIcns } from './app-bundle-icons';

test('appBundleIconFileNames reads CFBundleIconFile and adds icns extension', () => {
  const names = appBundleIconFileNames(`
    <plist><dict>
      <key>CFBundleIconFile</key>
      <string>AppIcon</string>
    </dict></plist>
  `);

  assert.deepEqual(names, ['AppIcon', 'AppIcon.icns']);
});

test('appBundleIconFileNames reads CFBundleIconFiles arrays', () => {
  const names = appBundleIconFileNames(`
    <plist><dict>
      <key>CFBundleIcons</key>
      <dict>
        <key>CFBundlePrimaryIcon</key>
        <dict>
          <key>CFBundleIconFiles</key>
          <array>
            <string>Icon16</string>
            <string>Icon512.icns</string>
          </array>
        </dict>
      </dict>
    </dict></plist>
  `);

  assert.deepEqual(names, ['Icon16', 'Icon16.icns', 'Icon512.icns']);
});

test('pngImagesFromIcns extracts embedded png icon entries', () => {
  const png = Buffer.concat([
    Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
    Buffer.alloc(24),
  ]);
  const entry = Buffer.alloc(8 + png.length);
  entry.write('ic12', 0, 'ascii');
  entry.writeUInt32BE(entry.length, 4);
  png.copy(entry, 8);
  const header = Buffer.alloc(8);
  header.write('icns', 0, 'ascii');
  header.writeUInt32BE(header.length + entry.length, 4);

  assert.deepEqual(pngImagesFromIcns(Buffer.concat([header, entry])), [png]);
});
