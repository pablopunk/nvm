import { systemPreferences } from 'electron';

export type OsPermissionState =
  | 'allowed'
  | 'denied'
  | 'not-determined'
  | 'unknown';

export type MediaPermissionKind = 'microphone' | 'camera' | 'screen';

export type PermissionReader = {
  platform: NodeJS.Platform;
  mediaStatus: (kind: MediaPermissionKind) => string;
  accessibilityTrusted: () => boolean;
  requestMediaAccess: (kind: 'microphone' | 'camera') => Promise<boolean>;
  requestAccessibilityAccess: () => boolean;
};

export function electronPermissionReader(): PermissionReader {
  return {
    platform: process.platform,
    mediaStatus: (kind) => {
      try {
        return systemPreferences.getMediaAccessStatus(kind);
      } catch {
        return 'unknown';
      }
    },
    accessibilityTrusted: () => {
      try {
        return systemPreferences.isTrustedAccessibilityClient(false);
      } catch {
        return false;
      }
    },
    requestMediaAccess: (kind) => systemPreferences.askForMediaAccess(kind),
    requestAccessibilityAccess: () => {
      try {
        return systemPreferences.isTrustedAccessibilityClient(true);
      } catch {
        return false;
      }
    },
  };
}

export function normalizeMediaPermissionState(
  status: unknown,
): OsPermissionState {
  if (status === 'granted') return 'allowed';
  if (status === 'denied' || status === 'restricted') return 'denied';
  if (status === 'not-determined') return 'not-determined';
  return 'unknown';
}

export type OsPermissionRow = {
  key: string;
  title: string;
  description: string;
  icon: string;
  anchor: string;
  state: OsPermissionState;
  canRequest: boolean;
};

export function readOsPermissions(
  reader: PermissionReader = electronPermissionReader(),
): OsPermissionRow[] {
  const rows: OsPermissionRow[] = [
    {
      key: 'microphone',
      title: 'Microphone',
      description: 'Voice dictation input',
      icon: 'mic',
      anchor: 'Microphone',
      state: normalizeMediaPermissionState(reader.mediaStatus('microphone')),
      canRequest: true,
    },
    {
      key: 'camera',
      title: 'Camera',
      description: 'Extension live camera views',
      icon: 'camera',
      anchor: 'Camera',
      state: normalizeMediaPermissionState(reader.mediaStatus('camera')),
      canRequest: true,
    },
  ];
  if (reader.platform === 'darwin') {
    rows.push(
      {
        key: 'accessibility',
        title: 'Accessibility',
        description: 'Paste text and read selected text',
        icon: 'accessibility',
        anchor: 'Accessibility',
        state: reader.accessibilityTrusted() ? 'allowed' : 'denied',
        canRequest: true,
      },
      {
        key: 'screen-recording',
        title: 'Screen Recording',
        description: 'Screen capture for OCR and images',
        icon: 'monitor',
        anchor: 'ScreenCapture',
        state: normalizeMediaPermissionState(reader.mediaStatus('screen')),
        canRequest: false,
      },
      {
        key: 'files-and-folders',
        title: 'Files & Folders',
        description: 'App indexing and file search',
        icon: 'folder',
        anchor: 'FilesAndFolders',
        state: 'unknown',
        canRequest: false,
      },
    );
  }
  const attentionFirst = (row: OsPermissionRow) =>
    row.state === 'denied' || row.state === 'not-determined' ? 0 : 1;
  return [...rows].sort((a, b) => attentionFirst(a) - attentionFirst(b));
}

const PRIVACY_PANE_ID = 'com.apple.settings.PrivacySecurity.extension';

function permissionSubtitle(row: OsPermissionRow) {
  switch (row.state) {
    case 'allowed':
      return 'Allowed — press Enter to review in Settings';
    case 'denied':
      return 'Not allowed — press Enter to open Settings';
    case 'not-determined':
      return 'Not asked yet — press Enter to allow';
    default:
      return 'Review in Settings — press Enter to open';
  }
}

export async function permissionsView(ctx: any, reader?: PermissionReader) {
  const rows = readOsPermissions(reader);
  const known = rows.filter((row) => row.state !== 'unknown');
  const allowed = known.filter((row) => row.state === 'allowed').length;

  return ctx.ui.list({
    id: 'os-permissions',
    title: 'Nevermind OS Permissions',
    subtitle:
      known.length === 0
        ? 'Review system access'
        : `${allowed} of ${known.length} allowed`,
    searchBarPlaceholder: 'Search permissions',
    emptyView: {
      title: 'No permissions',
      subtitle: 'No OS permissions apply on this platform.',
    },
    items: rows.map((row) => {
      const openSettings = ctx.actions.system.openSystemSettings(row.title, {
        paneId: PRIVACY_PANE_ID,
        anchor: row.anchor,
      });
      const needsPrompt =
        (row.canRequest &&
          row.state === 'not-determined' &&
          row.key !== 'accessibility') ||
        (row.key === 'accessibility' && row.state !== 'allowed');
      const primaryAction = !needsPrompt
        ? openSettings
        : ctx.actions.run(`Allow ${row.title}`, async (innerCtx: any) => {
            const activeReader = reader ?? electronPermissionReader();
            if (row.key === 'accessibility')
              activeReader.requestAccessibilityAccess();
            else
              await activeReader.requestMediaAccess(
                row.key as 'microphone' | 'camera',
              );
            return {
              view: await permissionsView(innerCtx, reader),
              navigation: 'replace',
            };
          });
      return {
        id: `os-permission:${row.key}`,
        title: row.title,
        subtitle: permissionSubtitle(row),
        icon: row.icon,
        primaryAction,
        actions: needsPrompt ? [openSettings] : [],
      };
    }),
  });
}

function permissionsRootItem(ctx: any) {
  const openPermissions = ctx.actions.run(
    'Open OS Permissions',
    async (innerCtx: any) => ({
      view: await permissionsView(innerCtx),
      navigation: 'push',
    }),
  );
  return {
    id: 'os-permissions',
    title: 'Nevermind OS Permissions',
    subtitle: 'Review microphone, camera, and system access',
    icon: 'shield-check',
    aliases: [
      'os permissions',
      'permissions',
      'microphone',
      'camera',
      'privacy',
    ],
    primaryAction: openPermissions,
  };
}

export function createPermissionsExtension() {
  return {
    id: 'nevermind.permissions',
    title: 'OS Permissions',
    subtitle: 'Review system access for Nevermind features',
    capabilities: ['system'] as const,
    actions(_ctx: any) {
      return [];
    },
    rootItems(ctx: any) {
      return [permissionsRootItem(ctx)];
    },
    searchItems(ctx: any, query: string) {
      const item = permissionsRootItem(ctx);
      const normalized = String(query || '')
        .trim()
        .toLowerCase();
      if (!normalized) return [item];
      const haystack = [item.title, item.subtitle, ...(item.aliases || [])]
        .join(' ')
        .toLowerCase();
      return normalized.split(/\s+/).every((word) => haystack.includes(word))
        ? [item]
        : [];
    },
  };
}
