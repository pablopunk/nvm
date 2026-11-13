# Command-K action menu parity: 1.x inventory

Source of truth for the ⌘K (Ctrl+K) action menu of every row kind in 2.0. Built from the 1.x source (the 1.x app), not from
memory. Each row is the menu of one kind; the menu lists the row's primary action first, then the row's own actions, then
the generic options that 1.x adds to every row (`palette/App.tsx`, `getOptionActionRows`).

Status values: **done** (2.0 matches the 1.x title, order, shortcut and condition), **partial** (2.0 has the action with a
different title, shortcut or condition), **missing** (not in 2.0), **extra** (2.0 has it, 1.x does not: removed for parity),
**n/a** (no 1.x counterpart), **traced** (read from source, not exercised in a 1.x run).

## A. Generic options (every row with a menu)

Order is fixed by `getOptionActionRows` (App.tsx:4526). Rows follow the primary action and the row's own actions.

| # | 1.x file:line | Kind(s) | Title | Shortcut | Condition | 2.0 status |
|---|---|---|---|---|---|---|
| G1 | App.tsx:4531 | customizable rows | Set keyboard shortcut | none | `canCustomizeCommandAction`: `customizable` or kind in app, builtin, clipboard-history, extension-action (model.ts:26, 679) | done (`shortcut_row_actions.go`) |
| G2 | App.tsx:4539 | customizable rows with a binding | Remove keyboard shortcut | none | G1 and a shortcut is bound; subtitle is the shortcut | done |
| G3 | App.tsx:4549 | rows with preview content and no preview action | Preview | ⌘Y | `canPreviewAction` and not `hasPreview` (App.tsx:4168, root-action-preview.ts:22) | done: `api.WithPreviewActions` (ext/api/item_preview.go) adds it to every row with text, a picture, a video or a file and no preview of its own |
| G4 | App.tsx:4558 | file rows without a preview or Quick Look action | Preview File | ⌘Y | `filePath` and not `hasQuickLook` and not `hasPreview` | done: added with G3 for a file row that has no preview of its own (file rows keep Preview Contents, so their menu is unchanged) |
| G5 | App.tsx:4567 | customizable rows | Set alias | none | G1 condition | done (`shortcut_row_actions.go`, `palette/aliasview`) |
| G6 | App.tsx:4575 | generated extension rows | Duplicate | none | kind extension-root-item or extension-action, and `removable` (generated) (App.tsx:4166) | done: copies the folder under a new id, enabled, then opens the builder chat (internal/ext/authoring) |
| G7 | App.tsx:4583 | file-backed extension rows | Rename | none | `extensionFile` and kind extension-root-item or extension-action (App.tsx:4156) | done: the form writes the title into extension.json (internal/ext/authoring) |
| G8 | App.tsx:4591 | generated extension rows | Submit as PR | none | G6 condition and GitHub CLI installed; disabled with "Requires GitHub CLI (gh auth login)" until authed | done: hidden without gh, disabled with "Requires GitHub CLI (gh auth login)" when signed out, else opens the PR (internal/ext/submit) |
| G9 | App.tsx:4602 | file-backed extension rows | Tweak with AI | none | G7 condition | done: opens the builder chat bound to the extension (builderEditView) |
| G10 | App.tsx:4610 | rows with a defaultActionId | Override with AI | none | `defaultActionId` (built-in with AI override) | not shown: no source sets defaultActionId, so 1.x never shows it; n/a |
| G11 | App.tsx:4618 | overridden rows | Restore original | none | `isOverridden` | not shown: overrides cannot be created in 1.x, so the condition is unreachable; n/a |
| G12 | App.tsx:4626 | AI chat rows and removable rows | Remove action (extension rows); Remove Chat (AI chat rows) | none | extension rows: `removable`; confirm "Delete “<title>” from Nevermind" with Remove action / Cancel (App.tsx:3985-4000). AI chat rows: Remove Chat (ai-chat-lifecycle.ts:82-92), confirm `Remove "<title>" and its history?` with Remove Chat / Cancel | done: builder-made extension rows confirm, then the extension is uninstalled; AI chat rows confirm, then the chat is removed |

Alias sub-flow (opened by G5, App.tsx:3955-3990 and 4215-4250): header "Alias for “<title>”" (App.tsx:2789). Rows: "Save alias “<query>”" when the query is new (subtitle "Make “<title>” appear for this phrase"); one row per saved alias ("Remove this alias", Enter removes it); "Done" (subtitle "<n> aliases saved" or "Close without changes"). Aliases are stored in config.json `aliases` keyed by action ID (portable-config-schema.ts:13, 73). Aliases match root search (action-ranker.ts:19, action-ranking.ts:43) and are not drawn on the row.

## B. Per row kind

### B1. Applications (`1.x/extensions/apps.ts:199`, customizable: true at :214)

| # | 1.x file:line | Title | Shortcut | Condition | 2.0 status |
|---|---|---|---|---|---|
| A1 | apps.ts:217 | Open <name> (primary) | Enter | always | done (`Open <name>`) |
| A2 | apps.ts:175 | Uninstall <name>… (destructive) | none | `app-uninstall` capability and path not under /System | done (`features/apps/view.go`) |
| A3 | (generic) | Set keyboard shortcut / Remove keyboard shortcut | | G1, G2 | done |
| A4 | (generic) | Set alias | | G5 | done |
| — | apps.ts:225 | Force Quit <name> | | 1.x shows it only in the Force Quit Apps command, not on the row | extra (removed from the row) |
| A5 | (new) | Show in Finder / Show in File Manager | ⌘↵ | every result with a file, applications included | done (a standard of palette results, `extview.WithRevealShortcut`) |
| — | (none) | Copy Path, Quit <name> | | 1.x has no such row action | extra (removed from the row) |

Total for an app row: Open, Uninstall (when supported), Show in Finder (⌘↵, new), Set keyboard shortcut, Remove keyboard shortcut (when bound), Set alias.

### B2. Files (`1.x/extensions/files.ts:33`)

| # | 1.x file:line | Title | Shortcut | Condition | 2.0 status |
|---|---|---|---|---|---|
| F1 | files.ts:33 | Open <name> (primary) | Enter | always | done |
| F2 | files.ts:9 | Preview Contents | ⌘Y | non-media file (`previewFile`) | done (2.0 label ⌃Y, the platform's modifier label convention) |
| F2b | files.ts:9 | Preview | ⌘Y | image or video file | done |
| F3 | files.ts:25 | Quick Look (macOS; "Preview File" elsewhere, only with the quick-look capability) | none | `quick-look` capability | done on macOS (shell.quickLook, from views-parity d2b7dec3) |
| F4 | os.ts:410 | Show in Finder (macOS) / Show in File Explorer / Show in File Manager | none | always | done (`revealTitle`) |
| — | (none) | Copy Path, Copy File, Move to Trash | | 1.x has none on file rows | extra: removed from the menu |
| — | G3, G4 | generic Preview and Preview File | | a file row keeps its own Preview Contents and Quick Look, and gets neither | done (generic, on the API) |

File rows have no keyboard shortcut and no alias in 1.x (no `customizable`).

### B3. Clipboard entries (`1.x/electron/clipboard/clipboard-history.ts:372-470`)

| # | 1.x file:line | Title | Shortcut | Condition | 2.0 status |
|---|---|---|---|---|---|
| C1 | clipboard-history.ts:392 | Copy Text (Copy Image for images; Copy Video Path for video) (primary) | Enter | always | traced: 2.0 primary title not compared in a 1.x run |
| C2 | clipboard-history.ts:372 | Preview | ⌘Y | always (preview intent) | done (2.0 label ⌃Y) |
| C3 | clipboard-history.ts:443 | Paste Text | none | text entries only (not image or video) | done (2.0 label ⌃↵) |
| C4 | clipboard-removal-actions.ts:1-140, clipboard-history.ts:469 | Manage History section: Remove Item | none | entry present; destructive, confirm "Remove “<title>” from clipboard history?" | done |
| C5 | clipboard-removal-actions.ts:50 | Remove Last... (submenu): 5 Minutes, Hour, Day, Week | none | entry present | done |
| C6 | clipboard-removal-actions.ts:124-139, clipboard-history.ts:508-512 | (history level, the view's `actionPanel` "Manage History", `actionPanelVisibility: 'menu'`) Remove Entries from Last Hour, Remove Entries from Last Day, Remove All Entries | history not empty | the view's actions join each entry's menu after the entry's own, as 1.x's `viewActionsForItem` does (F1, see the visibility table in B6); no "View" header is drawn yet |
| — | none | Pin, Unpin | | 1.x has none | removed; pinned state and sorting are gone from the store too |
| — | none | Ask AI, Clear All Unpinned | | 1.x has none | removed from the entry menu |

Clipboard entries have no keyboard shortcut or alias in 1.x.

### B4. Calculator results (`1.x/extensions/calculator.ts:86-136`)

| # | 1.x file:line | Title | Shortcut | Condition | 2.0 status |
|---|---|---|---|---|---|
| K1 | calculator.ts:91 | Copy Result (primary) | Enter | always | done |
| K2 | calculator.ts:97 | Continue Calculation | none | always (replaces the search with the result) | done |
| K3 | calculator.ts:106 | Swap Units | none | unit conversion with a swap query | done |
| K4 | calculator.ts:117 | Copy Unformatted Result | none | formatted result differs from raw | done |
| K5 | calculator.ts:123 | Paste Result | none | always | done (2.0 label ⌃↵) |

### B5. Commands and built-in extension rows (extension-action, `1.x/extensions/*.ts`)

Commands are durable actions (`commandFromItem`, extension-action-builders.ts:2) and are customizable unless the item sets `customizable: false` (view-normalizer.ts:100). Their menu is the generic set G1, G2, G5 after the command's own primary action (titled with the command title).

| Command | 1.x file:line | Kind | 1.x primary title | 2.0 status |
|---|---|---|---|---|
| Lock Screen | system.ts:10 | extension-action | Lock Screen | done (stable ID `command:lock-screen`; alias matches) |
| Sleep | system.ts:19 | extension-action | Sleep | done (stable ID) |
| Restart Computer | system.ts:28 | extension-action | Restart Computer | done (row and primary titled Restart Computer) |
| System Settings (Windows Settings on Windows) | system.ts:37, os.ts:395 | extension-action | System Settings | done (stable ID `command:system-settings`) |
| Quit Nevermind | system.ts:48 | extension-action | Quit Nevermind | done (stable ID `command:quit-nevermind`) |
| Open Downloads | system.ts:103 (placesItems) | extension-action | Open Downloads | done (stable ID `command:place-downloads`; row title is the folder name) |
| Open Documents | system.ts:103 | extension-action | Open Documents | done (stable ID) |
| Open Desktop | system.ts:103 | extension-action | Open Desktop | done (stable ID) |
| Force Quit Apps | apps.ts:255 | extension-action | Force Quit Apps | done |
| Dictate | dictation extension `dictateCommand` | extension-action | Dictate | done: panel action Settings (`command.ActionPanel`), built-in and customizable |
| Triage Selected Text with AI | ai-commands.ts:224 (`triage-selected-text-with-ai`) | extension-action | Triage Selected Text with AI | done: built-in `aicommands` (`triageCommand`), listed with the selection capability, customizable; its shortcut key is the 1.x key |
| Clipboard History, AI Chats, Emoji & Symbols, Find Files by Meaning, Quick Notes, New Floating Note, Dictate, Extensions, Keyboard Shortcuts, Background Tasks, Settings, Check for Updates, Recent Errors, Report a Problem, OS Permissions | various | extension / extension-root | command title | done (ID-mapped in `portable_keys.go`) |
| Window layouts (Left Half, Maximize, …) | window-management-commands.ts | extension-action | layout title | not re-traced in this pass; shortcuts keyed by the layout prefix |

### B6. Extension root items and extension actions (`extension-root-item`, `extension-action`)

Root items are customizable only when the item sets `customizable` or it has a persistent (registered) action (extension-root-action.ts:128). Their menu is the item's primary action, then its action panel, then the generic options G1-G12 that match their kind and flags.

| Case | 1.x file:line | Menu |
|---|---|---|
| Generated extension row | extension-root-action.ts:96 (`removable`) | primary, panel actions, G1, G2, G5, G6 Duplicate, G7 Rename, G8 Submit as PR, G9 Tweak with AI, G10-G11 when overridden, G12 Remove action |
| File-backed extension row | extension-root-action.ts:96 | primary, panel actions, G1, G2, G5, G7 Rename, G9 Tweak with AI |
| Command-backed row, not generated | extension-root-action.ts:128 | primary, panel actions, G1, G2, G5 |
| Built-in command with a panel (Dictate) | dictation extension `dictateCommand`, `actionPanel` Settings | primary, panel actions (Settings), G1, G2, G5 (done: `commandRow` maps the panel, `MenuWireActions`) |

`actionPanelVisibility` on an item or a view (follow-up F1, done in 2.0; see `internal/palette/extview/menu.go`):

| Setting | 1.x | 2.0 |
|---|---|---|
| item `visible` or `menu` | ⌘K lists the primary, the panel and the view's actions | same; `menu` and `visible` are equal because 2.0 draws no inline panel |
| item `hidden` | ⌘K does not open for the row; no row shortcut label; local shortcuts of its actions still run (`localActionCandidates`) | all its actions kept on the row for Enter and their shortcuts, marked `MenuHidden`, so the menu does not open |
| view `visible` or `menu` | view actions join each row's menu under View, and an empty list's ⌘K lists them | done: view actions join every row's menu under a View header, with or without the row's own actions; they are menu-only, so Enter runs the row's first own action (or nothing when it has none). Panel section titles are drawn as headers too (`palette/action_menu_rows.go`) |
| view `hidden` | view actions leave every ⌘K menu; their shortcuts still run | view actions kept as `MenuHidden` shortcut-only actions |

Extension authoring actions (G6-G12) are missing in 2.0. See the report for the implementation status.

### B7. AI chats (`1.x/electron/ai/ai-chat-views.ts`, `ai-chat-store.ts:190-212`)

| # | 1.x file:line | Title | Condition | 2.0 status |
|---|---|---|---|---|
| H1 | ai-chat-views.ts:39 | Open (primary, the chat title) | always | done |
| H2 | ai-created-actions.ts:110 | Tweak <title> (builder chats) | extension chat | missing |
| H3 | ai-chat-lifecycle.ts:82-92 | Remove Chat | kind ai-chat | done: confirmation `Remove "<title>" and its history?` (Generated extension files stay. for builder chats), confirm with Remove Chat / Cancel (ConversationRow, removeConfirm) |

H1-H3 are partly traced: the exact panel of an AI chat row was not read end to end.

### B8. Snippets, quicklinks, timers

No 1.x counterpart. Snippets carry their text as a detail, so they also get the generic Preview (G3). Snippets, quicklinks and "Start timer" are 2.0 features (the 1.x API only lists them as a helper, `nevermind-extension-api.d.ts`). They are n/a for parity and keep their 2.0 menus.

### B9. System and places built-ins (2.0 IDs)

2.0 IDs are stable and map to 1.x keys in `src/native/internal/shortcuts/portable_keys.go`:

| 2.0 ID | 1.x key | Row |
|---|---|---|
| command:lock-screen | extension:nevermind.system:builtin:lock-screen | Lock Screen |
| command:sleep | extension:nevermind.system:builtin:sleep | Sleep |
| command:restart | extension:nevermind.system:builtin:restart | Restart Computer |
| command:system-settings | extension:nevermind.system:builtin:settings | System Settings |
| command:quit | extension:nevermind.system:builtin:quit | Quit Nevermind |
| command:places-downloads | extension:nevermind.places:places:downloads | Open Downloads |
| command:places-documents | extension:nevermind.places:places:documents | Open Documents |
| command:places-desktop | extension:nevermind.places:places:desktop | Open Desktop |

## C. Counts

Counted over the rows above, after this change.

| Group | 1.x actions | done | disabled with reason | n/a (not shown in 1.x) | removed from 2.0 |
|---|---|---|---|---|---|
| Generic G1-G12 | 12 | 5 (G1, G2, G5, G9, G12 for builder rows) | 3 (G6, G7, G8) | 4 (G3, G4 covered by own Preview; G10, G11 unreachable) | 0 |
| Applications B1 | 2 + alias | 3 (Open, Uninstall, shortcut and alias) | 0 | 0 | 4 (Show in Folder, Copy Path, Quit, Force Quit) |
| Files B2 | 4 | 4 on macOS, 3 elsewhere (Quick Look is macOS only) | 0 | 0 | 3 (Copy Path, Copy File, Move to Trash) |
| Clipboard entries B3 | 5 | 5 | 0 | 0 | 4 (Pin, Unpin, Ask AI, Clear All Unpinned) |
| Calculator B4 | 5 | 5 | 0 | 0 | 0 |
| Commands and built-ins B5 | per command | done with stable IDs and alias matching | 0 | 0 | 0 |
| Extension rows B6 | 3 cases | authoring group | 3 | 0 | 0 |
| AI chats B7 | 3 | 1 | 0 | 0 | 0 |
| Snippets, quicklinks, timers B8 | 0 | 0 | 0 | 0 | n/a, 2.0 only |

Not covered by the menu test yet: extension root items (rows of an extension, not its commands). The AI override group is
not shown because 1.x never shows it.
