/**
 * Helpers for writing a theme file.
 *
 * A theme file describes its art direction once (medium, palette words, mood) plus one short
 * subject per image; `defineTheme` turns that into the seven `ThemeImage`s with prompts that
 * share the same style sentence word for word, so the set reads as one family.
 */
import type { Audience, CollectionId, IconName, Observance, IconStyle, LayoutId, QuickLinkSlot, Theme, ThemeImage, ThemePalette } from '@/designer/engine';

/**
 * Heading font stacks. Paper loads no third-party font CDN, so each stack is system faces first, then an OFL
 * look-alike (used only if Paper bundles it, see web/public/design-assets/fonts/), then the metric-compatible
 * faces ChromeOS and Linux ship (Tinos, Cousine), then a generic family. Single quotes only.
 * The engine skips the Lato and Helvetica stacks (they read as the default UI face).
 */
export const FONTS = {
  /** Canvas's own face. "Lato Extended" only exists on Canvas web; the apps embed plain "Lato". */
  lato: "'Lato Extended', Lato, 'Helvetica Neue', Helvetica, Arial, sans-serif",
  helvetica: "'Helvetica Neue', Helvetica, Arial, sans-serif",
  georgia: "Georgia, Gelasio, 'Times New Roman', Tinos, Times, serif",
  trebuchet: "'Trebuchet MS', 'Fira Sans', 'Lucida Grande', 'Lucida Sans Unicode', Arial, sans-serif",
  verdana: "Verdana, 'DejaVu Sans', Geneva, Tahoma, sans-serif",
  courier: "'Courier New', 'Courier Prime', Cousine, Courier, monospace",
} as const;

export interface SpotSpec {
  /** One simple subject, e.g. "a sea turtle". */
  subject: string;
  /** Screen-reader text. '' when the art adds nothing a reader would miss. */
  alt: string;
}

export interface ArtDirection {
  /** The shared style sentence, medium first. Repeated verbatim in all seven prompts. */
  medium: string;
  /** Palette in colour words that match palettes[0]. Repeated verbatim in all seven prompts. */
  palette: string;
  /** Mood sentence. Repeated verbatim in all seven prompts. */
  mood: string;
  /** Banner subject + where things sit (subject on the right, calm area on the left). */
  banner: string;
  /** A second banner composition in the same style. */
  bannerAlt: string;
  welcome: SpotSpec;
  /** An object or scene that stands in for the teacher. Never a face. */
  instructor: SpotSpec;
  /** Generic module/unit card art that suits any subject in the theme's tier. */
  cards: [string, string, string];
}

export interface ThemeSpec {
  id: string;
  name: string;
  description: string;
  audiences: Audience[];
  tags: string[];
  defaultLayout: LayoutId;
  /** [0] matches the art. Each may hand-tune its dark twin with `dark` (merged over darkPalette, then fixed). */
  palettes: ThemePalette[];
  iconStyle: IconStyle;
  headingFont?: string;
  corners: Theme['corners'];
  icons: Record<QuickLinkSlot, IconName>;
  collection?: CollectionId;
  premium?: boolean;
  observance?: Observance;
  /** Short name of the art direction, e.g. "layered paper-cut collage". */
  style: string;
  /** How the theme avoids relying on colour alone (e.g. red vs green). */
  colourBlind: string;
  art: ArtDirection;
}

export interface ThemeEntry {
  theme: Theme;
  style: string;
  colourBlind: string;
  art: ArtDirection;
}

/** Banner art is shown uncropped at 100% width, down to ~360px on phones. */
const BANNER_SCALE = 'The main subject is large and boldly shaped so it still reads when the banner is shown small on a phone';

function sentence(s: string): string {
  const t = s.trim();
  return /[.!?]$/.test(t) ? t : `${t}.`;
}

function prompt(art: ArtDirection, subject: string, extra?: string): string {
  return [art.medium, subject, extra, art.palette, art.mood]
    .filter((s): s is string => !!s)
    .map(sentence)
    .join(' ');
}

export function buildImages(art: ArtDirection): ThemeImage[] {
  const card = (n: number, subject: string): ThemeImage => ({
    id: `card-${n}`,
    kind: 'card',
    file: `card-${n}.webp`,
    width: 600,
    height: 400,
    alt: '',
    prompt: prompt(art, subject),
  });
  return [
    { id: 'banner', kind: 'banner', file: 'banner.webp', width: 1200, height: 300, alt: '', prompt: prompt(art, art.banner, BANNER_SCALE) },
    { id: 'banner-alt', kind: 'banner', file: 'banner-alt.webp', width: 1200, height: 300, alt: '', prompt: prompt(art, art.bannerAlt, BANNER_SCALE) },
    { id: 'spot-welcome', kind: 'spot', file: 'spot-welcome.webp', width: 600, height: 600, alt: art.welcome.alt, prompt: prompt(art, art.welcome.subject) },
    { id: 'spot-instructor', kind: 'spot', file: 'spot-instructor.webp', width: 600, height: 600, alt: art.instructor.alt, prompt: prompt(art, art.instructor.subject) },
    card(1, art.cards[0]),
    card(2, art.cards[1]),
    card(3, art.cards[2]),
  ];
}

export function defineTheme(spec: ThemeSpec): ThemeEntry {
  const { style, colourBlind, art, ...rest } = spec;
  const theme: Theme = { ...rest, images: buildImages(art) };
  if (theme.headingFont === undefined) delete theme.headingFont;
  return { theme, style, colourBlind, art };
}
