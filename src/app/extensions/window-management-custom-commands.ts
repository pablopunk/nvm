import type { ExtensionContext } from '../resources/nevermind-extension-api';
import { WINDOW_MANAGEMENT_SUBTITLE } from './window-management-commands';

const CUSTOM_COMMANDS_TITLE = 'Custom Window Commands';

const CUSTOM_COMMANDS_NOTICE =
  "Coming from Raycast, I presume? You don't need custom window commands here — just ask AI to build one. Describe what you want right in the command palette, for example “open Safari on the left at 60% and Terminal on the right at 40%”, and it will create an extension for you.";

function customWindowCommandsNoticeView(ctx: ExtensionContext) {
  return ctx.ui.previewView({
    title: CUSTOM_COMMANDS_TITLE,
    subtitle: WINDOW_MANAGEMENT_SUBTITLE,
    content: CUSTOM_COMMANDS_NOTICE,
  });
}

function customWindowCommandsCommand() {
  return {
    id: 'custom-commands',
    title: CUSTOM_COMMANDS_TITLE,
    subtitle: WINDOW_MANAGEMENT_SUBTITLE,
    icon: 'layout-list',
    run: (ctx: ExtensionContext) => customWindowCommandsNoticeView(ctx),
  };
}

export {
  CUSTOM_COMMANDS_NOTICE,
  CUSTOM_COMMANDS_TITLE,
  customWindowCommandsCommand,
};
