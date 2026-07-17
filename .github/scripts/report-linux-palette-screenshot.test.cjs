'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const test = require('node:test');
const zlib = require('node:zlib');
const {
  crc32,
  expectedHeight,
  expectedWidth,
  findScreenshotArtifact,
  marker,
  publishScreenshotReport,
  upsertReportComment,
  validatePng,
} = require('./report-linux-palette-screenshot.cjs');

function pngChunk(type, data) {
  const typeBuffer = Buffer.from(type, 'ascii');
  const chunk = Buffer.alloc(data.length + 12);
  chunk.writeUInt32BE(data.length, 0);
  typeBuffer.copy(chunk, 4);
  data.copy(chunk, 8);
  chunk.writeUInt32BE(
    crc32(Buffer.concat([typeBuffer, data])),
    data.length + 8,
  );
  return chunk;
}

function createValidScreenshot({
  width = expectedWidth,
  height = expectedHeight,
} = {}) {
  const header = Buffer.alloc(13);
  header.writeUInt32BE(width, 0);
  header.writeUInt32BE(height, 4);
  header.set([8, 6, 0, 0, 0], 8);
  const rows = Buffer.alloc((width * 4 + 1) * height);
  return Buffer.concat([
    Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]),
    pngChunk('IHDR', header),
    pngChunk('IDAT', zlib.deflateSync(rows)),
    pngChunk('IEND', Buffer.alloc(0)),
  ]);
}

function createGithubFixture() {
  const comments = [];
  let storedScreenshot;
  let nextCommentId = 1;
  const storageWrites = [];
  const github = {
    paginate: async (method, options) => method(options),
    rest: {
      actions: {
        listWorkflowRunArtifacts: async () => [],
      },
      git: {
        getRef: async () => ({ data: { object: { sha: 'default-sha' } } }),
        createRef: async () => undefined,
      },
      issues: {
        listComments: async () => comments,
        createComment: async ({ body }) => {
          comments.push({
            body,
            id: nextCommentId,
            user: { login: 'github-actions[bot]' },
          });
          nextCommentId += 1;
        },
        updateComment: async ({ body, comment_id: commentId }) => {
          comments.find((comment) => comment.id === commentId).body = body;
        },
        deleteComment: async ({ comment_id: commentId }) => {
          comments.splice(
            comments.findIndex((comment) => comment.id === commentId),
            1,
          );
        },
      },
      pulls: {
        get: async () => ({
          data: {
            base: { repo: { full_name: 'pablopunk/nvm' } },
            head: { sha: 'a'.repeat(40) },
          },
        }),
      },
      repos: {
        get: async () => ({ data: { default_branch: 'main' } }),
        getContent: async () => {
          if (!storedScreenshot)
            throw Object.assign(new Error('missing'), { status: 404 });
          return { data: { sha: storedScreenshot.sha } };
        },
        createOrUpdateFileContents: async (request) => {
          storageWrites.push(request);
          storedScreenshot = {
            content: request.content,
            sha: `sha-${storageWrites.length}`,
          };
        },
      },
    },
  };
  return { comments, github, storageWrites };
}

test('accepts a complete, decodable screenshot with the expected dimensions', () => {
  assert.deepEqual(validatePng(createValidScreenshot()), {
    height: expectedHeight,
    width: expectedWidth,
  });
});

test('rejects a signature-only, corrupt, or wrong-sized PNG', () => {
  assert.throws(
    () => validatePng(Buffer.from([137, 80, 78, 71, 13, 10, 26, 10])),
    /missing required|truncated/,
  );
  const corrupt = createValidScreenshot();
  corrupt[corrupt.length - 8] ^= 1;
  assert.throws(() => validatePng(corrupt), /CRC|IEND|decoded/);
  assert.throws(
    () => validatePng(createValidScreenshot({ width: 911 })),
    /Unexpected PNG dimensions/,
  );
});

test('treats a successful run without a screenshot as a clean no-op', async () => {
  const core = { notice: () => undefined };
  const { github } = createGithubFixture();
  assert.equal(
    await findScreenshotArtifact({
      core,
      github,
      owner: 'pablopunk',
      repo: 'nvm',
      runId: 123,
    }),
    null,
  );
});

test('serialized same-head reruns update storage and converge on one comment', async (t) => {
  const temporaryDirectory = fs.mkdtempSync(
    path.join(os.tmpdir(), 'nvm-report-'),
  );
  t.after(() =>
    fs.rmSync(temporaryDirectory, { recursive: true, force: true }),
  );
  const screenshotPath = path.join(temporaryDirectory, 'linux-palette.png');
  fs.writeFileSync(screenshotPath, createValidScreenshot());
  const fixture = createGithubFixture();
  const environment = {
    PR_NUMBER: '134',
    RUN_HEAD_SHA: 'a'.repeat(40),
    RUN_URL: 'https://github.com/pablopunk/nvm/actions/runs/123',
    SCREENSHOT_PATH: screenshotPath,
  };
  const input = {
    core: { notice: () => undefined },
    env: environment,
    github: fixture.github,
    owner: 'pablopunk',
    repo: 'nvm',
  };

  await publishScreenshotReport(input);
  await publishScreenshotReport(input);

  assert.equal(fixture.comments.length, 1);
  assert.match(fixture.comments[0].body, new RegExp(marker));
  assert.equal(fixture.storageWrites.length, 2);
  assert.equal(fixture.storageWrites[0].sha, undefined);
  assert.equal(fixture.storageWrites[1].sha, 'sha-1');
});

test('duplicate marker-owned comments are collapsed while human comments remain', async () => {
  const fixture = createGithubFixture();
  fixture.comments.push(
    { body: marker, id: 1, user: { login: 'github-actions[bot]' } },
    { body: marker, id: 2, user: { login: 'github-actions[bot]' } },
    { body: marker, id: 3, user: { login: 'human' } },
  );
  await upsertReportComment({
    body: `${marker}\nupdated`,
    github: fixture.github,
    owner: 'pablopunk',
    prNumber: 134,
    repo: 'nvm',
  });
  assert.equal(
    fixture.comments.filter(
      (comment) => comment.user.login === 'github-actions[bot]',
    ).length,
    1,
  );
  assert.equal(
    fixture.comments.some((comment) => comment.user.login === 'human'),
    true,
  );
});
