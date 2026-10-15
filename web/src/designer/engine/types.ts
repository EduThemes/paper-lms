/**
 * RMT Studio — shared contract.
 *
 * Every package builds against these types:
 *   - @rmt/core    implements render / a11y / starter content / icons
 *   - @rmt/themes  supplies Theme data (look) — palettes, art, icon style
 *   - builder app  edits a PageDoc and calls core to preview / export
 *   - server app   serves builder + assets, does LTI 1.3, serves icon SVGs
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
  background: string; // page background (Canvas pages are white; keep near-white)
  text: string;
  link: string; // must reach 4.5:1 on background AND surface
}

export interface ThemeImage {
  id: string; // e.g. "banner", "banner-alt", "spot-instructor"
  kind: 'banner' | 'spot' | 'card';
  /** File name under apps/server/public/assets/themes/<themeId>/ */
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
  palettes: Palette[]; // [0] is the default colourway
  iconStyle: IconStyle;
  /** Font stack. Canvas keeps inline font-family; first entry should be a web-safe or Canvas-loaded font. */
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
}

export interface RenderOptions {
  /** Absolute base for images/icons, no trailing slash, e.g. "https://rmt.eduthemes.org/assets". */
  assetBase: string;
  /** Canvas course id, substituted for {course} in link hrefs when known. */
  courseId?: string;
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
 * The public API of @rmt/core (implemented in src/index.ts).
 *
 *   createPage(theme: Theme, kind: PageKind, locale: Locale, layout?: LayoutId): PageDoc
 *   renderPage(doc: PageDoc, theme: Theme, opts: RenderOptions): string        // Canvas-safe HTML fragment
 *   checkAccessibility(doc: PageDoc, theme: Theme): A11yIssue[]
 *   contrastRatio(fg: string, bg: string): number
 *   derivePalette(base: { primary; secondary?; accent? }, name: string, id?: string): Palette   // AA-passing
 *   checkPalette(p: Palette): PairResult[]      fixPalette(p: Palette): Palette      sanitizePalette(x: unknown): Palette | null
 *   sanitizeRich(html: string): RichText
 *   renderIconSvg(name: IconName, o: { style: IconStyle; fg: string; bg: string; size?: number }): string
 *   iconUrl(assetBase: string, name: IconName, o: { style: IconStyle; fg: string; bg: string }): string
 *     -> `${assetBase}/icons/${style}/${name}.svg?fg=<hex no #>&bg=<hex no #>`
 *   themeImageUrl(assetBase: string, themeId: string, image: ThemeImage): string
 *     -> `${assetBase}/themes/${themeId}/${image.file}`
 *   LAYOUTS: { id: LayoutId; name: string; description: string }[]
 *   SECTION_TYPES: { type: SectionType; label: string; description: string }[]
 *   strings(locale: Locale): Record<string, string>                           // UI-facing starter copy
 */
