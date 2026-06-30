import fs from 'node:fs/promises';
import path from 'node:path';

function addIcnsExtension(name: string) {
  return name.toLowerCase().endsWith('.icns') ? name : `${name}.icns`;
}

function unique(values: string[]) {
  return [...new Set(values.filter(Boolean))];
}

export function appBundleIconFileNames(infoPlist: string) {
  const names: string[] = [];
  const directIconFile = infoPlist.match(
    /<key>CFBundleIconFile<\/key>\s*<string>([^<]+)<\/string>/,
  )?.[1];
  if (directIconFile) names.push(directIconFile.trim());

  for (const arrayMatch of infoPlist.matchAll(
    /<key>CFBundleIconFiles<\/key>\s*<array>([\s\S]*?)<\/array>/g,
  )) {
    for (const itemMatch of arrayMatch[1].matchAll(
      /<string>([^<]+)<\/string>/g,
    ))
      names.push(itemMatch[1].trim());
  }

  return unique(names.flatMap((name) => [name, addIcnsExtension(name)]));
}

export function pngImagesFromIcns(icns: Buffer) {
  if (icns.toString('ascii', 0, 4) !== 'icns') return [] as Buffer[];
  const images: Buffer[] = [];
  let offset = 8;
  while (offset + 8 <= icns.length) {
    const size = icns.readUInt32BE(offset + 4);
    if (size < 8 || offset + size > icns.length) break;
    const data = icns.subarray(offset + 8, offset + size);
    if (
      data
        .subarray(0, 8)
        .equals(Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]))
    )
      images.push(Buffer.from(data));
    offset += size;
  }
  return images;
}

function pngArea(png: Buffer) {
  if (png.length < 24) return 0;
  return png.readUInt32BE(16) * png.readUInt32BE(20);
}

export async function findAppBundleIconPath(appPath: string) {
  if (!appPath.endsWith('.app')) return null;
  const resourcesPath = path.join(appPath, 'Contents', 'Resources');
  const [infoPlist, resources] = await Promise.all([
    fs
      .readFile(path.join(appPath, 'Contents', 'Info.plist'), 'utf8')
      .catch(() => ''),
    fs.readdir(resourcesPath).catch(() => [] as string[]),
  ]);
  if (!resources.length) return null;

  const resourcesByLowerName = new Map(
    resources.map((name) => [name.toLowerCase(), name] as const),
  );
  for (const candidate of appBundleIconFileNames(infoPlist)) {
    const resourceName = resourcesByLowerName.get(candidate.toLowerCase());
    if (resourceName) return path.join(resourcesPath, resourceName);
  }

  const fallback = resources.find((name) =>
    name.toLowerCase().endsWith('.icns'),
  );
  return fallback ? path.join(resourcesPath, fallback) : null;
}

export async function readAppBundleIconPng(appPath: string) {
  const iconPath = await findAppBundleIconPath(appPath);
  if (!iconPath?.toLowerCase().endsWith('.icns')) return null;
  const images = pngImagesFromIcns(await fs.readFile(iconPath));
  return images.sort((a, b) => pngArea(b) - pngArea(a))[0] || null;
}
