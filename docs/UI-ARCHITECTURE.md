# UI architecture and maintenance

KRIEMHILD shares a visual language, not a universal page template.

## Entry points and styling boundaries

`src/main.jsx` mounts `UIProvider` around `App` and imports `worldbuilding/ui.css`
after the legacy terrain stylesheet. `App` owns World routes, Age context and
navigation. `TerrainApp` owns the generator, Map Builder, Settlement Builder and
legacy project workflows; its subtree has the `terrain-app` styling boundary.
`style.css` is scoped to that boundary and cannot style a worldbuilding heading,
canvas or control by accident.

`worldbuilding/ui.css` owns forest/gold design tokens, typography, control states,
focus indicators, shared section tabs, dialog sizes and responsive navigation.
Workspace styles own composition: Atlas tiles, character graphs, institution
directories, conceptual articles, Biosphere panels, System Designer panes and
Library archives/editors. Preserve those differences when adding a page.

Terrain tools reuse the same color, radius, focus and motion tokens at a denser
32px control size. Their canvas layout and compact tool-panel typography remain
separate from worldbuilding pages.

The `flora` route remains an alias for Biosphere. `atlas` and the World root both
open Atlas. Old `/world/:project/...` routes remain supported by TerrainApp.
Terrain and Map records are specialized persisted data, not sidebar categories.

## Controls and dialogs

Use explicit input types, visible labels and optional-field help. Shared controls
apply inside `history-app` and the portal's `ui-root`, not arbitrary documents.
Document paper and spreadsheet cells have explicit presentation exceptions.
Use `primary`, `danger`, `quiet` or `ui-icon-button` for action emphasis.

Use `ui/Dialog.jsx` for overlays. Pass a title, `onClose`, body children, explicit
`actions` and, for forms, an asynchronous `onSubmit`. Return/await the save promise
so duplicate submission is prevented and failures remain in the dialog. Choose
`compact` (480px), `form` (680px), `editor` (840px) or `wizard` (980px). The wizard
keeps a stable height across steps. Only the body scrolls; actions stay available.

Mark Cancel/Close footer buttons `data-dialog-dismiss`. The component then applies
the same dirty-draft guard as Escape and the header close button. Background clicks
do not dismiss drafts. Focus is trapped, the application is inert, scrolling is
locked, and focus returns to the opener. Nested confirmations preserve that state.
Dialogs must not depend on selectors such as `> div` to discover their action row.

Non-React terrain code and React pages use `confirmAction`, `requestText` and
`inform` from `ui/feedback.js`. They return promises: always await the result.
Browser-native prompts remain only for the browser's reload/close protection,
where a custom asynchronous dialog cannot safely replace `beforeunload`.

`useUnsavedChanges` registers an asynchronous navigation guard and returns the
same guard for local record switches. A confirmed discard clears the guard before
navigation continues, avoiding a second prompt. App navigation, Age switches and
browser Back/Forward consult the registry. Writing remains independent of in-world
records, even though it shares this interface behavior.

## Adding or changing a workspace

1. Choose a composition appropriate to its task; reuse controls and tokens.
2. Keep queries scoped to World and Age and preserve existing persistence paths.
3. Use shared Dialog, feedback and draft guards rather than bespoke overlays.
4. Include pending, validation, error, empty and successful-save feedback.
5. Check menus, long content, nested editors, cancellation and keyboard operation.
6. Test 1440px, 900px and 390px widths. Tables/graphs may scroll locally; pages
   must not acquire horizontal overflow. Keep saves above the fixed Age footer.

Run `npm run format:ui`, `npm run check:ui`, `npm test` and `npm run build`.
The UI check catches escaped terrain CSS, legacy dialogs/native prompts and
duplicated shared tabs. `tests/ui.test.mjs` exercises focus, modal structure,
submission failures, nested confirmations and navigation guards. Existing restart,
ZIP, parity and reference tests remain part of the normal suite.

For real-browser workflow and screenshot checks, run a local server with an
expendable populated world (accepted terrain, system, organization, place and
Library document examples), set `KRIEMHILD_UI_URL` and `KRIEMHILD_UI_WORLD`, and run
`npm run test:ui:browser`. The default browser channel is installed Chrome;
`KRIEMHILD_UI_BROWSER` can select another installed Playwright channel. Results,
failure traces and screenshots are written beneath ignored `.tools/` directories.
The Library round-trip test creates and deletes its own temporary archive and file;
other checks discard drafts and generation previews. They do not replace terrain.
