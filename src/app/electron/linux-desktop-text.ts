import {
  assertDesktopTextTarget,
  DesktopTextAccessError,
  type DesktopTextTarget,
  desktopTextFailure,
  findDesktopTextCommand,
  runDesktopTextCommand,
} from './desktop-text-access';

type Backend = 'x11' | 'hyprland' | 'sway';
type WindowNode = {
  id?: number;
  pid?: number;
  focused?: boolean;
  nodes?: WindowNode[];
  floating_nodes?: WindowNode[];
};

function swayWindows(node: WindowNode): WindowNode[] {
  return [
    node,
    ...(node.nodes ?? []).flatMap(swayWindows),
    ...(node.floating_nodes ?? []).flatMap(swayWindows),
  ];
}

function targetForWindow(
  backend: Backend,
  windowId: string,
  pid: number,
): DesktopTextTarget | null {
  if (
    !/^(?:\d+|0x[\da-f]+)$/i.test(windowId) ||
    !Number.isSafeInteger(pid) ||
    pid <= 0
  )
    return null;
  return { bundleId: `linux:${backend}:${windowId}:${pid}`, windowId, pid };
}

export function createLinuxDesktopText(
  dependencies: {
    environment?: NodeJS.ProcessEnv;
    findCommand?: (command: string) => string | null | Promise<string | null>;
    run?: typeof runDesktopTextCommand;
  } = {},
) {
  const environment = dependencies.environment ?? process.env;
  const run = dependencies.run ?? runDesktopTextCommand;
  const findCommand =
    dependencies.findCommand ??
    ((command) => findDesktopTextCommand(command, environment.PATH ?? ''));
  const wayland =
    environment.XDG_SESSION_TYPE === 'wayland' ||
    Boolean(environment.WAYLAND_DISPLAY);
  const backend: Backend | null = !wayland
    ? 'x11'
    : environment.HYPRLAND_INSTANCE_SIGNATURE
      ? 'hyprland'
      : environment.SWAYSOCK
        ? 'sway'
        : null;
  const tools = new Map<string, Promise<string | null>>();
  const requiredTools =
    backend === 'x11'
      ? ['xdotool', 'xclip']
      : backend === 'hyprland'
        ? ['hyprctl']
        : backend === 'sway'
          ? ['swaymsg', 'wtype']
          : [];

  function tool(name: string) {
    if (!tools.has(name)) tools.set(name, Promise.resolve(findCommand(name)));
    return tools.get(name)!;
  }

  async function assertAvailable() {
    if (!backend)
      throw new DesktopTextAccessError(
        'unsupported',
        'This Wayland desktop does not support selected-text control. Use an X11 session, Hyprland, or Sway.',
      );
    if (backend === 'x11' && !environment.DISPLAY)
      throw new DesktopTextAccessError(
        'unavailable',
        'No X11 desktop is connected. Start Nevermind in your desktop session.',
      );
    const paths = await Promise.all(requiredTools.map(tool));
    const missing = requiredTools.filter((_name, index) => !paths[index]);
    if (missing.length)
      throw new DesktopTextAccessError(
        'unavailable',
        `Install ${missing.join(' and ')} to enable selected-text control, then restart Nevermind.`,
      );
  }

  function available() {
    return Boolean(backend && (backend !== 'x11' || environment.DISPLAY));
  }

  async function command(name: string, args: string[]) {
    await assertAvailable();
    try {
      return await run((await tool(name))!, args);
    } catch (error) {
      const code = (error as NodeJS.ErrnoException).code;
      if (code === 'EACCES')
        throw new DesktopTextAccessError(
          'denied',
          'Linux blocked desktop control. Check desktop session access and executable permissions.',
        );
      if (code === 'ENOENT')
        throw new DesktopTextAccessError(
          'unavailable',
          `The ${name} desktop tool is missing. Install it and restart Nevermind.`,
        );
      throw new DesktopTextAccessError(
        'unavailable',
        'Could not control the Linux desktop. Check that Nevermind and the source app use the same desktop session.',
      );
    }
  }

  async function target(): Promise<DesktopTextTarget | null> {
    await assertAvailable();
    if (backend === 'x11') {
      const windowId = (await command('xdotool', ['getactivewindow'])).trim();
      if (!/^\d+$/.test(windowId)) return null;
      const pid = Number(
        (await command('xdotool', ['getwindowpid', windowId])).trim(),
      );
      return targetForWindow(backend, windowId, pid);
    }
    if (backend === 'hyprland') {
      const active = JSON.parse(
        await command('hyprctl', ['-j', 'activewindow']),
      );
      return targetForWindow(
        backend,
        String(active.address ?? ''),
        Number(active.pid),
      );
    }
    const tree = JSON.parse(await command('swaymsg', ['-r', '-t', 'get_tree']));
    const active = swayWindows(tree).find((node) => node.focused && node.pid);
    return active
      ? targetForWindow('sway', String(active.id), Number(active.pid))
      : null;
  }

  async function restore(expected: DesktopTextTarget) {
    await assertAvailable();
    if (
      !expected.windowId ||
      !expected.bundleId.startsWith(`linux:${backend}:`)
    )
      return false;
    if (backend === 'x11') {
      const pid = Number(
        await command('xdotool', ['getwindowpid', expected.windowId]),
      );
      if (pid !== expected.pid) return false;
      await command('xdotool', ['windowactivate', '--sync', expected.windowId]);
    } else if (backend === 'hyprland') {
      const clients = JSON.parse(
        await command('hyprctl', ['-j', 'clients']),
      ) as { address: string; pid: number }[];
      if (
        !clients.some(
          (client) =>
            client.address === expected.windowId && client.pid === expected.pid,
        )
      )
        return false;
      await command('hyprctl', [
        'dispatch',
        'focuswindow',
        `address:${expected.windowId}`,
      ]);
    } else {
      const tree = JSON.parse(
        await command('swaymsg', ['-r', '-t', 'get_tree']),
      );
      if (
        !swayWindows(tree).some(
          (node) =>
            String(node.id) === expected.windowId && node.pid === expected.pid,
        )
      )
        return false;
      await command('swaymsg', [
        '-r',
        `[con_id=${expected.windowId}]`,
        'focus',
      ]);
    }
    return (await target())?.bundleId === expected.bundleId;
  }

  async function shortcut(expected: DesktopTextTarget, key: 'c' | 'v') {
    assertDesktopTextTarget(await target(), expected);
    if (backend === 'x11')
      await command('xdotool', ['key', '--clearmodifiers', `ctrl+${key}`]);
    else if (backend === 'hyprland')
      await command('hyprctl', [
        'dispatch',
        'sendshortcut',
        `CTRL,${key},address:${expected.windowId}`,
      ]);
    else await command('wtype', ['-M', 'ctrl', '-k', key, '-m', 'ctrl']);
  }

  async function read(expected: DesktopTextTarget) {
    assertDesktopTextTarget(await target(), expected);
    return null;
  }

  async function copy(expected: DesktopTextTarget) {
    await shortcut(expected, 'c');
    return true;
  }

  async function paste(expectedId?: string) {
    const current = await target();
    if (!current || (expectedId && current.bundleId !== expectedId))
      throw new DesktopTextAccessError(
        'unavailable',
        'The source window lost focus. Select the text and try again.',
      );
    await shortcut(current, 'v');
  }

  async function access() {
    try {
      await assertAvailable();
      if (backend === 'x11') await command('xdotool', ['getdisplaygeometry']);
      else if (backend === 'hyprland')
        await command('hyprctl', ['-j', 'version']);
      else await command('swaymsg', ['-r', '-t', 'get_version']);
      return {
        state: 'allowed' as const,
        message: 'Desktop control is available. Press Enter to check again.',
      };
    } catch (error) {
      return desktopTextFailure(error);
    }
  }

  return { available, access, target, restore, read, copy, paste };
}
