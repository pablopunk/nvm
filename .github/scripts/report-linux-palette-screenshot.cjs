'use strict';

const fs = require('node:fs');
const path = require('node:path');
const zlib = require('node:zlib');

const marker = '<!-- linux-palette-smoke -->';
const storageBranch = 'ci-screenshots';
const expectedWidth = 912;
const expectedHeight = 672;
const maximumPngBytes = 5 * 1024 * 1024;
const pngSignature = Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]);

const crcTable = Array.from({ length: 256 }, (_, value) => {
  let crc = value;
  for (let bit = 0; bit < 8; bit += 1)
    crc = crc & 1 ? 0xedb88320 ^ (crc >>> 1) : crc >>> 1;
  return crc >>> 0;
});

function crc32(buffer) {
  let crc = 0xffffffff;
  for (const byte of buffer) crc = crcTable[(crc ^ byte) & 0xff] ^ (crc >>> 8);
  return (crc ^ 0xffffffff) >>> 0;
}

function validatePng(screenshot) {
  if (screenshot.length > maximumPngBytes)
    throw new Error(`Unexpected screenshot size: ${screenshot.length}`);
  if (
    screenshot.length < pngSignature.length ||
    !screenshot.subarray(0, pngSignature.length).equals(pngSignature)
  )
    throw new Error('Downloaded artifact is not a PNG');

  let offset = pngSignature.length;
  let imageHeader;
  let sawImageData = false;
  let sawImageEnd = false;
  let imageDataEnded = false;
  const compressedImageData = [];

  while (offset < screenshot.length) {
    if (offset + 12 > screenshot.length)
      throw new Error('PNG contains a truncated chunk header');
    const chunkLength = screenshot.readUInt32BE(offset);
    const chunkEnd = offset + 12 + chunkLength;
    if (chunkEnd > screenshot.length)
      throw new Error('PNG contains a truncated chunk');
    const chunkType = screenshot.toString('ascii', offset + 4, offset + 8);
    if (!/^[A-Za-z]{4}$/.test(chunkType))
      throw new Error(`PNG contains an invalid chunk type: ${chunkType}`);
    const chunkData = screenshot.subarray(offset + 8, offset + 8 + chunkLength);
    const expectedCrc = screenshot.readUInt32BE(offset + 8 + chunkLength);
    const actualCrc = crc32(
      screenshot.subarray(offset + 4, offset + 8 + chunkLength),
    );
    if (actualCrc !== expectedCrc)
      throw new Error(`PNG ${chunkType} chunk failed CRC validation`);

    if (!imageHeader && chunkType !== 'IHDR')
      throw new Error('PNG must begin with an IHDR chunk');
    if (chunkType === 'IHDR') {
      if (imageHeader || chunkLength !== 13)
        throw new Error('PNG must contain exactly one valid IHDR chunk');
      imageHeader = {
        width: chunkData.readUInt32BE(0),
        height: chunkData.readUInt32BE(4),
        bitDepth: chunkData[8],
        colorType: chunkData[9],
        compression: chunkData[10],
        filter: chunkData[11],
        interlace: chunkData[12],
      };
      if (
        imageHeader.width !== expectedWidth ||
        imageHeader.height !== expectedHeight
      )
        throw new Error(
          `Unexpected PNG dimensions: ${imageHeader.width}x${imageHeader.height}`,
        );
      if (
        imageHeader.bitDepth !== 8 ||
        imageHeader.colorType !== 6 ||
        imageHeader.compression !== 0 ||
        imageHeader.filter !== 0 ||
        imageHeader.interlace !== 0
      )
        throw new Error('PNG must be a non-interlaced 8-bit RGBA image');
    } else if (chunkType === 'IDAT') {
      if (imageDataEnded)
        throw new Error('PNG IDAT chunks must be consecutive');
      sawImageData = true;
      compressedImageData.push(chunkData);
    } else if (sawImageData) {
      imageDataEnded = true;
    }

    offset = chunkEnd;
    if (chunkType === 'IEND') {
      if (chunkLength !== 0 || offset !== screenshot.length)
        throw new Error('PNG must end with one empty IEND chunk');
      sawImageEnd = true;
      break;
    }
  }

  if (!imageHeader || !sawImageData || !sawImageEnd)
    throw new Error('PNG is missing required IHDR, IDAT, or IEND chunks');

  const rowBytes = imageHeader.width * 4;
  const expectedInflatedBytes = (rowBytes + 1) * imageHeader.height;
  let pixels;
  try {
    pixels = zlib.inflateSync(Buffer.concat(compressedImageData), {
      maxOutputLength: expectedInflatedBytes,
    });
  } catch (error) {
    throw new Error(`PNG image data cannot be decoded: ${error.message}`);
  }
  if (pixels.length !== expectedInflatedBytes)
    throw new Error(`Unexpected decoded PNG size: ${pixels.length}`);
  for (let row = 0; row < imageHeader.height; row += 1) {
    const filterType = pixels[row * (rowBytes + 1)];
    if (filterType > 4)
      throw new Error(`Invalid PNG row filter: ${filterType}`);
  }

  return { height: imageHeader.height, width: imageHeader.width };
}

async function findScreenshotArtifact({ core, github, owner, repo, runId }) {
  const artifacts = await github.paginate(
    github.rest.actions.listWorkflowRunArtifacts,
    {
      owner,
      repo,
      run_id: runId,
      per_page: 100,
    },
  );
  const screenshots = artifacts.filter(
    (artifact) => artifact.name === 'linux-palette-screenshot',
  );
  if (screenshots.length === 0) {
    core.notice('No Linux palette screenshot; smoke was not applicable');
    return null;
  }
  if (screenshots.length !== 1)
    throw new Error(
      `Expected one screenshot artifact, found ${screenshots.length}`,
    );
  if (screenshots[0].expired)
    throw new Error('Screenshot artifact has expired');
  return screenshots[0];
}

async function ensureStorageBranch({ github, owner, repo }) {
  try {
    await github.rest.git.getRef({
      owner,
      repo,
      ref: `heads/${storageBranch}`,
    });
    return;
  } catch (error) {
    if (error.status !== 404) throw error;
  }

  const { data: repository } = await github.rest.repos.get({ owner, repo });
  const { data: defaultBranch } = await github.rest.git.getRef({
    owner,
    repo,
    ref: `heads/${repository.default_branch}`,
  });
  try {
    await github.rest.git.createRef({
      owner,
      repo,
      ref: `refs/heads/${storageBranch}`,
      sha: defaultBranch.object.sha,
    });
  } catch (error) {
    if (error.status !== 422) throw error;
  }
}

async function storeScreenshot({ github, owner, repo, prNumber, screenshot }) {
  await ensureStorageBranch({ github, owner, repo });
  const screenshotFile = `pr-${prNumber}/linux-palette.png`;
  let existingSha;
  try {
    const { data: existing } = await github.rest.repos.getContent({
      owner,
      repo,
      path: screenshotFile,
      ref: storageBranch,
    });
    if (!Array.isArray(existing)) existingSha = existing.sha;
  } catch (error) {
    if (error.status !== 404) throw error;
  }
  await github.rest.repos.createOrUpdateFileContents({
    owner,
    repo,
    branch: storageBranch,
    path: screenshotFile,
    message: `CI: update Linux palette screenshot for PR #${prNumber}`,
    content: screenshot.toString('base64'),
    sha: existingSha,
  });
  return screenshotFile;
}

async function upsertReportComment({ github, owner, repo, prNumber, body }) {
  const comments = await github.paginate(github.rest.issues.listComments, {
    owner,
    repo,
    issue_number: prNumber,
    per_page: 100,
  });
  const reportComments = comments.filter(
    (comment) =>
      comment.user?.login === 'github-actions[bot]' &&
      comment.body?.includes(marker),
  );
  if (reportComments.length === 0) {
    await github.rest.issues.createComment({
      owner,
      repo,
      issue_number: prNumber,
      body,
    });
    return;
  }

  await github.rest.issues.updateComment({
    owner,
    repo,
    comment_id: reportComments[0].id,
    body,
  });
  for (const comment of reportComments.slice(1)) {
    await github.rest.issues.deleteComment({
      owner,
      repo,
      comment_id: comment.id,
    });
  }
}

async function publishScreenshotReport({ core, github, owner, repo, env }) {
  const prNumber = Number(env.PR_NUMBER);
  const runHeadSha = env.RUN_HEAD_SHA;
  const runUrl = env.RUN_URL;
  if (!Number.isInteger(prNumber) || prNumber < 1)
    throw new Error(`Invalid PR number: ${env.PR_NUMBER}`);
  if (!/^[0-9a-f]{40}$/i.test(runHeadSha))
    throw new Error(`Invalid workflow head SHA: ${runHeadSha}`);
  if (!/^https:\/\/github\.com\//.test(runUrl))
    throw new Error(`Invalid workflow URL: ${runUrl}`);

  const { data: pullRequest } = await github.rest.pulls.get({
    owner,
    repo,
    pull_number: prNumber,
  });
  if (pullRequest.base.repo?.full_name !== `${owner}/${repo}`)
    throw new Error('Pull request does not target this repository');
  if (pullRequest.head.sha !== runHeadSha) {
    core.notice('Skipping a stale screenshot run');
    return { status: 'stale' };
  }

  const screenshotPath = path.resolve(env.SCREENSHOT_PATH);
  const screenshotStats = fs.lstatSync(screenshotPath);
  if (!screenshotStats.isFile() || screenshotStats.isSymbolicLink())
    throw new Error('Downloaded screenshot must be a regular file');
  const screenshot = fs.readFileSync(screenshotPath);
  validatePng(screenshot);

  const screenshotFile = await storeScreenshot({
    github,
    owner,
    repo,
    prNumber,
    screenshot,
  });
  const imageUrl = `https://raw.githubusercontent.com/${owner}/${repo}/${storageBranch}/${screenshotFile}?sha=${runHeadSha}`;
  const body = [
    marker,
    '## Linux palette smoke',
    '',
    `![Passing Linux palette smoke](${imageUrl})`,
    '',
    `Captured from [passing CI run](${runUrl}) at \`${runHeadSha}\`.`,
  ].join('\n');
  await upsertReportComment({ github, owner, repo, prNumber, body });
  return { status: 'published' };
}

module.exports = {
  crc32,
  expectedHeight,
  expectedWidth,
  findScreenshotArtifact,
  marker,
  publishScreenshotReport,
  storeScreenshot,
  upsertReportComment,
  validatePng,
};
