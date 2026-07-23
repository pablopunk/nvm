import {
  declaredExtensionCapabilities,
  EXTENSION_TRUST_DISCLOSURE,
} from '../extension-capabilities';
import { inspectExtensionManifest } from '../extension-manifest';
import { extensionContext } from './_context';

async function extensionsView(ctx) {
  const entries = await extensionContext.extensionManager.list();
  return ctx.ui.list({
    id: 'trusted-extensions',
    title: 'Extensions',
    subtitle: EXTENSION_TRUST_DISCLOSURE,
    emptyView: { title: 'No local extensions found.' },
    items: entries.map((entry) => extensionItem(ctx, entry)),
  });
}

function extensionStatus(entry) {
  if (entry.proposal) return entry.enabled ? 'Pending Update' : 'Pending';
  return entry.enabled ? 'Enabled' : 'Disabled';
}

function extensionSourceContent(entry, declared) {
  const capabilities = declared.capabilities.length
    ? declared.capabilities.map((value) => `- ${value}`).join('\n')
    : '- Not statically declared';
  const currentSource = `## Current Source\n\n\`\`\`ts\n${entry.source || ''}\n\`\`\``;
  const proposedSource = `## Proposed Source\n\n\`\`\`ts\n${entry.proposalSource || ''}\n\`\`\``;
  return `# ${entry.filename}\n\n${EXTENSION_TRUST_DISCLOSURE}\n\n## Declared capabilities\n\n${capabilities}\n\n${entry.proposal ? `${proposedSource}\n\n${currentSource}` : currentSource}`;
}

function extensionItem(ctx, entry) {
  const source = entry.proposal ? entry.proposalSource : entry.source;
  const manifest = inspectExtensionManifest(source || '');
  const declared = declaredExtensionCapabilities({
    capabilities:
      manifest.provenance === 'capabilities'
        ? manifest.capabilities
        : undefined,
    permissions:
      manifest.provenance === 'legacy-permissions'
        ? manifest.capabilities
        : undefined,
  });
  const status = extensionStatus(entry);
  const refresh = async () => ({
    view: await extensionsView(ctx),
    navigation: 'replace' as const,
  });
  const enable = ctx.actions.run(
    entry.proposal && entry.enabled ? 'Apply Update' : 'Enable',
    async () => {
      await extensionContext.extensionManager.enable(entry.filename);
      return refresh();
    },
  );
  const disable = ctx.actions.run('Disable', async () => {
    await extensionContext.extensionManager.disable(entry.filename);
    return refresh();
  });
  const discard = entry.proposal
    ? ctx.actions.run('Discard Proposal', async () => {
        await extensionContext.extensionManager.discard(entry.filename);
        return refresh();
      })
    : null;
  const sourceView = ctx.actions.push('View Source', {
    type: 'preview',
    title: entry.filename,
    content: extensionSourceContent(entry, declared),
  });
  return {
    id: `extension:${entry.filename}`,
    title: entry.filename,
    subtitle: `${status} · ${declared.provenance === 'undeclared' ? 'Capabilities undeclared' : `${declared.capabilities.length} declared capabilities`}`,
    icon: entry.enabled ? 'check' : 'file-text',
    primaryAction: entry.proposal || !entry.enabled ? enable : sourceView,
    actions: [sourceView, entry.enabled ? disable : enable, discard].filter(
      Boolean,
    ),
  };
}

export function createExtensionsExtension() {
  return {
    id: 'nevermind.extensions',
    title: 'Extensions',
    capabilities: ['extensions.manage'] as const,
    commands: [
      {
        id: 'manage',
        title: 'Extensions',
        subtitle: 'Review, enable, disable, or update local extensions',
        icon: 'file-text',
        run: (ctx) => extensionsView(ctx),
      },
    ],
  };
}
