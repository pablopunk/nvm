const assert = require('node:assert/strict');
const { createServer } = require('node:http');
const fs = require('node:fs/promises');
const { createRequire } = require('node:module');
const os = require('node:os');
const path = require('node:path');
const { fileURLToPath, pathToFileURL } = require('node:url');
const asar = require('@electron/asar');

function sendFixtureResponse(response) {
  response.writeHead(200, {
    'cache-control': 'no-cache',
    connection: 'keep-alive',
    'content-type': 'text/event-stream; charset=utf-8',
  });
  response.end(
    `data: ${JSON.stringify({
      id: 'packaged-pi-fixture',
      object: 'chat.completion.chunk',
      created: 1,
      model: 'packaged-fixture-model',
      choices: [
        {
          index: 0,
          delta: { content: 'Packaged Pi fixture completed.' },
          finish_reason: 'stop',
        },
      ],
    })}\n\ndata: [DONE]\n\n`,
  );
}

function assistantText(messages) {
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const message = messages[index];
    if (message.role !== 'assistant') continue;
    if (typeof message.content === 'string') return message.content;
    if (Array.isArray(message.content))
      return message.content
        .filter((part) => part.type === 'text')
        .map((part) => part.text)
        .join('');
  }
  return '';
}

async function packagedPiArchiveNodeSmoke(archivePath, evidenceDirectory) {
  const temporary = await fs.mkdtemp(
    path.join(os.tmpdir(), 'nevermind-packaged-pi-'),
  );
  const extractionRoot = path.join(temporary, 'application');
  const agentDir = path.join(temporary, 'pi-agent');
  const workspaceDir = path.join(temporary, 'workspace');
  const evidencePath = path.join(
    evidenceDirectory,
    'pi-archive-node-evidence.json',
  );
  await fs.mkdir(evidenceDirectory, { recursive: true });
  await fs.mkdir(extractionRoot, { recursive: true });
  asar.extractAll(path.resolve(archivePath), extractionRoot);

  const manifest = JSON.parse(
    await fs.readFile(path.join(extractionRoot, 'package.json'), 'utf8'),
  );
  const packagedRequire = createRequire(
    path.join(extractionRoot, 'package.json'),
  );
  const esmLoaderPath = path.join(
    extractionRoot,
    'pi-archive-smoke-loader.mjs',
  );
  await fs.writeFile(
    esmLoaderPath,
    [
      'export async function loadPiPackages() {',
      '  return Promise.all([',
      "    import('@earendil-works/pi-coding-agent'),",
      "    import('@earendil-works/pi-ai'),",
      '  ]);',
      '}',
      'export function resolvePiPackages() {',
      '  return {',
      "    codingAgent: import.meta.resolve('@earendil-works/pi-coding-agent'),",
      "    piAi: import.meta.resolve('@earendil-works/pi-ai'),",
      '  };',
      '}',
      '',
    ].join('\n'),
  );
  const esmLoader = await import(pathToFileURL(esmLoaderPath).href);
  const packageEntries = esmLoader.resolvePiPackages();
  const archiveNodeModules = `${path.join(extractionRoot, 'node_modules')}${path.sep}`;
  const piCodingAgentEntry = fileURLToPath(packageEntries.codingAgent);
  const piAiEntry = fileURLToPath(packageEntries.piAi);
  assert.ok(
    piCodingAgentEntry.startsWith(archiveNodeModules),
    'Pi coding agent must resolve from the extracted application archive',
  );
  assert.ok(
    piAiEntry.startsWith(archiveNodeModules),
    'Pi AI must resolve from the extracted application archive',
  );
  const [pi] = await esmLoader.loadPiPackages();
  const webPackagePath = packagedRequire.resolve('pi-web-access/package.json');
  const webPackageRoot = path.dirname(webPackagePath);
  assert.ok(
    webPackagePath.startsWith(archiveNodeModules),
    'Pi web access must resolve from the extracted application archive',
  );
  const packageVersions = {
    'pi-coding-agent': manifest.dependencies['@earendil-works/pi-coding-agent'],
    'pi-ai': manifest.dependencies['@earendil-works/pi-ai'],
    'pi-web-access': manifest.dependencies['pi-web-access'],
  };

  let providerCalls = 0;
  const server = createServer(async (request, response) => {
    for await (const _chunk of request) {
    }
    if (request.url !== '/v1/chat/completions') {
      response.writeHead(404);
      response.end('unknown fixture path');
      return;
    }
    providerCalls += 1;
    sendFixtureResponse(response);
  });
  let session;
  try {
    await fs.mkdir(agentDir, { recursive: true });
    await fs.mkdir(workspaceDir, { recursive: true });
    process.env.PI_CODING_AGENT_DIR = agentDir;
    await new Promise(function listenForFixture(resolve) {
      server.listen(0, '127.0.0.1', resolve);
    });
    const address = server.address();
    assert.ok(address && typeof address !== 'string');
    const baseUrl = `http://127.0.0.1:${address.port}/v1`;
    const modelRuntime = await pi.ModelRuntime.create({
      authPath: path.join(agentDir, 'auth.json'),
      modelsPath: null,
      allowModelNetwork: false,
    });
    modelRuntime.registerProvider('packaged-fixture', {
      api: 'openai-completions',
      baseUrl,
      models: [
        {
          id: 'packaged-fixture-model',
          name: 'Packaged Pi Fixture',
          api: 'openai-completions',
          type: 'chat',
          input: ['text'],
          cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 },
          reasoning: false,
          contextWindow: 8_192,
          maxTokens: 1_024,
        },
      ],
    });
    await modelRuntime.setRuntimeApiKey('packaged-fixture', 'fixture-only');
    const model = modelRuntime.getModel(
      'packaged-fixture',
      'packaged-fixture-model',
    );
    assert.ok(model, 'packaged provider should resolve its fixture model');
    const resourceLoader = new pi.DefaultResourceLoader({
      cwd: workspaceDir,
      agentDir,
      additionalExtensionPaths: [path.dirname(webPackagePath)],
      disabledBuiltinExtensions: ['codemode', 'mcp', 'tool-search'],
      noExtensions: true,
      noSkills: true,
      noPromptTemplates: true,
      noThemes: true,
      noContextFiles: true,
      systemPrompt: 'Return the supplied local fixture response.',
    });
    await resourceLoader.reload();
    const loadedExtensions = resourceLoader.getExtensions();
    assert.equal(loadedExtensions.errors.length, 0);
    const webExtension = loadedExtensions.extensions.find(
      function extensionFromPackagedWebAccess(extension) {
        return [extension.path, extension.resolvedPath].some((candidate) => {
          if (typeof candidate !== 'string') return false;
          const resolvedCandidate = path.isAbsolute(candidate)
            ? candidate
            : path.resolve(webPackageRoot, candidate);
          const relative = path.relative(webPackageRoot, resolvedCandidate);
          return (
            relative === '' ||
            (!path.isAbsolute(relative) &&
              !relative.startsWith(`..${path.sep}`) &&
              relative !== '..')
          );
        });
      },
    );
    assert.ok(
      webExtension,
      'Pi must load the web extension from the extracted app archive',
    );
    const { session: createdSession } = await pi.createAgentSession({
      cwd: workspaceDir,
      agentDir,
      model,
      modelRuntime,
      resourceLoader,
      tools: [],
      noTools: 'all',
      sessionManager: pi.SessionManager.inMemory(workspaceDir),
      settingsManager: pi.SettingsManager.inMemory({
        compaction: { enabled: true, reserveTokens: 1_024 },
        retry: { enabled: false },
      }),
    });
    session = createdSession;
    await session.prompt('Run the credential-free packaged fixture.');
    assert.match(
      assistantText(session.agent.state.messages),
      /Packaged Pi fixture completed\./,
    );
    assert.equal(providerCalls, 1);
    assert.equal(session.getSessionStats().assistantMessages, 1);
    const evidence = {
      packageVersions,
      resolvedPackageEntries: {
        'pi-coding-agent': path.relative(extractionRoot, piCodingAgentEntry),
        'pi-ai': path.relative(extractionRoot, piAiEntry),
        'pi-web-access': path.relative(extractionRoot, webPackagePath),
      },
      loadedPiPackages: ['pi-coding-agent', 'pi-ai'],
      loadedWebExtensionPath: path.relative(
        extractionRoot,
        path.resolve(webExtension.resolvedPath || webExtension.path),
      ),
      runtimeBoundary: 'stock-node-session-from-extracted-app-asar',
      runtime: 'host-node',
      electronRuntime: false,
      adapterIntegration: false,
      fixtureSession: 'success',
      providerCalls,
      provider: 'loopback-openai-completions',
      architecture: `${process.platform}-${process.arch}`,
      commitSha: process.env.GITHUB_SHA || 'local',
    };
    await fs.writeFile(evidencePath, `${JSON.stringify(evidence, null, 2)}\n`);
    console.log(JSON.stringify(evidence));
  } finally {
    await session?.dispose();
    await new Promise((resolve) => server.close(resolve));
    await fs.rm(temporary, { recursive: true, force: true });
  }
}

if (require.main === module) {
  packagedPiArchiveNodeSmoke(process.argv[2], process.argv[3]).catch(
    function reportPackagedPiArchiveNodeFailure(error) {
      console.error(error);
      process.exitCode = 1;
    },
  );
}

module.exports = { packagedPiArchiveNodeSmoke };
