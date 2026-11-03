import assert from 'node:assert/strict';
import test from 'node:test';
import {
  createLinkPreviewService,
  parseLinkMetadata,
  singleWebUrl,
} from './link-preview';

test('singleWebUrl accepts only one public http(s) URL', () => {
  assert.equal(
    singleWebUrl(' https://github.com/a/b/pull/1 '),
    'https://github.com/a/b/pull/1',
  );
  assert.equal(singleWebUrl('see https://github.com'), null);
  assert.equal(singleWebUrl('ftp://example.com/file'), null);
  assert.equal(singleWebUrl('http://localhost:3000'), null);
  assert.equal(singleWebUrl('http://192.168.1.2/admin'), null);
  assert.equal(singleWebUrl('http://intranet/page'), null);
  assert.equal(singleWebUrl('http://[::1]/'), null);
  assert.equal(singleWebUrl('http://169.254.169.254/latest'), null);
  assert.equal(singleWebUrl('http://100.64.0.1/'), null);
  assert.equal(singleWebUrl('http://8.8.8.8/'), 'http://8.8.8.8/');
});

test('parseLinkMetadata prefers Open Graph and resolves relative images', () => {
  const html = `<head><title>Fallback &amp; title</title>
    <meta property="og:title" content="PR &quot;40902&quot;">
    <meta content="Adds a thing" property="og:description">
    <meta property="og:site_name" content="GitHub">
    <meta property="og:image" content="/img/card.png"></head>`;
  assert.deepEqual(parseLinkMetadata(html, 'https://github.com/a/b'), {
    title: 'PR "40902"',
    description: 'Adds a thing',
    siteName: 'GitHub',
    image: 'https://github.com/img/card.png',
  });
});

test('parseLinkMetadata handles quoted > and mixed attribute styles', () => {
  const html = `<meta property="og:title" content="A > B">
    <meta name='description' content='x > "y"'>
    <meta content=plain property=og:site_name>
    <meta property="og:image"
          content="https://cdn.example.com/a.png" />`;
  assert.deepEqual(parseLinkMetadata(html, 'https://a.dev/'), {
    title: 'A > B',
    description: 'x > "y"',
    siteName: 'plain',
    image: 'https://cdn.example.com/a.png',
  });
});

test('parseLinkMetadata falls back to the document title', () => {
  const metadata = parseLinkMetadata('<title> Hi </title>', 'https://a.dev');
  assert.equal(metadata.title, 'Hi');
});

test('service returns a loading placeholder, loads once, and reports settlement', async () => {
  let fetches = 0;
  const settled: string[] = [];
  const service = createLinkPreviewService({
    fetchHtml: async () => {
      fetches++;
      return {
        html: '<meta property="og:title" content="Hello">',
        finalUrl: 'https://a.dev/',
      };
    },
    onSettled: (url) => settled.push(url),
  });
  assert.equal(service.peek('https://a.dev/').status, 'loading');
  service.load('https://a.dev/');
  service.load('https://a.dev/');
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(fetches, 1);
  assert.deepEqual(settled, ['https://a.dev/']);
  assert.equal(service.peek('https://a.dev/').title, 'Hello');
});

test('service marks failures unavailable without retrying', async () => {
  let fetches = 0;
  const service = createLinkPreviewService({
    fetchHtml: async () => {
      fetches++;
      throw new Error('offline');
    },
  });
  service.load('https://a.dev/');
  await new Promise((resolve) => setImmediate(resolve));
  service.load('https://a.dev/');
  assert.equal(service.peek('https://a.dev/').status, 'unavailable');
  assert.equal(fetches, 1);
});

test('service shows metadata first, then swaps in the host-fetched image', async () => {
  const settled: Array<string | undefined> = [];
  let service: ReturnType<typeof createLinkPreviewService>;
  service = createLinkPreviewService({
    fetchHtml: async () => ({
      html: '<meta property="og:title" content="Hi"><meta property="og:image" content="https://cdn.dev/a.png">',
      finalUrl: 'https://a.dev/',
    }),
    fetchImage: async (url) => {
      assert.equal(url, 'https://cdn.dev/a.png');
      return 'data:image/png;base64,AAAA';
    },
    onSettled: (url) => settled.push(service.peek(url).image),
  });
  service.load('https://a.dev/');
  await new Promise((resolve) => setTimeout(resolve, 10));
  assert.deepEqual(settled, [undefined, 'data:image/png;base64,AAAA']);
});

test('service never exposes a remote image URL to the renderer', async () => {
  const service = createLinkPreviewService({
    fetchHtml: async () => ({
      html: '<meta property="og:title" content="Hi"><meta property="og:image" content="http://127.0.0.1/x">',
      finalUrl: 'https://a.dev/',
    }),
    fetchImage: async () => null,
  });
  service.load('https://a.dev/');
  await new Promise((resolve) => setTimeout(resolve, 10));
  assert.equal(service.peek('https://a.dev/').image, undefined);
  assert.equal(service.peek('https://a.dev/').title, 'Hi');
});
