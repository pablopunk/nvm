import assert from 'node:assert/strict'
import test from 'node:test'
import type {
  ActionPanelVisibility,
  ExtensionAccessoryTone,
  ExtensionEditorFormat,
  ExtensionFormFieldType,
  ExtensionFormValue,
  ExtensionImage,
  ExtensionPermission as PublicExtensionPermission,
  ForegroundColor,
  PatchMode,
  ViewPresentation,
  ViewSize,
} from './resources/nevermind-extension-api'
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
} from './model'

type Equal<A, B> = (<T>() => T extends A ? 1 : 2) extends (<T>() => T extends B ? 1 : 2)
  ? (<T>() => T extends B ? 1 : 2) extends (<T>() => T extends A ? 1 : 2)
    ? true
    : false
  : false

type AssertEqual<A, B> = Equal<A, B> extends true ? true : never

const permissionContract: AssertEqual<HostExtensionPermission, PublicExtensionPermission> = true
const patchModeContract: AssertEqual<NonNullable<CommandViewPatch['mode']>, PatchMode> = true
const foregroundContract: AssertEqual<CommandItemForeground, ForegroundColor> = true
const accessoryToneContract: AssertEqual<CommandAccessoryTone, ExtensionAccessoryTone> = true
const formValueContract: AssertEqual<CommandFormValue, ExtensionFormValue> = true
const formFieldTypeContract: AssertEqual<CommandFormFieldType, ExtensionFormFieldType> = true
const imageContract: AssertEqual<CommandImage, ExtensionImage> = true
const actionPanelVisibilityContract: AssertEqual<NonNullable<CommandItem['actionPanelVisibility']>, ActionPanelVisibility> = true
const viewTypeContract: AssertEqual<NonNullable<CommandView['type']>, NonNullable<import('./resources/nevermind-extension-api').ExtensionView['type']>> = true
const viewSizeContract: AssertEqual<NonNullable<CommandView['size']>, ViewSize> = true
const viewPresentationContract: AssertEqual<NonNullable<CommandView['presentation']>, ViewPresentation> = true
const editorFormatContract: AssertEqual<NonNullable<CommandView['format']>, ExtensionEditorFormat> = true

export const modelContractAssertions = {
  permissionContract,
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
}

test('host model shares canonical public extension API literal contracts', () => {
  for (const value of Object.values(modelContractAssertions)) assert.equal(value, true)
})
