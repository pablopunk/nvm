// biome-ignore-all lint/style/noMagicNumbers: Type equality requires distinct literal discriminator branches.
import assert from 'node:assert/strict';
import test from 'node:test';
import {
  EXTENSION_WINDOW_CAPABILITIES,
  type ExtensionWindowCapability as HostExtensionWindowCapability,
} from './electron/extension-window-capabilities';
import type {
  CommandAccessoryTone,
  CommandFormFieldType,
  CommandFormValue,
  CommandImage,
  CommandItem,
  CommandItemForeground,
  CommandView,
  CommandViewPatch,
  ExtensionPermission as HostExtensionPermission,
} from './model';
import type {
  ActionPanelVisibility,
  ExtensionAccessoryTone,
  ExtensionEditorFormat,
  ExtensionFormFieldType,
  ExtensionFormValue,
  ExtensionImage,
  ExtensionView,
  ExtensionWebviewPermission,
  ForegroundColor,
  PatchMode,
  ExtensionPermission as PublicExtensionPermission,
  ExtensionWindowCapability as PublicExtensionWindowCapability,
  ViewPresentation,
  ViewSize,
} from './resources/nevermind-extension-api';

type Equal<A, B> =
  (<T>() => T extends A ? 1 : 2) extends <T>() => T extends B ? 1 : 2
    ? (<T>() => T extends B ? 1 : 2) extends <T>() => T extends A ? 1 : 2
      ? true
      : false
    : false;

type AssertEqual<A, B> = Equal<A, B> extends true ? true : never;

const permissionContract: AssertEqual<
  HostExtensionPermission,
  PublicExtensionPermission
> = true;
const extensionWindowCapabilityContract: AssertEqual<
  HostExtensionWindowCapability,
  PublicExtensionWindowCapability
> = true;
const patchModeContract: AssertEqual<
  NonNullable<CommandViewPatch['mode']>,
  PatchMode
> = true;
const foregroundContract: AssertEqual<CommandItemForeground, ForegroundColor> =
  true;
const accessoryToneContract: AssertEqual<
  CommandAccessoryTone,
  ExtensionAccessoryTone
> = true;
const formValueContract: AssertEqual<CommandFormValue, ExtensionFormValue> =
  true;
const formFieldTypeContract: AssertEqual<
  CommandFormFieldType,
  ExtensionFormFieldType
> = true;
const imageContract: AssertEqual<CommandImage, ExtensionImage> = true;
const actionPanelVisibilityContract: AssertEqual<
  NonNullable<CommandItem['actionPanelVisibility']>,
  ActionPanelVisibility
> = true;
const viewTypeContract: AssertEqual<
  NonNullable<CommandView['type']>,
  NonNullable<
    import('./resources/nevermind-extension-api').ExtensionView['type']
  >
> = true;
const viewSizeContract: AssertEqual<
  NonNullable<CommandView['size']>,
  ViewSize
> = true;
const viewPresentationContract: AssertEqual<
  NonNullable<CommandView['presentation']>,
  ViewPresentation
> = true;
const editorFormatContract: AssertEqual<
  NonNullable<CommandView['format']>,
  ExtensionEditorFormat
> = true;
const webviewPermissionContract: AssertEqual<
  NonNullable<CommandView['webviewPermissions']>[number],
  ExtensionWebviewPermission
> = true;
const viewLayoutContract: AssertEqual<
  NonNullable<CommandView['layout']>,
  NonNullable<ExtensionView['layout']>
> = true;
const viewAspectRatioContract: AssertEqual<
  NonNullable<CommandView['aspectRatio']>,
  NonNullable<ExtensionView['aspectRatio']>
> = true;
const viewColumnsContract: AssertEqual<
  NonNullable<CommandView['columns']>,
  NonNullable<ExtensionView['columns']>
> = true;

const modelContractAssertions = {
  permissionContract,
  extensionWindowCapabilityContract,
  patchModeContract,
  foregroundContract,
  accessoryToneContract,
  formValueContract,
  formFieldTypeContract,
  imageContract,
  actionPanelVisibilityContract,
  viewTypeContract,
  viewSizeContract,
  viewPresentationContract,
  editorFormatContract,
  webviewPermissionContract,
  viewLayoutContract,
  viewAspectRatioContract,
  viewColumnsContract,
};

test('host model shares canonical public extension API literal contracts', () => {
  for (const value of Object.values(modelContractAssertions)) {
    assert.equal(value, true);
  }
  assert.deepEqual(EXTENSION_WINDOW_CAPABILITIES, [
    'windows.always-on-top',
    'windows.all-spaces',
    'windows.frame-restore',
    'windows.display-recovery',
  ]);
});
