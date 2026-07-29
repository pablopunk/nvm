// biome-ignore-all lint/style/useNamingConvention: Launch Services plist keys use Apple's canonical names.
interface OpenWithApp {
  id?: string;
  name: string;
  path?: string;
  [key: string]: unknown;
}

interface AppDocumentType {
  CFBundleTypeExtensions?: unknown[];
  LSItemContentTypes?: unknown[];
}

const EXTENSION_MATCH_SCORE = 3;
const CONTENT_TYPE_MATCH_SCORE = 2;
const GENERIC_MATCH_SCORE = 1;

function documentTypeScore(
  type: AppDocumentType,
  fileExtension: string,
  contentTypes: Set<string>,
) {
  const extensions = (type.CFBundleTypeExtensions || []).map((value) =>
    String(value).toLowerCase(),
  );
  const itemTypes = (type.LSItemContentTypes || []).map(String);
  if (fileExtension && extensions.includes(fileExtension)) {
    return EXTENSION_MATCH_SCORE;
  }
  if (itemTypes.some((itemType) => contentTypes.has(itemType))) {
    return CONTENT_TYPE_MATCH_SCORE;
  }
  if (
    extensions.includes('*') ||
    itemTypes.includes('public.data') ||
    itemTypes.includes('public.item')
  ) {
    return GENERIC_MATCH_SCORE;
  }
  return 0;
}

async function compatibleOpenWithApps(
  candidates: OpenWithApp[],
  fileExtension: string,
  contentTypes: Set<string>,
  documentTypesForApp: (appPath: string) => Promise<AppDocumentType[]>,
) {
  const apps = candidates.filter((item) =>
    item.path?.toLowerCase().endsWith('.app'),
  );
  const scored: Array<OpenWithApp & { score: number }> = [];

  await Promise.all(
    apps.map(async (item) => {
      if (!item.path) {
        return;
      }
      const documentTypes = await documentTypesForApp(item.path);
      const score = Math.max(
        0,
        ...documentTypes.map((type) =>
          documentTypeScore(type, fileExtension, contentTypes),
        ),
      );
      if (score) {
        scored.push({ ...item, score });
      }
    }),
  );

  if (scored.length === 0) {
    return apps.sort((a, b) => a.name.localeCompare(b.name));
  }

  return scored
    .sort((a, b) => b.score - a.score || a.name.localeCompare(b.name))
    .map(({ score: _score, ...item }) => item);
}

export type { OpenWithApp };
export { compatibleOpenWithApps };
