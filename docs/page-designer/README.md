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
   Values are validated, not just property names. Alt text and titles keep any printable text.
2. **Page stylesheet** (wiki pages only, `wiki_pages.css`). Real CSS, sanitized on save and scoped on
   output to `#paper-page-<id>`. Teachers only. Account setting `pages.custom_css_enabled` (default on) turns
   off hand-written CSS; designer-generated CSS still applies.
3. **Dark mode.** Paper's dark mode is the `dark` class on `<html>`. In a page stylesheet,
   `@media (prefers-color-scheme: dark)` is rewritten to follow that class (and `light` to its absence), so
   authors write standard CSS and it tracks the user's Paper toggle, not just the OS.

The page root is a containment box (`contain: layout paint; isolation: isolate`), so nothing inside a page,
including `position: fixed` from its stylesheet, can draw over Paper's own interface. Every rich-content
viewer applies the same containment.

### Page stylesheet rules (`csssanitize`)

```go
// Sanitize parses author CSS and returns it normalized, keeping only what is allowed.
// It is idempotent. ErrTooLarge when the input exceeds MaxBytes (200 KB).
func Sanitize(css string) (string, error)

// Scope rewrites sanitized CSS so it applies only inside root (e.g. "#paper-page-42").
func Scope(css, root string) string
```

- Every selector is prefixed with `root` as an ancestor. `:root`, `html` and `body` (alone or compound at
  the start) become `root` itself.
- `@media (prefers-color-scheme: dark)` → rules prefixed `html.dark root`; `light` → `html:not(.dark) root`.
  Other `@media` conditions are kept. Allowed at-rules: `@media`, `@supports`, `@container`, `@keyframes`
  (names are prefixed per page; `animation`/`animation-name` references are rewritten to match).
- Dropped: `@import`, `@font-face`, `@namespace`, `@charset`, `@page`, `@layer`, `@property`, unknown at-rules,
  `expression()`, `-moz-binding`, `behavior`, `javascript:`, and `url()` other than `https:`,
  `/api/v1/files/<id>/download`, `/api/v1/design/icons/…` and `/design-assets/…`.
- `</style` can never appear in output.

## Data model

| Column | Type | What |
| --- | --- | --- |
| `wiki_pages.css` | `text NOT NULL DEFAULT ''` | Sanitized, unscoped page stylesheet. |
| `wiki_pages.design` | `jsonb NULL` | The `PageDoc` a designed page was made from. `NULL` = an ordinary page. |
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

- Show returns `css` (stored), `scoped_css` (scoped to `#paper-page-<id>`) and `design` (object or `null`).
  List returns `has_design` and omits `css`/`design`.
- Create/Update accept `wiki_page[css]` and `wiki_page[design]` (`null` clears it, turning the page back into
  an ordinary page). Only users who can manage course content may set either; a student editor sending them
  gets 403. With `pages.custom_css_enabled` off, `css` is accepted only together with a non-null `design`.
- `design` must be a JSON object with `version: 1` and a string `id`, ≤ 256 KB.

**Courses**: show includes `look` and `custom_css_enabled` (the resolved `pages.custom_css_enabled` setting for
the course's account); update accepts `course[look]` (`null` clears it).

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
