const assert = require('node:assert/strict');
const { createServer } = require('node:http');
const fs = require('node:fs/promises');
const { createRequire } = require('node:module');
const os = require('node:os');
const path = require('node:path');
const { pathToFileURL } = require('node:url');
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

async function packagedPiSessionSmoke(archivePath, evidenceDirectory) {
  const temporary = await fs.mkdtemp(
    path.join(os.tmpdir(), 'nevermind-packaged-pi-'),
  );
  const extractionRoot = path.join(temporary, 'application');
  const agentDir = path.join(temporary, 'pi-agent');
  const workspaceDir = path.join(temporary, 'workspace');
  await fs.mkdir(evidenceDirectory, { recursive: true });
  await fs.mkdir(extractionRoot, { recursive: true });
  asar.extractAll(path.resolve(archivePath), extractionRoot);

  const packagedRequire = createRequire(
    path.join(extractionRoot, 'package.json'),
  );
  const piEntryPath = packagedRequire.resolve(
    '@earendil-works/pi-coding-agent',
  );
  const piAiEntryPath = packagedRequire.resolve('@earendil-works/pi-ai');
  const piPackagePath = path.resolve(
    path.dirname(piEntryPath),
    '../package.json',
  );
  const piAiPackagePath = path.resolve(
    path.dirname(piAiEntryPath),
    '../package.json',
  );
  const webPackagePath = packagedRequire.resolve('pi-web-access/package.json');
  const pi = await import(pathToFileURL(piEntryPath).href);
  const piPackage = JSON.parse(await fs.readFile(piPackagePath, 'utf8'));
  const piAiPackage = JSON.parse(await fs.readFile(piAiPackagePath, 'utf8'));
  const webPackage = JSON.parse(await fs.readFile(webPackagePath, 'utf8'));

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
    assert.equal(resourceLoader.getExtensions().errors.length, 0);
    assert.ok(resourceLoader.getExtensions().extensions.length > 0);
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
      packagedImports: {
        'pi-coding-agent': piPackage.version,
        'pi-ai': piAiPackage.version,
        'pi-web-access': webPackage.version,
      },
      fixtureSession: 'success',
      providerCalls,
      provider: 'loopback-openai-completions',
      architecture: `${process.platform}-${process.arch}`,
      commitSha: process.env.GITHUB_SHA || 'local',
    };
    await fs.writeFile(
      path.join(evidenceDirectory, 'pi-session-evidence.json'),
      `${JSON.stringify(evidence, null, 2)}\n`,
    );
    console.log(JSON.stringify(evidence));
  } finally {
    await session?.dispose();
    await new Promise((resolve) => server.close(resolve));
    await fs.rm(temporary, { recursive: true, force: true });
  }
}

if (require.main === module) {
  packagedPiSessionSmoke(process.argv[2], process.argv[3]).catch(
    function reportPackagedPiFailure(error) {
      console.error(error);
      process.exitCode = 1;
    },
  );
}

module.exports = { packagedPiSessionSmoke };
