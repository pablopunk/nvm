# Command-K action menu parity: 1.x inventory

Source of truth for the ⌘K (Ctrl+K) action menu of every row kind in 2.0. Built from the 1.x source (`src/app`), not from
memory. Each row is the menu of one kind; the menu lists the row's primary action first, then the row's own actions, then
the generic options that 1.x adds to every row (`src/app/palette/App.tsx`, `getOptionActionRows`).

Status values: **done** (2.0 matches the 1.x title, order, shortcut and condition), **partial** (2.0 has the action with a
different title, shortcut or condition), **missing** (not in 2.0), **extra** (2.0 has it, 1.x does not: removed for parity),
**n/a** (no 1.x counterpart), **traced** (read from source, not exercised in a 1.x run).

## A. Generic options (every row with a menu)

Order is fixed by `getOptionActionRows` (App.tsx:4526). Rows follow the primary action and the row's own actions.

| # | 1.x file:line | Kind(s) | Title | Shortcut | Condition | 2.0 status |
|---|---|---|---|---|---|---|
| G1 | App.tsx:4531 | customizable rows | Set keyboard shortcut | none | `canCustomizeCommandAction`: `customizable` or kind in app, builtin, clipboard-history, extension-action (model.ts:26, 679) | done (`shortcut_row_actions.go`) |
| G2 | App.tsx:4539 | customizable rows with a binding | Remove keyboard shortcut | none | G1 and a shortcut is bound; subtitle is the shortcut | done |
| G3 | App.tsx:4549 | rows with preview content and no preview action | Preview | ⌘Y | `canPreviewAction` and not `hasPreview` (App.tsx:4168, root-action-preview.ts:22) | missing (no generic row Preview) |
| G4 | App.tsx:4558 | file rows without a preview or Quick Look action | Preview File | ⌘Y | `filePath` and not `hasQuickLook` and not `hasPreview` | missing |
| G5 | App.tsx:4567 | customizable rows | Set alias | none | G1 condition | missing (added in this change) |
| G6 | App.tsx:4575 | generated extension rows | Duplicate | none | kind extension-root-item or extension-action, and `removable` (generated) (App.tsx:4166) | missing |
| G7 | App.tsx:4583 | file-backed extension rows | Rename | none | `extensionFile` and kind extension-root-item or extension-action (App.tsx:4156) | missing |
| G8 | App.tsx:4591 | generated extension rows | Submit as PR | none | G6 condition and GitHub CLI installed; disabled with "Requires GitHub CLI (gh auth login)" until authed | missing |
| G9 | App.tsx:4602 | file-backed extension rows | Tweak with AI | none | G7 condition | missing |
| G10 | App.tsx:4610 | rows with a defaultActionId | Override with AI | none | `defaultActionId` (built-in with AI override) | missing (AI override not in 2.0) |
| G11 | App.tsx:4618 | overridden rows | Restore original | none | `isOverridden` | missing |
| G12 | App.tsx:4626 | AI chat rows and removable rows | Remove action | none | kind ai-chat, or `removable`; confirm: "Delete “<title>” from Nevermind" with Remove action / Cancel (App.tsx:3985-4000) | missing as "Remove action" (2.0 has Delete on extension rows and chats) |

Alias sub-flow (opened by G5, App.tsx:3955-3990 and 4215-4250): header "Alias for “<title>”" (App.tsx:2789). Rows: "Save alias “<query>”" when the query is new (subtitle "Make “<title>” appear for this phrase"); one row per saved alias ("Remove this alias", Enter removes it); "Done" (subtitle "<n> aliases saved" or "Close without changes"). Aliases are stored in config.json `aliases` keyed by action ID (portable-config-schema.ts:13, 73). Aliases match root search (action-ranker.ts:19, action-ranking.ts:43) and are not drawn on the row.

## B. Per row kind

### B1. Applications (`src/app/extensions/apps.ts:199`, customizable: true at :214)

| # | 1.x file:line | Title | Shortcut | Condition | 2.0 status |
|---|---|---|---|---|---|
| A1 | apps.ts:217 | Open <name> (primary) | Enter | always | done (`Open <name>`) |
| A2 | apps.ts:175 | Uninstall <name>… (destructive) | none | `app-uninstall` capability and path not under /System | done (`features/apps/view.go`) |
| A3 | (generic) | Set keyboard shortcut / Remove keyboard shortcut | | G1, G2 | done |
| A4 | (generic) | Set alias | | G5 | missing (added in this change) |
| — | apps.ts:225 | Force Quit <name> | | 1.x shows it only in the Force Quit Apps command, not on the row | extra (removed from the row) |
| — | (none) | Show in Folder, Copy Path, Quit <name> | | 1.x has no such row action | extra (removed from the row) |

Total for an app row in 1.x: Open, Uninstall (when supported), Set keyboard shortcut, Remove keyboard shortcut (when bound), Set alias.

### B2. Files (`src/app/extensions/files.ts:33`)

| # | 1.x file:line | Title | Shortcut | Condition | 2.0 status |
|---|---|---|---|---|---|
| F1 | files.ts:33 | Open <name> (primary) | Enter | always | done |
| F2 | files.ts:9 | Preview Contents | ⌘Y | non-media file (`previewFile`) | partial (2.0 "Preview", ⌃Y) |
| F2b | files.ts:9 | Preview | ⌘Y | image or video file | partial |
| F3 | files.ts:25 | Quick Look (macOS; "Preview File" elsewhere, only with the quick-look capability) | none | `quick-look` capability | missing |
| F4 | os.ts:410 | Show in Finder (macOS) / Show in File Explorer / Show in File Manager | none | always | partial (2.0 "Show in Folder", ⌃↵) |
| — | (none) | Copy Path, Copy File, Move to Trash | | 1.x has none on file rows | extra (removed) |
| — | G3, G4 | generic Preview and Preview File | | suppressed by F2/F3 | n/a |

File rows have no keyboard shortcut and no alias in 1.x (no `customizable`).

### B3. Clipboard entries (`src/app/electron/clipboard/clipboard-history.ts:372-470`)

| # | 1.x file:line | Title | Shortcut | Condition | 2.0 status |
|---|---|---|---|---|---|
| C1 | clipboard-history.ts:392 | Copy Text (Copy Image for images; Copy Video Path for video) (primary) | Enter | always | partial (2.0 uses the row's primary title) |
| C2 | clipboard-history.ts:372 | Preview | ⌘Y | always (preview intent) | partial |
| C3 | clipboard-history.ts:443 | Paste Text | none | text entries only (not image or video) | partial (2.0 "Paste", ⌃↵, also on images) |
| C4 | clipboard-removal-actions.ts:1-140, clipboard-history.ts:469 | Manage History section: Remove Item | none | entry present; destructive, confirm "Remove “<title>” from clipboard history?" | done |
| C5 | clipboard-removal-actions.ts:50 | Remove Last... (submenu): 5 Minutes, Hour, Day, Week | none | entry present | done |
| C6 | clipboard-removal-actions.ts:124-139 | (history level, Clipboard History view) Remove Entries from Last Hour, Remove Entries from Last Day, Remove All Entries | none | history not empty | done |
| — | none | Pin, Unpin, Ask AI, Clear Unpinned | | 1.x has none | extra (see report: PLAN J05 asks for pin) |

Clipboard entries have no keyboard shortcut or alias in 1.x.

### B4. Calculator results (`src/app/extensions/calculator.ts:86-136`)

| # | 1.x file:line | Title | Shortcut | Condition | 2.0 status |
|---|---|---|---|---|---|
| K1 | calculator.ts:91 | Copy Result (primary) | Enter | always | done |
| K2 | calculator.ts:97 | Continue Calculation | none | always (replaces the search with the result) | done |
| K3 | calculator.ts:106 | Swap Units | none | unit conversion with a swap query | done |
| K4 | calculator.ts:117 | Copy Unformatted Result | none | formatted result differs from raw | done |
| K5 | calculator.ts:123 | Paste Result | none | always | partial (2.0 binds ⌃↵) |

### B5. Commands and built-in extension rows (extension-action, `src/app/extensions/*.ts`)

Commands are durable actions (`commandFromItem`, extension-action-builders.ts:2) and are customizable unless the item sets `customizable: false` (view-normalizer.ts:100). Their menu is the generic set G1, G2, G5 after the command's own primary action (titled with the command title).

| Command | 1.x file:line | Kind | 1.x primary title | 2.0 status |
|---|---|---|---|---|
| Lock Screen | system.ts:10 | extension-action | Lock Screen | partial (2.0 keys the shortcut by title) |
| Sleep | system.ts:19 | extension-action | Sleep | partial |
| Restart Computer | system.ts:28 | extension-action | Restart Computer | partial |
| System Settings (Windows Settings on Windows) | system.ts:37, os.ts:395 | extension-action | System Settings | partial |
| Quit Nevermind | system.ts:48 | extension-action | Quit Nevermind | partial |
| Open Downloads | system.ts:103 (placesItems) | extension-action | Open Downloads | partial (keyed by title) |
| Open Documents | system.ts:103 | extension-action | Open Documents | partial |
| Open Desktop | system.ts:103 | extension-action | Open Desktop | partial |
| Force Quit Apps | apps.ts:255 | extension-action | Force Quit Apps | done |
| Clipboard History, AI Chats, Emoji & Symbols, Find Files by Meaning, Quick Notes, New Floating Note, Dictate, Extensions, Keyboard Shortcuts, Background Tasks, Settings, Check for Updates, Recent Errors, Report a Problem, OS Permissions | various | extension / extension-root | command title | done (ID-mapped in `portable_keys.go`) |
| Window layouts (Left Half, Maximize, …) | window-management-commands.ts | extension-action | layout title | partial (keys via layout prefix) |

### B6. Extension root items and extension actions (`extension-root-item`, `extension-action`)

Root items are customizable only when the item sets `customizable` or it has a persistent (registered) action (extension-root-action.ts:128). Their menu is the item's primary action, then its action panel, then the generic options G1-G12 that match their kind and flags.

| Case | 1.x file:line | Menu |
|---|---|---|
| Generated extension row | extension-root-action.ts:96 (`removable`) | primary, panel actions, G1, G2, G5, G6 Duplicate, G7 Rename, G8 Submit as PR, G9 Tweak with AI, G10-G11 when overridden, G12 Remove action |
| File-backed extension row | extension-root-action.ts:96 | primary, panel actions, G1, G2, G5, G7 Rename, G9 Tweak with AI |
| Command-backed row, not generated | extension-root-action.ts:128 | primary, panel actions, G1, G2, G5 |

Extension authoring actions (G6-G12) are missing in 2.0. See the report for the implementation status.

### B7. AI chats (`src/app/electron/ai/ai-chat-views.ts`, `ai-chat-store.ts:190-212`)

| # | 1.x file:line | Title | Condition | 2.0 status |
|---|---|---|---|---|
| H1 | ai-chat-views.ts:39 | Open (primary, the chat title) | always | done |
| H2 | ai-created-actions.ts:110 | Tweak <title> (builder chats) | extension chat | missing |
| H3 | (G12) | Remove action | kind ai-chat | partial (2.0 "Delete", ⌃⌫) |

H1-H3 are partly traced: the exact panel of an AI chat row was not read end to end.

### B8. Snippets, quicklinks, timers

No 1.x counterpart. Snippets, quicklinks and "Start timer" are 2.0 features (the 1.x API only lists them as a helper, `nevermind-extension-api.d.ts`). They are n/a for parity and keep their 2.0 menus.

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

| | 1.x | done | partial | missing | extra (removed) | n/a |
|---|---|---|---|---|---|---|
| Generic options G1-G12 | 12 | see report | | | | |
| Application rows | 3 + 1 | | | | | |
| File rows | 4 | | | | | |
| Clipboard entry rows | 6 | | | | | |
| Calculator rows | 5 | | | | | |
| Commands and built-ins | 1 (+ title) | | | | | |
| AI chat rows | 3 | | | | | |

The counts are filled in by the parity test (`cmdk_parity_test.go`) and the report.
