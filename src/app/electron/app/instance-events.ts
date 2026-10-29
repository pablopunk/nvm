type InstanceEventDeps = {
  app: { on: (event: string, listener: (...args: any[]) => void) => unknown };
  scheme: string;
  testMode: boolean;
  isReady: () => boolean;
  showPalette: () => void;
  processLink: (url: string) => void;
  bufferedLinks: string[];
};

export function registerInstanceEvents(deps: InstanceEventDeps) {
  function receiveLink(url: string) {
    if (deps.isReady()) deps.processLink(url);
    else deps.bufferedLinks.push(url);
  }
  if (!deps.testMode)
    deps.app.on('second-instance', (_event, argv: string[]) => {
      deps.showPalette();
      const link = argv?.find((arg) => arg.startsWith(`${deps.scheme}://`));
      if (link) receiveLink(link);
    });
  deps.app.on('open-url', (_event, url: string) => receiveLink(url));
}
