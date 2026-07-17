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
  const status = entry.proposal
    ? entry.enabled
      ? 'Pending Update'
      : 'Pending'
    : entry.enabled
      ? 'Enabled'
      : 'Disabled';
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
    content: `# ${entry.filename}\n\n${EXTENSION_TRUST_DISCLOSURE}\n\n## Declared capabilities\n\n${declared.capabilities.length ? declared.capabilities.map((value) => `- ${value}`).join('\n') : '- Not statically declared'}\n\n## Current Source\n\n\`\`\`ts\n${entry.source || ''}\n\`\`\`${entry.proposal ? `\n\n## Proposed Source\n\n\`\`\`ts\n${entry.proposalSource || ''}\n\`\`\`` : ''}`,
  });
  return {
    id: `extension:${entry.filename}`,
    title: manifest.title || entry.filename,
    subtitle: `${status} · ${declared.provenance}`,
    accessories: [{ text: status }],
    primaryAction: sourceView,
    actionPanel: {
      sections: [
        {
          actions: [
            sourceView,
            entry.enabled ? disable : enable,
            entry.proposal ? enable : null,
            discard,
          ].filter(Boolean),
        },
      ],
    },
  };
}

export function createExtensionsExtension() {
  return {
    id: 'nevermind.extensions',
    title: 'Extensions',
    capabilities: [] as const,
    commands: [
      {
        id: 'extensions',
        title: 'Extensions',
        subtitle: 'Review and manage trusted local extensions',
        icon: 'puzzle',
        run: (ctx) => extensionsView(ctx),
      },
    ],
  };
}
