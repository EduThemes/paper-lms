/**
 * Paper Page Designer — engine contract (see docs/page-designer/README.md).
 *
 *   - engine (this folder)  render / a11y / starter content / icons / palette maths
 *   - themes                Theme data (look): palettes, art, icon style
 *   - designer UI           edits a PageDoc and calls the engine to preview / save
 *   - server                stores the page HTML + stylesheet, serves icon SVGs and theme art
 *
 * A *theme* is the look (palette, art, icon style, type). A *layout* is the
 * structure. They are independent: any theme can be rendered with any layout.
 *
 * Change this file only by agreement — it is the seam between parallel work.
 */

export type Locale = 'en' | 'es' | 'pt';

/** Visual tiers. Research: K-2 and 3-5 need different treatments (reading level, icon size, tone). */
export type Audience =
  | 'early-elementary' // K-2: picture-first, very large tap targets, minimal text
  | 'upper-elementary' // 3-5: playful but more text, still big targets
  | 'secondary' // 6-12
  | 'higher-ed'
  | 'professional' // PD, corporate training, faculty development
  | 'specialty'; // counseling, library, athletics, CTE, healthcare programs

/** Structural families. Each nods to a Canvas RMT layout but is its own design. */
export type LayoutId =
  | 'classic-band' // title band + banner + welcome + icon tiles (the familiar RMT homepage, elevated)
  | 'split-hero' // banner beside welcome copy on desktop, stacks on mobile
  | 'card-grid' // modular cards: icon + label + one-line description
  | 'sidebar' // main column + side rail (instructor card, quick links, dates)
  | 'timeline' // weeks/modules as a vertical schedule
  | 'editorial' // large type, generous whitespace, minimal chrome
  | 'playful'; // chunky rounded tiles, big icons — primary grades

/** Page types a course needs. Each layout must render every kind. */
export type PageKind =
  | 'home' // course front page
  | 'start-here' // getting started / orientation
  | 'overview' // course overview / syllabus summary
  | 'module' // module or unit overview
  | 'instructor'; // meet your instructor

/** Shelves in the gallery. 'essentials' is the original 30-theme catalog. */
export type CollectionId =
  | 'essentials' // everyday themes by audience tier
  | 'signature' // premium, high-craft looks
  | 'arcade' // video game, pixel and arcade themes (Quantitown energy)
  | 'heroes-comics' // original superheroes, comic and graphic-novel looks
  | 'cartoons' // cartoon and storybook worlds
  | 'holidays' // religious and cultural holidays from many traditions
  | 'special-days'; // school events, heritage months, awareness days

/** A holiday or special day a theme is made for. Dates move (lunar calendars), so store months, not dates. */
/** Countries whose school calendars the builder knows (ISO 3166-1 alpha-2). */
export type Region = 'US' | 'CA' | 'GB' | 'IE' | 'AU' | 'NZ';

export interface Observance {
  name: string; // e.g. "Diwali"
  /** Traditions/communities that observe it, e.g. ["Hindu", "Sikh", "Jain"]. Empty for civic/school days. */
  traditions: string[];
  /** Months (1-12) when it usually falls, so the gallery can surface what's coming up. */
  months: number[];
  /**
   * Where a country's calendar differs (Black History Month is February in the US, October in the UK;
   * the long summer break is December in Australia), the months for that country. Countries not listed
   * use `months`.
   */
  regionalMonths?: Partial<Record<Region, number[]>>;
  /** One line a teacher sees, e.g. "Date follows the Hindu lunar calendar (Oct or Nov)". */
  note?: string;
}

export type IconStyle = 'solid-circle' | 'soft-square' | 'outline' | 'duotone';

/** Lucide icon name, e.g. "book-open", "calendar-days". */
export type IconName = string;

/**
 * All colours are hex. Pairs named `on*` are the text colour used on top of the
 * base colour and MUST reach WCAG AA 4.5:1 against it. `text` on `background`
 * and `text` on `surface` must too. `accent` is decorative only — never text.
 */
export interface Palette {
  id: string;
  name: string;
  primary: string;
  onPrimary: string;
  secondary: string;
  onSecondary: string;
  accent: string;
  surface: string; // tinted panel background
  onSurface: string;
  background: string; // page background (light pages are white; keep near-white)
  text: string;
  link: string; // must reach 4.5:1 on background AND surface
}

/** The ten colours of a palette, without id and name. */
export type PaletteColors = Omit<Palette, 'id' | 'name'>;

/**
 * A theme's colourway. `dark` optionally hand-tunes the derived dark colourway: these colours are merged over
 * `darkPalette(palette)` and the result is fixed to pass every PALETTE_PAIRS rule.
 */
export interface ThemePalette extends Palette {
  dark?: Partial<PaletteColors>;
}

export interface ThemeImage {
  id: string; // e.g. "banner", "banner-alt", "spot-instructor"
  kind: 'banner' | 'spot' | 'card';
  /** File name under web/public/design-assets/themes/<themeId>/ (a `<name>@2x.<ext>` twin sits beside it). */
  file: string;
  width: number;
  height: number;
  /** Written for screen-reader users, not the generator. Empty string = decorative. */
  alt: string;
  /** Generation prompt for kie.ai. Must say no text / no logos. */
  prompt: string;
}

export interface Theme {
  id: string; // kebab-case, stable — used in asset paths
  name: string;
  description: string;
  audiences: Audience[];
  tags: string[];
  defaultLayout: LayoutId;
  palettes: ThemePalette[]; // [0] is the default colourway
  iconStyle: IconStyle;
  /** Heading font stack. Rendered in the designer stylesheet, never inline, so a reader's own font wins. */
  headingFont?: string;
  corners: 'square' | 'soft' | 'round';
  images: ThemeImage[];
  /** Gallery shelf. Defaults to 'essentials' when absent. */
  collection?: CollectionId;
  /** Paid tier. The builder shows a badge; enforcement is the server's job. */
  premium?: boolean;
  /** Set on holiday and special-day themes. */
  observance?: Observance;
  /** Theme-chosen icons for the default quick links, keyed by semantic slot. */
  icons?: Partial<Record<QuickLinkSlot, IconName>>;
}

export type QuickLinkSlot =
  | 'start'
  | 'syllabus'
  | 'modules'
  | 'assignments'
  | 'discussions'
  | 'grades'
  | 'help'
  | 'instructor'
  | 'calendar'
  | 'resources';

/** Restricted rich text: HTML limited to p, strong, em, a[href], ul, ol, li, br. core.sanitizeRich() enforces it. */
export type RichText = string;

export interface QuickLink {
  id: string;
  label: string;
  /** When set, the link was generated from this Canvas module (builder can refresh it). */
  moduleId?: string;
  href: string; // Canvas-relative ("/courses/:id/modules") or absolute; may contain the token {course}
  icon: IconName;
  description?: string;
  slot?: QuickLinkSlot;
}

export type Section =
  | { id: string; type: 'title'; text: string; subtitle?: string }
  | { id: string; type: 'banner'; imageId?: string; customSrc?: string; alt: string }
  | { id: string; type: 'welcome'; heading: string; body: RichText }
  | { id: string; type: 'quick-links'; heading: string; links: QuickLink[] }
  | {
      id: string;
      type: 'instructor';
      heading: string;
      name: string;
      role?: string;
      email?: string;
      officeHours?: string;
      photoSrc?: string;
      photoAlt?: string;
      bio: RichText;
    }
  | { id: string; type: 'callout'; variant: 'note' | 'tip' | 'important'; heading: string; body: RichText }
  | { id: string; type: 'objectives'; heading: string; items: string[] }
  | {
      id: string;
      type: 'schedule';
      heading: string;
      items: { id: string; title: string; dates?: string; summary?: string; href?: string }[];
    }
  | {
      id: string;
      type: 'this-week';
      heading: string; // e.g. "This week"
      title: string; // e.g. "Week 3: Cell structure"
      dates?: string;
      items: { id: string; label: string; href?: string; due?: string }[];
      /** When set, the builder refreshes items from this Canvas module id. */
      moduleId?: string;
    }
  | { id: string; type: 'text'; heading?: string; body: RichText }
  | { id: string; type: 'divider' };

export type SectionType = Section['type'];

export interface PageDoc {
  version: 1;
  id: string; // uuid; also written into the output as an element id so a page can be re-opened
  themeId: string;
  /** One of the theme's built-in palettes. Kept even when `palette` is set, as the fallback. */
  paletteId: string;
  /**
   * The teacher's own colours (a custom or "My colour schemes" palette). When present and valid, the renderer
   * uses it instead of the theme palette named by `paletteId`. Same contrast rules as Palette. Optional, so
   * docs saved before custom colours load and render exactly as before.
   */
  palette?: Palette;
  layout: LayoutId;
  kind: PageKind;
  locale: Locale;
  sections: Section[];
  options: {
    density: 'comfortable' | 'compact';
    headingStyle: 'band' | 'underline' | 'plain';
  };
  /**
   * The teacher's own stylesheet (Advanced), appended after the designer's. Written against `:root` as the
   * page; Paper's server sanitizes and scopes it. The engine never reads it.
   */
  customCss?: string;
}

export interface RenderOptions {
  /** Base for theme art, no trailing slash, e.g. "/design-assets". Art is `${assetBase}/themes/<id>/<file>`. */
  assetBase: string;
  /** Base for icon SVGs, e.g. "/api/v1/design/icons". Defaults to `${assetBase}/icons`. */
  iconBase?: string;
  /** Course id, substituted for {course} in link hrefs when known. */
  courseId?: string;
  /**
   * Emit a dark colourway (default true): colours that differ between light and dark become
   * `var(--pd-cN, #light)` and the stylesheet sets them under `prefers-color-scheme: dark`. false = plain hex.
   */
  dark?: boolean;
}

/** renderDesign output: the page body and the designer stylesheet (written against `:root` = the page). */
export interface RenderResult {
  html: string;
  css: string;
}

/** `courses.look` colours, snake_case. */
export interface LookColors {
  primary: string;
  on_primary: string;
  secondary: string;
  on_secondary: string;
  accent: string;
  surface: string;
  on_surface: string;
  background: string;
  text: string;
  link: string;
}

/** The `courses.look` JSON (docs/page-designer/README.md, "Data model"). */
export interface CourseLook {
  version: 1;
  theme_id: string;
  palette_id: string;
  colors: LookColors;
  dark_colors: LookColors;
  /** Absent when the theme has no banner art. */
  banner?: { src: string; src_2x: string; alt: string };
}

export interface A11yIssue {
  id: string;
  severity: 'error' | 'warning' | 'suggestion';
  sectionId?: string;
  /** Plain-language message a teacher understands. */
  message: string;
  /**
   * Optional one-click fix the builder can offer. `patch` applies to the section named by `sectionId`;
   * `docPatch` applies to the whole doc (palette: null removes the custom palette).
   */
  fix?: { label: string; patch?: Partial<Section>; docPatch?: { palette: Palette | null } };
}

/**
 * The public API of the engine (implemented in index.ts).
 *
 *   createPage(theme: Theme, kind: PageKind, locale: Locale, layout?: LayoutId): PageDoc
 *   renderDesign(doc: PageDoc, theme: Theme, opts: RenderOptions): RenderResult   // { html, css }
 *   renderPage(doc: PageDoc, theme: Theme, opts: RenderOptions): string           // renderDesign(...).html
 *   darkPalette(p: Palette): Palette            themeDarkPalette(p: ThemePalette): Palette
 *   courseLook(theme: Theme, paletteId: string, palette?: Palette): CourseLook
 *   checkAccessibility(doc: PageDoc, theme: Theme): A11yIssue[]
 *   contrastRatio(fg: string, bg: string): number
 *   derivePalette(base: { primary; secondary?; accent? }, name: string, id?: string): Palette   // AA-passing
 *   checkPalette(p: Palette): PairResult[]      fixPalette(p: Palette): Palette      sanitizePalette(x: unknown): Palette | null
 *   sanitizeRich(html: string): RichText
 *   renderIconSvg(name: IconName, o: { style: IconStyle; fg: string; bg: string; size?: number }): string
 *   iconUrl(assetBase: string, name: IconName, o: { style: IconStyle; fg: string; bg: string }): string
 *     -> `${assetBase}/icons/${style}/${name}.svg?fg=<hex no #>&bg=<hex no #>`
 *   iconSrc(iconBase: string, name: IconName, o): string  -> `${iconBase}/${style}/${name}.svg?fg=…&bg=…`
 *   themeImageUrl(assetBase: string, themeId: string, image: ThemeImage): string
 *     -> `${assetBase}/themes/${themeId}/${image.file}`   (themeImageUrl2x -> `…/<name>@2x.<ext>`)
 *   LAYOUTS: { id: LayoutId; name: string; description: string }[]
 *   SECTION_TYPES: { type: SectionType; label: string; description: string }[]
 *   strings(locale: Locale): Record<string, string>                           // UI-facing starter copy
 */
