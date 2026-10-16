# Page Designer

A built-in designer for course pages: home, start here, course overview, module overview and meet your
instructor. A teacher picks a theme and layout, edits content in place, and saves a real wiki page. No HTML.

- **122 themes** in seven collections (Essentials, Signature, Arcade & Games, Heroes & Comics, Cartoons &
  Storybooks, Holidays & Festivals, Special Days) across six audience tiers, each with three colourways and a
  matched set of text-free artwork. Holidays carry observance dates per country for the "Coming up" shelf.
- **7 layouts**, independent of theme: classic band, split hero, card grid, sidebar, timeline, editorial,
  playful.
- **Light and dark mode.** Every theme has an AA-checked dark colourway that follows the user's Paper theme.
- **Portable.** A designed page is still an ordinary wiki page. Its HTML exports to Common Cartridge and
  opens in Canvas looking the way it does in Paper's light mode.

This document is the contract between the parts. Change it in the same PR as any change to the shapes below.

## Parts

| Path | What |
| --- | --- |
| `web/src/designer/engine/` | Rendering engine (TypeScript, no framework): `PageDoc` + `Theme` → `{ html, css }`, layouts, accessibility checker, palette maths, starter copy (en/es/pt), icon geometry. `types.ts` is the document contract. |
| `web/src/designer/themes/` | Theme catalog (TypeScript): palettes, art direction, icons, observances. |
| `web/src/designer/ui/` | Designer UI (React, JSX, shadcn + Tailwind). Entry page: `web/src/pages/PageDesignerPage.jsx`. |
| `web/public/design-assets/themes/<themeId>/` | Theme artwork, WebP at 1x and `@2x`. Served statically at `/design-assets/…`. |
| `internal/security/htmlsanitize/` | Rich-content HTML policy (inline CSS allowlist with value validation). |
| `internal/security/csssanitize/` | Page stylesheet sanitizer and scoper. |
| `internal/designassets/` | Icon SVG rendering (Lucide geometry, four styles) for `/api/v1/design/icons/…`. |

The engine and themes are TypeScript; `npm run typecheck:designer` checks them and `npm test` runs their
tests. Everything else in `web/` stays JSX.

## Styling model: three layers

1. **Inline styles** (every rich-content field). A wider allowlist than Canvas: layout (`display`, flex,
   grid, `gap`, alignment), `position` (not `fixed`/`sticky`) with offsets and `z-index`, backgrounds
   (`url()` limited to https and Paper's own files/assets, gradients, size/position/repeat), `font-weight`,
   `letter-spacing`, `text-transform`, `text-shadow`, `box-shadow`, `opacity`, `transform`, `transition`,
   `filter`, `aspect-ratio`, `object-fit`, logical properties, `calc()`/`min()`/`max()`/`clamp()`/`var()`.
   Values are validated, not just property names. Only `background` and `background-image` may load a URL
   (`url()`/`image-set()`); `filter`, `border-image*`, `list-style*`, `clip-path`, `shape-outside` and the
   rest keep their non-URL values, and `cursor` takes keywords only. Alt text and titles keep any printable
   text.
2. **Page stylesheet** (wiki pages only, `wiki_pages.css`). Real CSS, sanitized on save and scoped on
   output to `#paper-page-<id>`. Teachers only. Setting `pages.custom_css_enabled` (default on), resolved
   for the **course's account** (the value course show reports, not the editor's own account), turns off
   hand-written CSS: a stylesheet is then accepted only together with a design, and a page without a design
   carries none (see "API" and "Other write paths"). Designer-generated CSS still applies. See
   "The custom CSS setting" for how it resolves and who can change it.
3. **Dark mode.** Paper's dark mode is the `dark` class on `<html>`. In a page stylesheet,
   `@media (prefers-color-scheme: dark)` is rewritten to follow that class (and `light` to its absence), so
   authors write standard CSS and it tracks the user's Paper toggle, not just the OS.

The page root is a containment box (`contain: layout paint; isolation: isolate`), so nothing inside a page,
including `position: fixed` from its stylesheet, can draw over Paper's own interface. Every rich-content
viewer applies the same containment.

**Email is different.** A mail client applies none of that containment, so outgoing email bodies
(`htmlsanitize.SanitizeEmail`, used by every notification email) keep the same elements but only the
conservative inline styles rich content had before the designer: typography, colour, box model and alignment.
No `position`, offsets, `z-index`, transforms, and no URL-loading values.

### Page stylesheet rules (`csssanitize`)

```go
// Sanitize parses author CSS and returns it normalized, keeping only what is allowed.
// It is idempotent. ErrTooLarge when the input exceeds MaxBytes (200 KB), or when the
// normalized (reformatted, so larger) sheet would exceed MaxScopeBytes (400 KB).
func Sanitize(css string) (string, error)

// Resanitize re-applies the rules to a stored, already-sanitized sheet (input up to MaxScopeBytes).
func Resanitize(css string) (string, error)

// Scope rewrites sanitized CSS so it applies only inside root (e.g. "#paper-page-42").
// Input up to MaxScopeBytes, so a sheet that grew past MaxBytes when sanitized still scopes.
func Scope(css, root string) string
```

- Every selector is prefixed with `root` as an ancestor. A bare `:root`, `html`, `body` or `html body`
  becomes `root` itself; a qualified one keeps its qualifier on the real element (`html.dark p` →
  `html.dark root p`). Those qualifiers may only be classes and `[attr]` / `[attr=value]` tests: any other
  (`html:has(…)`, `body:not(.x)`, `html[lang^=e]`) drops the selector, since it would test Paper's document
  outside the page (and a `:has()` on the root could look at its siblings). Selectors that reach siblings of
  the root (`:root + p`, a leading `~`) are dropped. Write light-only rules with
  `@media (prefers-color-scheme: light)`, not `html:not(.dark)`.
- Inside `:is()`, `:where()` and `:not()` a selector may not use a combinator (descendant, `>`, `+`, `~`),
  and nothing inside any selector function may name `html`, `body`, `:root` or `:scope`;
  `:nth-child(An+B of S)` is refused. Such a selector is dropped (`p:is(html:has(a[href*=x]) *)`).
  `:has()` elsewhere is fine (`.card:has(img)`, `li:has(> img)`).
- Rules whose subject is the root itself may only set custom properties and properties that paint inside
  the box (colour, fonts, backgrounds, borders, padding, text and gap properties), so a stylesheet can't
  undo the containment with `position: fixed`, `display: contents` and the like.
- `@media (prefers-color-scheme: dark)` → rules prefixed `html.dark root`; `light` → `html:not(.dark) root`.
  Other `@media` conditions are kept. Allowed at-rules: `@media`, `@supports`, `@container`, `@keyframes`
  (names are prefixed per page; `animation`/`animation-name` references are rewritten to match).
- Dropped: `@import`, `@font-face`, `@namespace`, `@charset`, `@page`, `@layer`, `@property`, unknown at-rules,
  `expression()`, `-moz-binding`, `behavior`, `javascript:`, and `url()` other than `https:`,
  `/api/v1/files/<id>/download`, `/api/v1/design/icons/…` and `/design-assets/…`.
- A value may load a URL only in `background` / `background-image`, and in `content` on a rule whose
  selectors all target `img` (the designer's `img[data-pd-icon]{content:url(…)}` dark-mode icon swap). Not in
  `filter`, `border-image`, `list-style`, `cursor`, `mask`, `clip-path`, nor a custom property (`var()` could
  carry it anywhere).
- `</style` can never appear in output. Comments are removed and the sheet is reformatted, so the
  designer keeps the teacher's own source in `doc.customCss`.
- Values use the same validator as inline styles (`htmlsanitize.CSSValueOK`): `@` only inside URLs (for
  `@2x` art), no backslash escapes, functions from an allowlist.

## The custom CSS setting

`pages.custom_css_enabled` (Settings Engine, `internal/service/settings/catalog.go`; bool, instance and
account scope, default `true`) controls **hand-written** CSS: Page CSS on ordinary pages and Custom CSS in
the designer's Advanced tab. Designed pages keep their generated colours, dark mode and layout either way.

**Off anywhere above wins** (`Definition.OffLocksBelow`). For an account, the effective value is `false` if
the instance row is `false` or any account in its chain (the account itself and every
`parent_account_id` ancestor) has an explicit `false`; otherwise the nearest explicit `true`, else the
default. A sub-account can turn it off under a parent that allows it, but can't turn it back on under a
parent (or the instance) that turned it off. `settings.Service.Get` applies this, so the shared settings
lookup, course show, the page API, every other write path and the super-admin panel agree. A failed lookup
falls back to on (and is logged), as before.

**Who sets it.** Super-admins at any scope (`/superadmin/settings`). Tenant admins for their own account,
on **Admin → Settings → Pages** (`web/src/components/admin/PageCssSettings.jsx`), through the account-admin
settings API:

- `GET /api/v1/accounts/:account_id/settings` → one entry per key with `AccountAdminEditable` (today only
  this one):

  ```json
  [{ "key": "pages.custom_css_enabled", "label": "Hand-written page CSS", "description": "…",
     "value": null, "effective": false, "locked": true,
     "locked_by": { "scope": "account", "account_id": 4, "account_name": "Example District" } }]
  ```

  `value` is this account's own row (`null` = inherits). `locked_by` is the highest level above the
  account holding it off: `{ "scope": "instance" }` or an ancestor account; `null` when not locked (the
  account's own `false` is not a lock).
- `PUT /api/v1/accounts/:account_id/settings/:key` with `{ "value": true | false | null }`. `null` clears
  the account's row (inherit). `true` while locked is 422 ("Turned off by Example District…" /
  "…by your Paper administrator"). A key that isn't account-admin editable is 404; a value of the wrong
  JSON type is 400. Writes emit the `setting_change` audit row with the admin's user id.

Both are behind `RequireAdmin`: a tenant admin for their own account (another account is 404, a teacher 403),
a super-admin for any existing account. Sub-account administration isn't built yet; when it is, a
parent-account admin acting on a descendant needs only the access check in
`AccountSettingsHandler.settingsAccount` widened.

## Data model

| Column | Type | What |
| --- | --- | --- |
| `wiki_pages.css` | `text NOT NULL DEFAULT ''` | Sanitized, unscoped page stylesheet. |
| `wiki_pages.css_source` | `text NOT NULL DEFAULT ''` | The stylesheet as the teacher typed it (comments, formatting), for the editor only. Never served to readers. |
| `wiki_pages.design` | `jsonb NULL` | The `PageDoc` a designed page was made from, cleaned by the server (below). `NULL` = an ordinary page. Sent to course staff only. |
| `wiki_page_revisions.css`, `.design` | same | Snapshotted with every revision, restored with it. |
| `courses.look` | `jsonb NULL` | The course's look (below). `NULL` = Paper's generated header. |
| `design_colour_schemes` | table | A teacher's saved palettes: `id, account_id, user_id, name, palette jsonb, created_at, updated_at`. |

`courses.look` is denormalized so the course header never loads the theme catalog:

```json
{
  "version": 1,
  "theme_id": "tide-pool",
  "palette_id": "default",
  "colors": { "primary": "#12355b", "on_primary": "#ffffff", "secondary": "…", "on_secondary": "…",
              "accent": "…", "surface": "…", "on_surface": "…", "background": "…", "text": "…", "link": "…" },
  "dark_colors": { "…same keys…": "…" },
  "banner": { "src": "/design-assets/themes/tide-pool/banner.webp",
              "src_2x": "/design-assets/themes/tide-pool/banner@2x.webp", "alt": "" }
}
```

The server validates it: `theme_id` matches `^[a-z0-9-]{1,64}$`, colours are `#rrggbb`, banner paths are
under `/design-assets/` or are `/api/v1/files/<id>/download` for a file in this course, total size ≤ 16 KB.

## API (additions to the Canvas-compatible shapes)

**Pages** (`/api/v1/courses/:course_id/pages…`)

- Show (including the front page and the anonymous public page) returns `css` (stored), `scoped_css` (scoped
  to `#paper-page-<id>`) and `has_design`. Course staff (who can see unpublished content) also get `design`
  (object or `null`) and `css_source` while it still sanitizes to `css`; readers get neither, and render
  `body` + `scoped_css`. Clients decide "is this a designed page" from `has_design`.
  List returns `has_design` and omits `css`/`design`.
- Revisions return `has_design` and, with the body, `css`; `design` for course staff only; never `css_source`.
- Create/Update accept `wiki_page[css]` and `wiki_page[design]` (`null` clears it, turning the page back into
  an ordinary page). Only users who can manage course content may set either; a student editor sending them
  gets 403. With `pages.custom_css_enabled` off for the course's account, `css` is accepted only together
  with a non-null `design`, and a request that sets `design: null` also clears `css` and `css_source`
  (hand-written CSS can't outlive the design). With it on, a detach without `css` keeps the stylesheet.
- `design` must be a JSON object with `version: 1` and a string `id`, ≤ 256 KB (else 422). The rest is
  cleaned, not refused (`designdoc.ValidateDesign`), against `engine/types.ts`, because the designer writes
  some fields straight into the page:
  - unknown section types and members, and unknown top-level members, are dropped; at most 60 sections and
    100 entries per list;
  - RichText (`welcome`/`callout`/`text` `body`, `instructor.bio`) keeps only `p, strong, em, a[href], ul, ol,
    li, br`, links passing the engine's `safeHref` rule;
  - link targets (`quick-links.links[].href`, `schedule.items[].href`, `this-week.items[].href`) are kept only
    if `safeHref` accepts them (https, http, mailto, relative incl. `{course}`), else the field is dropped;
  - images (`banner.customSrc`, `instructor.photoSrc`) only if `https://…`, `/api/v1/files/<id>/download…` or
    `/design-assets/…` (no http or protocol-relative upgrade);
  - `palette` only if it passes the colour-scheme validation;
  - plain strings lose control characters and are cut to 200 (ids, headings, labels) or 2,000 characters;
    `customCss` stays text (it is sanitized when it becomes `css`), cut to 200 KB.

  A design the engine produced comes back with the same values, so `data-pd-hash` still matches.

**Other write paths.** Revision restore, Commons import, blueprint sync, course copy, batch clone and Common
Cartridge import store a page's design and stylesheet without the page API. Each re-cleans the design
(`ValidateDesign`) and re-sanitizes the stylesheet, and when `pages.custom_css_enabled` is off for the
destination course's account, a page that ends up without a design gets no stylesheet (nor `css_source`).

**Courses**: show includes `look` and `custom_css_enabled` (the resolved `pages.custom_css_enabled` setting for
the course's account, "off anywhere above wins"); update accepts `course[look]` (`null` clears it).

**Colour schemes**: `GET/POST /api/v1/users/self/design_colour_schemes`,
`PUT/DELETE /api/v1/users/self/design_colour_schemes/:id`. Body `{ name, palette }`; palette keys as in
`engine/types.ts` `Palette`. At most 50 per user.

**Icons**: `GET /api/v1/design/icons/:style/:name.svg?fg=rrggbb&bg=rrggbb`. Public, no auth.
`style` is `solid-circle | soft-square | outline | duotone`; unknown names render `circle-dashed`. Output is
byte-identical to the engine's `renderIconSvg`. `Cache-Control: public, max-age=31536000, immutable`,
`Content-Security-Policy: default-src 'none'; style-src 'unsafe-inline'`.

## Engine output

```ts
renderDesign(doc: PageDoc, theme: Theme, opts: RenderOptions): { html: string; css: string }
// opts: { assetBase: '/design-assets', iconBase: '/api/v1/design/icons', courseId?: string, dark?: boolean }
```

- `html` is the page body. The root is `<div id="pd-<doc.id>" class="paper-design" data-pd-doc="…"
  data-pd-v="1" …>`. Colours that differ between the light and dark colourways are written as
  `var(--pd-cN, #light)`, so the HTML alone renders the light page.
- `css` is the designer stylesheet, written against `:root` (the page): dark-mode custom properties inside
  `@media (prefers-color-scheme: dark)`, hover and focus states, `prefers-reduced-motion` handling, and the
  heading font as `font-family: var(--reading-font-family, <theme font>)` so a reader's dyslexia font wins.
- The teacher's own CSS (`doc.customCss`, Advanced) is appended after it by the designer.

**Export to Common Cartridge**: `var(--pd-…, #hex)` is flattened to `#hex`, and `/design-assets/…` and
`/api/v1/design/icons/…` URLs become absolute (`FRONTEND_URL`). The page stylesheet and design travel in
Paper's own cartridge metadata so a Paper → Paper copy keeps them; Canvas ignores them.

## Where teachers find it

- **Pages**: "Design a page". Designed pages show a badge; "Edit" opens the designer, and "Edit as HTML"
  detaches the design after a warning.
- **Course home** (teachers): "Design your home page" when the course has no front page.
- **Whole-course makeover** (`/courses/:courseId/designer/makeover`): one look for Home, Start Here, Course
  overview, Meet your instructor and an overview per module, linked to each other, set as the front page,
  and applied as the course look.
- **Course settings → Look**: the course header banner and colours.
- Route: `/courses/:courseId/designer` (`?page=<slug>` edits a page, `?kind=home|start-here|overview|module|instructor` starts one).
