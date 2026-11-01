// biome-ignore-all lint: Fake Electron tray objects are intentionally loose.
import assert from 'node:assert/strict';
import test from 'node:test';
import {
  createMenuBarStatusItems,
  menuRebuildCooldownMs,
  type StatusItemMenuEntry,
  type StatusItemSpec,
  statusItemIconFile,
  statusAreaIsDark,
  statusItemMenuTemplate,
  statusItemTooltip,
} from './menubar-status-items';

const menu: StatusItemMenuEntry[] = [
  { type: 'header', label: 'Focus' },
  {
    type: 'item',
    label: 'Start',
    sublabel: '25 minutes',
    accelerator: 'Command+Shift+P',
    icon: 'play',
  },
  { type: 'separator' },
  { type: 'item', label: 'Sound', checked: true },
];

function fakeImage() {
  return {
    template: false,
    setTemplateImage(value) {
      this.template = value;
    },
  };
}

function createFakeElectron() {
  const trays: any[] = [];
  const menus: any[] = [];
  class Tray {
    image: any;
    title = '';
    tooltip = '';
    contextMenu: any;
    destroyed = false;
    popUps = 0;
    listeners = new Map<string, () => void>();
    constructor(image) {
      this.image = image;
      trays.push(this);
    }
    setImage(image) {
      this.image = image;
    }
    setTitle(title) {
      this.title = title;
    }
    setToolTip(tooltip) {
      this.tooltip = tooltip;
    }
    setContextMenu(contextMenu) {
      this.contextMenu = contextMenu;
    }
    popUpContextMenu() {
      this.popUps += 1;
    }
    on(event, listener) {
      this.listeners.set(event, listener);
    }
    destroy() {
      this.destroyed = true;
    }
  }
  const Menu = {
    buildFromTemplate(template) {
      const built = {
        template,
        listeners: new Map(),
        on(event, listener) {
          this.listeners.set(event, listener);
        },
      };
      menus.push(built);
      return built;
    },
  };
  const loaded: string[] = [];
  const nativeImage = {
    createFromPath(file: string) {
      loaded.push(file);
      return fakeImage();
    },
  };
  const nativeTheme = { shouldUseDarkColors: true, on() {} };
  return {
    trays,
    menus,
    loaded,
    electron: { Tray, Menu, nativeImage, nativeTheme } as any,
  };
}

const spec: StatusItemSpec = {
  icon: 'timer',
  title: '24:59',
  tooltip: 'Pomodoro',
  menu,
};

test('macOS keeps sublabels, shortcuts and row icons', () => {
  const template = statusItemMenuTemplate(menu, 'darwin', () => ({}) as any);
  assert.deepEqual(template[0], { label: 'Focus', enabled: false });
  assert.equal(template[1].label, 'Start');
  assert.equal(template[1].sublabel, '25 minutes');
  assert.equal(template[1].accelerator, 'Command+Shift+P');
  assert.equal(template[1].registerAccelerator, false);
  assert.ok(template[1].icon);
  assert.equal(template[3].type, 'checkbox');
});

test('Linux folds subtitles into labels and drops shortcuts GTK rejects', () => {
  const template = statusItemMenuTemplate(menu, 'linux', () => ({}) as any);
  assert.equal(template[1].label, 'Start — 25 minutes');
  assert.equal(template[1].sublabel, undefined);
  assert.equal(template[1].accelerator, undefined);
  assert.equal(template[1].icon, undefined);
});

test('Windows shows shortcuts but folds subtitles', () => {
  const template = statusItemMenuTemplate(menu, 'win32', () => ({}) as any);
  assert.equal(template[1].label, 'Start — 25 minutes');
  assert.equal(template[1].accelerator, 'Command+Shift+P');
});

test('the status title moves into the tooltip where trays have no title', () => {
  assert.equal(statusItemTooltip(spec, 'darwin'), 'Pomodoro');
  assert.equal(statusItemTooltip(spec, 'win32'), 'Pomodoro — 24:59');
});

test('icons use template images on macOS and theme variants elsewhere', () => {
  assert.equal(statusItemIconFile('timer', 'darwin', true), 'timer.png');
  assert.equal(statusItemIconFile('timer', 'linux', true), 'timer-white.png');
  assert.equal(statusItemIconFile('timer', 'win32', false), 'timer.png');
});

test('macOS status items get a template image and monospaced title', () => {
  const fake = createFakeElectron();
  const items = createMenuBarStatusItems({
    platform: 'darwin',
    iconsDir: '/icons',
    electron: fake.electron,
  });

  assert.equal(items.upsert('pomodoro:main', spec), true);
  assert.equal(fake.trays.length, 1);
  assert.equal(fake.trays[0].image.template, true);
  assert.equal(fake.trays[0].title, '24:59');
  assert.equal(fake.loaded[0], '/icons/timer.png');
});

test('Windows opens the menu on a primary click and reports the open', () => {
  const fake = createFakeElectron();
  const opened: string[] = [];
  const items = createMenuBarStatusItems({
    platform: 'win32',
    iconsDir: '/icons',
    electron: fake.electron,
    onMenuOpened: (id) => opened.push(id),
  });
  items.upsert('pomodoro:main', spec);

  fake.trays[0].listeners.get('click')();

  assert.equal(fake.trays[0].popUps, 1);
  assert.deepEqual(opened, ['pomodoro:main']);
});

test('menu rows wait while the menu is open; the title keeps updating', async () => {
  const fake = createFakeElectron();
  const items = createMenuBarStatusItems({
    platform: 'darwin',
    iconsDir: '/icons',
    electron: fake.electron,
  });
  items.upsert('pomodoro:main', spec);
  const openMenu = fake.trays[0].contextMenu;
  openMenu.listeners.get('menu-will-show')();

  items.upsert('pomodoro:main', {
    ...spec,
    title: '24:58',
    menu: [{ type: 'item', label: 'Stop' }],
  });
  assert.equal(fake.trays[0].title, '24:58');
  assert.equal(fake.trays[0].contextMenu, openMenu);

  openMenu.listeners.get('menu-will-close')();
  await new Promise((resolve) => setTimeout(resolve, 0));
  assert.notEqual(fake.trays[0].contextMenu, openMenu);
  assert.equal(fake.trays[0].contextMenu.template[0].label, 'Stop');
});

test('a ticking title never rebuilds the native menu', () => {
  const fake = createFakeElectron();
  const items = createMenuBarStatusItems({
    platform: 'linux',
    iconsDir: '/icons',
    electron: fake.electron,
  });
  items.upsert('pomodoro:main', spec);
  const menusBuilt = fake.menus.length;

  items.upsert('pomodoro:main', { ...spec, title: '24:58' });
  items.upsert('pomodoro:main', { ...spec, title: '24:57' });

  assert.equal(fake.menus.length, menusBuilt);
  assert.equal(fake.trays[0].tooltip, 'Pomodoro — 24:57');
});

test('retainOnly destroys status items that are no longer contributed', () => {
  const fake = createFakeElectron();
  const items = createMenuBarStatusItems({
    platform: 'linux',
    iconsDir: '/icons',
    electron: fake.electron,
  });
  items.upsert('a:main', spec);
  items.upsert('b:main', spec);

  items.retainOnly(['b:main']);

  assert.equal(fake.trays[0].destroyed, true);
  assert.equal(fake.trays[1].destroyed, false);
  assert.deepEqual(items.ids(), ['b:main']);
});

test('unsupported platforms create nothing', () => {
  const fake = createFakeElectron();
  const items = createMenuBarStatusItems({
    platform: 'aix',
    iconsDir: '/icons',
    electron: fake.electron,
  });
  assert.equal(items.supported, false);
  assert.equal(items.upsert('a:main', spec), false);
  assert.equal(fake.trays.length, 0);
});

test('the Windows taskbar theme wins over the app theme', () => {
  assert.equal(
    statusAreaIsDark({
      platform: 'win32',
      shouldUseDarkColors: false,
      shouldUseDarkColorsForSystemIntegratedUI: true,
    }),
    true,
  );
});

test('GNOME-family panels are dark even with a light app theme', () => {
  assert.equal(
    statusAreaIsDark({
      platform: 'linux',
      desktopEnvironment: 'ubuntu:GNOME',
      shouldUseDarkColors: false,
    }),
    true,
  );
  assert.equal(
    statusAreaIsDark({
      platform: 'linux',
      desktopEnvironment: 'KDE',
      shouldUseDarkColors: false,
    }),
    false,
  );
});

test('unchanged specs keep the menu and clicks reach the newest handler', () => {
  const fake = createFakeElectron();
  const items = createMenuBarStatusItems({
    platform: 'linux',
    iconsDir: '/icons',
    electron: fake.electron,
  });
  const clicks: string[] = [];
  const specWithClick = (label: string): StatusItemSpec => ({
    ...spec,
    menu: [
      {
        type: 'item',
        label: 'Run',
        submenu: [
          { type: 'item', label: 'Nested', onClick: () => clicks.push(label) },
        ],
      },
    ],
  });
  items.upsert('a:main', specWithClick('first'));
  const builtMenus = fake.menus.length;

  items.upsert('a:main', specWithClick('second'));
  fake.menus.at(-1).template[0].submenu[0].click();

  assert.equal(fake.menus.length, builtMenus);
  assert.deepEqual(clicks, ['second']);
});

test('Linux coalesces menu rebuilds so an open menu survives frequent changes', () => {
  assert.equal(menuRebuildCooldownMs('linux'), 5000);
  assert.equal(menuRebuildCooldownMs('darwin'), 0);
  const fake = createFakeElectron();
  const items = createMenuBarStatusItems({
    platform: 'linux',
    iconsDir: '/icons',
    electron: fake.electron,
  });
  items.upsert('pomodoro:main', spec);
  const menusBuilt = fake.menus.length;

  items.upsert('pomodoro:main', {
    ...spec,
    menu: [{ type: 'item', label: 'A' }],
  });
  items.upsert('pomodoro:main', {
    ...spec,
    menu: [{ type: 'item', label: 'B' }],
  });

  assert.equal(fake.menus.length, menusBuilt);
  items.retainOnly([]);
});
