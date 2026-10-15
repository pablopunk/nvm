// biome-ignore-all lint: Desktop selection readers moved verbatim from main.ts keep their async signatures.
export type DesktopSelectionDeps = {
  selectedFilePaths: () => Promise<string[]>;
  fileToExtensionFile: (filePath: string) => Promise<unknown>;
  selectedText: () => Promise<unknown>;
  frontmostApp: () => Promise<unknown>;
};

export function createDesktopSelection(deps: DesktopSelectionDeps) {
  const { selectedFilePaths, fileToExtensionFile, selectedText, frontmostApp } =
    deps;

  async function selectedFiles() {
    return selectedFilePaths();
  }

  async function selectedExtensionFiles() {
    const paths = await selectedFilePaths();
    return Promise.all(paths.map(fileToExtensionFile));
  }

  async function readDesktopSelection() {
    const [text, files, app] = await Promise.all([
      selectedText(),
      selectedExtensionFiles(),
      frontmostApp(),
    ]);
    return { text, files, sourceApp: app };
  }

  return { selectedFiles, selectedExtensionFiles, readDesktopSelection };
}
