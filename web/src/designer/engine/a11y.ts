/**
 * Pre-insert accessibility checklist. Two passes:
 *  1. Content rules on the PageDoc (alt text, link text, headings, rich text, phone-width) — these can
 *     point at a section and often offer a one-click fix.
 *  2. Canvas's own RCE checker rules run on the rendered HTML (contrast with inherited colours and sizes,
 *     heading order, h1, img alt, adjacent links, list structure, tables), so teachers see the same
 *     verdicts Canvas will give them.
 */
import { contrastRatio, normHex } from './color';
import { fixPalette, isNearWhite, paletteFailures, sanitizePalette } from './palette';
import { attr, decodeEntities, parseHtml, parseStyle, textOf, walk, type HElement, type HNode } from './html';
import { renderPage } from './render';
import { safeHref, sanitizeRich } from './sanitize';
import { strings } from './strings';
import type { A11yIssue, PageDoc, QuickLink, RenderOptions, Section, Theme } from './types';

export interface A11yOptions {
  /** Known at insert time inside Canvas. Without it, `{course}` links are flagged. */
  courseId?: string;
  assetBase?: string;
}

const FILENAME_RE = /[^\s]+(.*?)\.(jpg|jpeg|png|gif|svg|bmp|webp)$/i; // Canvas img-alt-filename
const VAGUE = new Set([
  'click here', 'here', 'click', 'link', 'this link', 'more', 'read more', 'learn more', 'go', 'go here', 'this',
  'haz clic aquí', 'haga clic aquí', 'clic aquí', 'aquí', 'más', 'leer más', 'clique aqui', 'clique', 'aqui', 'mais', 'saiba mais', 'leia mais',
]);
const REDUNDANT_ALT = /^(an?\s+)?(image|picture|photo|graphic|imagen|foto|imagem|figura)\s+(of|de|da|do)\s+/i;
const LIST_LIKE = /^\s*(\d{1,3}[.)]|[a-zA-Z][.)]|[-*•–])\s+/;

const isAllCaps = (s: string) => {
  const letters = s.replace(/[^\p{L}]/gu, '');
  return letters.length >= 12 && letters === letters.toLocaleUpperCase() && letters !== letters.toLocaleLowerCase();
};
const sentenceCase = (s: string) => {
  const l = s.toLocaleLowerCase();
  return l.charAt(0).toLocaleUpperCase() + l.slice(1);
};

const DEFAULT_HEADING_KEY: Partial<Record<Section['type'], string>> = {
  welcome: 'home.welcome.heading',
  'quick-links': 'home.links.heading',
  instructor: 'home.instructor.heading',
  'this-week': 'home.week.heading',
  objectives: 'module.objectives.heading',
  schedule: 'overview.schedule.heading',
  callout: 'ui.note',
};

export function checkAccessibility(doc: PageDoc, theme: Theme, opts: A11yOptions = {}): A11yIssue[] {
  const issues: A11yIssue[] = [];
  const S = strings(doc.locale);
  const seen = new Map<string, number>();
  const add = (i: Omit<A11yIssue, 'id'> & { rule: string }) => {
    const base = `${i.rule}${i.sectionId ? `:${i.sectionId}` : ''}`;
    const n = (seen.get(base) ?? 0) + 1;
    seen.set(base, n);
    const { rule, ...rest } = i;
    void rule;
    issues.push({ id: n > 1 ? `${base}:${n}` : base, ...rest });
  };

  /* ------------------------------------------------------------ 0. custom colours */
  if (doc.palette !== undefined && doc.palette !== null) {
    const p = sanitizePalette(doc.palette);
    if (!p)
      add({
        rule: 'palette-invalid',
        severity: 'error',
        message: 'Your custom colours include something that isn’t a colour code, so the theme’s colours are used instead. Pick your colours again, or go back to the theme’s colours.',
        fix: { label: 'Use the theme’s colours', docPatch: { palette: null } },
      });
    else {
      const bad = paletteFailures(p);
      const fix = { label: 'Fix my colours', docPatch: { palette: fixPalette(p) } };
      if (bad.length)
        add({
          rule: 'palette-contrast',
          severity: 'error',
          message: `Some of your custom colours are hard to read together: ${bad
            .map(b => `${b.label.toLocaleLowerCase()} (${b.ratio.toFixed(2)}:1)`)
            .join(', ')}. Text needs a contrast of at least 4.5:1.`,
          fix,
        });
      if (!isNearWhite(p.background))
        add({
          rule: 'palette-background',
          severity: 'warning',
          message: 'Your page background colour is quite dark. Canvas pages are white, so a near-white background looks right in Canvas and its apps.',
          fix,
        });
    }
  }

  /* ------------------------------------------------------------ 1. content rules */
  if (!doc.sections.some(s => s.type === 'title'))
    add({ rule: 'no-title', severity: 'suggestion', message: 'Add a Course title section so the course name is real text that screen readers and search can use (not just part of a picture).' });

  const linkTexts = new Map<string, Set<string>>();
  let courseTokenSeen = false;
  const noteLink = (label: string, href: string) => {
    const key = label.trim().toLocaleLowerCase();
    if (!key) return;
    if (!linkTexts.has(key)) linkTexts.set(key, new Set());
    linkTexts.get(key)!.add(href.trim());
  };

  const checkHeading = (s: Section, text: string | undefined, field: string, required: boolean) => {
    const v = (text ?? '').trim();
    if (!v) {
      if (required) {
        const key = DEFAULT_HEADING_KEY[s.type];
        add({
          rule: 'empty-heading',
          severity: 'error',
          sectionId: s.id,
          message: 'This section has no heading. Headings let screen-reader users jump around the page.',
          fix: key ? { label: `Use “${S[key]}”`, patch: { [field]: S[key] } as Partial<Section> } : undefined,
        });
      }
      return;
    }
    if (v.length > 120)
      add({ rule: 'paragraphs-for-headings', severity: 'error', sectionId: s.id, message: `This heading is ${v.length} characters long. Canvas flags headings over 120 characters — keep it short and move the rest into the text.` });
    if (isAllCaps(v))
      add({
        rule: 'all-caps',
        severity: 'warning',
        sectionId: s.id,
        message: 'Long ALL-CAPS text is harder to read, and some screen readers spell it out letter by letter.',
        fix: { label: 'Change to sentence case', patch: { [field]: sentenceCase(v) } as Partial<Section> },
      });
  };

  const checkAlt = (s: Section, alt: string | undefined, field: string, what: string, fallback?: string) => {
    const a = (alt ?? '').trim();
    if (FILENAME_RE.test(a))
      add({
        rule: 'img-alt-filename',
        severity: 'error',
        sectionId: s.id,
        message: `The ${what} description looks like a file name (“${a}”). Describe what the picture shows instead.`,
        fix: fallback !== undefined ? { label: fallback ? 'Use the theme’s description' : 'Mark as decorative', patch: { [field]: fallback } as Partial<Section> } : undefined,
      });
    if (a.length > 120)
      add({ rule: 'img-alt-length', severity: 'error', sectionId: s.id, message: `The ${what} description is ${a.length} characters. Canvas wants 120 or fewer — say what matters in one short sentence.` });
    if (REDUNDANT_ALT.test(a))
      add({
        rule: 'img-alt-redundant',
        severity: 'suggestion',
        sectionId: s.id,
        message: 'No need to start with “Image of…” — screen readers already say it is an image.',
        fix: { label: 'Remove “Image of”', patch: { [field]: a.replace(REDUNDANT_ALT, '').replace(/^./, c => c.toLocaleUpperCase()) } as Partial<Section> },
      });
  };

  const checkRich = (s: Section, body: string | undefined, field = 'body') => {
    const html = body ?? '';
    const clean = sanitizeRich(html);
    const norm = (x: string) => x.replace(/\s+/g, ' ').replace(/<br\s*\/?>/g, '<br>').replace(/\s*\/>/g, '>').trim();
    if (html.trim() && norm(html) !== norm(clean))
      add({
        rule: 'rich-stripped',
        severity: 'suggestion',
        sectionId: s.id,
        message: 'Some formatting in this text (pasted styles, images or tables) will be removed. Only paragraphs, bold, italic, lists and links are kept.',
        fix: { label: 'Clean up the text', patch: { [field]: clean } as Partial<Section> },
      });
    const tree = parseHtml(clean);
    const paras = tree.filter((n): n is HElement => n.type === 'el' && n.name === 'p');
    const listy = paras.filter(p => LIST_LIKE.test(textOf([p])));
    if (listy.length)
      add({
        rule: 'list-structure',
        severity: 'warning',
        sectionId: s.id,
        message: 'Some lines start like list items (“1.”, “-”, “*”) but are plain paragraphs. Use a real list so screen readers announce it.',
        fix: { label: 'Turn them into a list', patch: { [field]: fixLists(tree) } as Partial<Section> },
      });
    walk(tree, el => {
      if (el.name !== 'a') return;
      const label = textOf(el.children);
      const href = attr(el, 'href') ?? '';
      if (href.includes('{course}')) courseTokenSeen = true;
      noteLink(label, decodeEntities(href));
      if (VAGUE.has(label.toLocaleLowerCase().replace(/[.!…]+$/, '')))
        add({ rule: 'link-text-vague', severity: 'warning', sectionId: s.id, message: `The link text “${label}” doesn’t say where it goes. Use words like “Week 2 reading” instead.` });
    });
  };

  const checkLink = (s: Section, l: QuickLink, idx: number, all: QuickLink[]) => {
    const label = (l.label ?? '').trim();
    if (!label) {
      add({ rule: 'link-text-empty', severity: 'error', sectionId: s.id, message: `Quick link ${idx + 1} has no label, so it would be announced as an empty link.` });
      return;
    }
    if (VAGUE.has(label.toLocaleLowerCase().replace(/[.!…]+$/, '')))
      add({ rule: 'link-text-vague', severity: 'warning', sectionId: s.id, message: `“${label}” doesn’t say where the link goes. Name the destination, e.g. “Syllabus”.` });
    if (!l.href?.trim() || safeHref(l.href) === null)
      add({
        rule: 'link-href',
        severity: 'error',
        sectionId: s.id,
        message: `“${label}” has ${l.href?.trim() ? 'a web address that isn’t allowed (only https, mailto and course links work)' : 'no destination yet'}.`,
        fix: { label: 'Remove this link', patch: { links: all.filter(x => x !== l) } as Partial<Section> },
      });
    else if (/^http:/i.test(l.href.trim()))
      add({ rule: 'link-http', severity: 'suggestion', sectionId: s.id, message: `“${label}” uses http://. It will be switched to the secure https:// version.` });
    if (l.href?.includes('{course}')) courseTokenSeen = true;
    noteLink(label, l.href ?? '');
    if (label.length > 40)
      add({ rule: 'link-text-long', severity: 'suggestion', sectionId: s.id, message: `“${label}” is long for a button. Aim for 1–4 words and put details in the description.` });
    const longWord = label.split(/\s+/).find(w => w.length > 16);
    if (longWord)
      add({ rule: 'phone-long-word', severity: 'warning', sectionId: s.id, message: `The word “${longWord}” may not fit on a phone screen and could be cut off. Try a shorter word or add a hyphen.` });
  };

  for (const s of doc.sections) {
    switch (s.type) {
      case 'title':
        checkHeading(s, s.text, 'text', true);
        break;
      case 'banner': {
        const themeImg = theme.images?.find(i => i.id === s.imageId);
        const src = s.customSrc?.trim();
        if (!src && !themeImg)
          add({ rule: 'banner-missing', severity: 'warning', sectionId: s.id, message: 'This banner has no picture yet, so nothing will show. Pick a theme image or upload your own.' });
        if (src && /^data:/i.test(src))
          add({ rule: 'img-data-uri', severity: 'error', sectionId: s.id, message: 'Pasted (embedded) images are removed by Canvas. Upload the picture instead.' });
        else if (src && /^http:/i.test(src))
          add({ rule: 'img-https', severity: 'error', sectionId: s.id, message: 'This picture uses an insecure http:// address, which Canvas pages block.', fix: { label: 'Use https://', patch: { customSrc: src.replace(/^http:/i, 'https:') } } });
        else if (src && !/^https:\/\//i.test(src) && !/^\/(?!\/)/.test(src))
          add({ rule: 'img-src', severity: 'error', sectionId: s.id, message: 'This picture’s address isn’t a secure web link (https://), so it won’t show in Canvas.' });
        checkAlt(s, s.alt, 'alt', 'banner', themeImg?.alt ?? '');
        if (!s.alt?.trim() && (src || themeImg))
          add({
            rule: 'img-decorative',
            severity: 'suggestion',
            sectionId: s.id,
            message: 'The banner is marked as decorative, so screen readers skip it. That’s right for mood pictures; if it shows something important, describe it.',
            fix: themeImg?.alt ? { label: 'Use the theme’s description', patch: { alt: themeImg.alt } } : undefined,
          });
        break;
      }
      case 'welcome':
        checkHeading(s, s.heading, 'heading', true);
        checkRich(s, s.body);
        if (!sanitizeRich(s.body ?? '').trim())
          add({ rule: 'empty-text', severity: 'warning', sectionId: s.id, message: 'The welcome message is empty.' });
        break;
      case 'text':
        checkHeading(s, s.heading, 'heading', false);
        checkRich(s, s.body);
        break;
      case 'callout':
        checkHeading(s, s.heading, 'heading', true);
        checkRich(s, s.body);
        break;
      case 'instructor':
        checkHeading(s, s.heading, 'heading', true);
        checkRich(s, s.bio, 'bio');
        if (s.photoSrc?.trim()) {
          if (!s.photoAlt?.trim())
            add({
              rule: 'img-alt',
              severity: 'error',
              sectionId: s.id,
              message: 'Your photo has no description. Screen-reader users will hear nothing (or the file name).',
              fix: { label: `Use “${photoOf(doc.locale, s.name)}”`, patch: { photoAlt: photoOf(doc.locale, s.name) } },
            });
          checkAlt(s, s.photoAlt, 'photoAlt', 'photo', photoOf(doc.locale, s.name));
          if (/^http:/i.test(s.photoSrc.trim()))
            add({ rule: 'img-https', severity: 'error', sectionId: s.id, message: 'Your photo uses an insecure http:// address, which Canvas pages block.', fix: { label: 'Use https://', patch: { photoSrc: s.photoSrc.trim().replace(/^http:/i, 'https:') } } });
        }
        if (s.email?.trim() && !/^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(s.email.trim()))
          add({ rule: 'email-invalid', severity: 'warning', sectionId: s.id, message: `“${s.email}” doesn’t look like an email address, so the link won’t work.` });
        else if (/@example\.(edu|com|org)$/i.test(s.email?.trim() ?? ''))
          add({ rule: 'placeholder', severity: 'suggestion', sectionId: s.id, message: 'Replace the sample email address with yours.' });
        break;
      case 'quick-links': {
        checkHeading(s, s.heading, 'heading', true);
        s.links.forEach((l, i) => checkLink(s, l, i, s.links));
        const longLabels = s.links.filter(l => (l.label ?? '').length > 22);
        if (s.links.length > 4 && longLabels.length)
          add({ rule: 'phone-crowded', severity: 'suggestion', sectionId: s.id, message: `On a phone these ${s.links.length} buttons stack into a long column, and long labels wrap onto several lines. Shorten the labels or move some links elsewhere.` });
        if ((doc.layout === 'playful' || theme.audiences?.includes('early-elementary')) && s.links.length > 6)
          add({ rule: 'young-too-many', severity: 'suggestion', sectionId: s.id, message: 'Young learners do best with 6 or fewer big picture buttons on a page.' });
        if (!s.links.length) add({ rule: 'empty-links', severity: 'warning', sectionId: s.id, message: 'This quick-links section has no links yet.' });
        break;
      }
      case 'objectives': {
        checkHeading(s, s.heading, 'heading', true);
        const empty = s.items.filter(x => !x?.trim()).length;
        if (empty) add({ rule: 'empty-items', severity: 'warning', sectionId: s.id, message: `${empty} item(s) in this list are empty.`, fix: { label: 'Remove empty items', patch: { items: s.items.filter(x => x?.trim()) } } });
        s.items.forEach(x => isAllCaps(x ?? '') && add({ rule: 'all-caps', severity: 'warning', sectionId: s.id, message: 'Long ALL-CAPS text is harder to read, and some screen readers spell it out.', fix: { label: 'Change to sentence case', patch: { items: s.items.map(y => (isAllCaps(y ?? '') ? sentenceCase(y) : y)) } } }));
        break;
      }
      case 'schedule':
        checkHeading(s, s.heading, 'heading', true);
        s.items.forEach(it => {
          if (it.href) {
            if (safeHref(it.href) === null) add({ rule: 'link-href', severity: 'error', sectionId: s.id, message: `“${it.title}” links to an address that isn’t allowed.` });
            if (it.href.includes('{course}')) courseTokenSeen = true;
            noteLink(it.title, it.href);
          }
        });
        break;
      case 'this-week':
        checkHeading(s, s.heading, 'heading', true);
        checkHeading(s, s.title, 'title', true);
        if (!s.items.some(i => i.label?.trim())) add({ rule: 'empty-items', severity: 'warning', sectionId: s.id, message: 'Add at least one thing to do this week.' });
        s.items.forEach(it => {
          if (it.href) {
            if (safeHref(it.href) === null) add({ rule: 'link-href', severity: 'error', sectionId: s.id, message: `“${it.label}” links to an address that isn’t allowed.` });
            if (it.href.includes('{course}')) courseTokenSeen = true;
            noteLink(it.label, it.href);
          }
        });
        break;
      case 'divider':
        break;
    }
  }

  for (const [label, hrefs] of linkTexts)
    if (hrefs.size > 1)
      add({ rule: 'link-duplicate-text', severity: 'warning', message: `Two links both say “${label}” but go to different places. Give each a distinct label so students know which is which.` });

  if (courseTokenSeen && !opts.courseId)
    add({ rule: 'course-token', severity: 'warning', message: 'Some links point to “this course” but the course isn’t known yet. Insert the page from inside Canvas (or set the course) so the links work.' });

  /* ------------------------------------------------------------ 2. Canvas checker rules on the rendered HTML */
  const html = renderPage(doc, theme, { assetBase: opts.assetBase ?? 'https://assets.invalid', courseId: opts.courseId ?? '0' });
  scanRendered(parseHtml(html), add);
  return issues;
}

const photoOf = (locale: string, name: string) => {
  const n = (name ?? '').replace(/[[\]]/g, '').trim();
  return locale === 'es' ? `Foto de ${n}` : locale === 'pt' ? `Foto de ${n}` : `Photo of ${n}`;
};

/** Turn runs of list-like paragraphs into <ul>/<ol>. */
function fixLists(tree: HNode[]): string {
  const ser = (n: HNode): string => (n.type === 'text' ? n.raw : n.type === 'el' ? (n.name === 'br' ? '<br>' : `<${n.name}${n.name === 'a' ? ` href="${attr(n, 'href') ?? ''}"` : ''}>${n.children.map(ser).join('')}</${n.name}>`) : '');
  let out = '';
  let run: { ordered: boolean; items: string[] } | null = null;
  const flush = () => {
    if (run) out += `<${run.ordered ? 'ol' : 'ul'}>${run.items.map(i => `<li>${i}</li>`).join('')}</${run.ordered ? 'ol' : 'ul'}>`;
    run = null;
  };
  for (const n of tree) {
    if (n.type === 'el' && n.name === 'p' && LIST_LIKE.test(textOf([n]))) {
      const inner = n.children.map(ser).join('').replace(/^\s*(\d{1,3}[.)]|[a-zA-Z][.)]|[-*•–])\s+/, '');
      const ordered = /^\s*(\d{1,3}|[a-zA-Z])[.)]/.test(textOf([n]));
      if (run && run.ordered !== ordered) flush();
      run ??= { ordered, items: [] };
      run.items.push(inner);
    } else {
      flush();
      out += ser(n);
    }
  }
  flush();
  return out;
}

/* ---------------------------------------------------------------- rendered-HTML scan */

interface Inh {
  fg: string;
  bg: string;
  px: number;
  bold: boolean;
  bgImage: boolean;
  hidden: boolean;
  section?: string;
}

function fontPx(value: string, parent: number): number | null {
  const v = value.trim();
  const clampMin = /^clamp\(\s*([\d.]+)(rem|em|px)/.exec(v);
  const m = clampMin ?? /^([\d.]+)(rem|em|px)$/.exec(v);
  if (!m) return null;
  const n = parseFloat(m[1]);
  return m[2] === 'px' ? n : m[2] === 'rem' ? n * 16 : n * parent;
}

const HEADING_PX: Record<string, number> = { h2: 1.5 * 16, h3: 1.17 * 16, h4: 16, h5: 0.83 * 16, h6: 0.67 * 16 };

function scanRendered(nodes: HNode[], add: (i: Omit<A11yIssue, 'id'> & { rule: string }) => void): void {
  const contrastSeen = new Set<string>();
  let lastLevel = 1;

  const visit = (list: HNode[], inh: Inh) => {
    for (const n of list) {
      if (n.type === 'comment') continue;
      if (n.type === 'text') {
        const txt = decodeEntities(n.raw).replace(/ /g, ' ').trim();
        if (!txt) continue;
        if (inh.hidden || inh.bgImage) continue;
        const ratio = contrastRatio(inh.fg, inh.bg);
        const large = inh.px >= 24 || (inh.bold && inh.px >= 18.66);
        const need = large ? 3 : 4.5;
        if (ratio + 1e-9 < need) {
          const key = `${inh.section}|${inh.fg}|${inh.bg}|${large}`;
          if (!contrastSeen.has(key)) {
            contrastSeen.add(key);
            add({
              rule: large ? 'large-text-contrast' : 'small-text-contrast',
              severity: 'error',
              sectionId: inh.section,
              message: `Some text here is hard to read: ${inh.fg} on ${inh.bg} has a contrast of ${ratio.toFixed(2)}:1, and ${large ? 'large' : 'normal-size'} text needs ${need}:1. Try another colourway.`,
            });
          }
        }
        continue;
      }
      const el = n;
      const next: Inh = { ...inh };
      const sec = attr(el, 'data-rmt-section');
      if (sec) next.section = sec;
      for (const [k, v] of parseStyle(attr(el, 'style'))) {
        if (k === 'color') next.fg = normHex(v) ?? next.fg;
        else if (k === 'background' || k === 'background-color') {
          if (/url\(|gradient\(/i.test(v)) next.bgImage = true;
          const hex = /#[0-9a-fA-F]{3,6}\b/.exec(v);
          if (hex) next.bg = normHex(hex[0]) ?? next.bg;
        } else if (k === 'background-image' && v !== 'none') next.bgImage = true;
        else if (k === 'font-size') next.px = fontPx(v, inh.px) ?? next.px;
        else if (k === 'clip' && /rect\(\s*0/.test(v)) next.hidden = true;
        else if (k === 'display' && v === 'none') next.hidden = true;
      }
      if (/^h[1-6]$/.test(el.name)) {
        next.bold = true;
        if (!parseStyle(attr(el, 'style')).some(([k]) => k === 'font-size')) next.px = HEADING_PX[el.name] ?? next.px;
        const level = Number(el.name[1]);
        const text = textOf(el.children);
        if (level === 1) add({ rule: 'headings-start-at-h2', severity: 'error', sectionId: next.section, message: 'Canvas already uses the biggest heading (H1) for the page title. Start at H2.' });
        else if (level > lastLevel + 1) add({ rule: 'headings-sequence', severity: 'error', sectionId: next.section, message: `A level-${level} heading follows a level-${lastLevel} heading, skipping a level.` });
        if (!text) add({ rule: 'empty-heading', severity: 'error', sectionId: next.section, message: 'There is an empty heading on the page.' });
        else if (text.length > 120) add({ rule: 'paragraphs-for-headings', severity: 'error', sectionId: next.section, message: 'A heading is over 120 characters long.' });
        lastLevel = level;
      }
      if (el.name === 'strong' || el.name === 'b') next.bold = true;
      if (el.name === 'img') {
        const alt = attr(el, 'alt');
        if (alt === undefined) add({ rule: 'img-alt', severity: 'error', sectionId: next.section, message: 'A picture has no description (alt text).' });
        else {
          if (FILENAME_RE.test(alt.trim())) add({ rule: 'img-alt-filename', severity: 'error', sectionId: next.section, message: 'A picture’s description is a file name.' });
          if (alt.length > 120) add({ rule: 'img-alt-length', severity: 'error', sectionId: next.section, message: 'A picture’s description is over 120 characters.' });
        }
        continue;
      }
      if (el.name === 'a') {
        const href = attr(el, 'href') ?? '';
        const idx = list.indexOf(el);
        let j = idx - 1;
        while (j >= 0 && list[j].type === 'text' && !decodeEntities((list[j] as { raw: string }).raw).trim()) j--;
        const prev = list[j];
        if (href && prev && prev.type === 'el' && prev.name === 'a' && attr(prev, 'href') === href)
          add({ rule: 'adjacent-links', severity: 'warning', sectionId: next.section, message: 'Two links side by side go to the same place. Combine them into one link.' });
        visit(el.children, next);
        continue;
      }
      if (el.name === 'p' && LIST_LIKE.test(textOf(el.children)))
        add({ rule: 'list-structure', severity: 'warning', sectionId: next.section, message: 'A paragraph starts like a list item. Use a real list.' });
      if (el.name === 'table') {
        const caption = el.children.find((c): c is HElement => c.type === 'el' && c.name === 'caption');
        if (!caption || !textOf(caption.children)) add({ rule: 'table-caption', severity: 'error', sectionId: next.section, message: 'A table has no caption.' });
        let ths = 0;
        walk(el.children, c => {
          if (c.name === 'th') {
            ths++;
            if (!['row', 'col', 'rowgroup', 'colgroup'].includes(attr(c, 'scope') ?? '')) add({ rule: 'table-header-scope', severity: 'error', sectionId: next.section, message: 'A table header cell is missing its scope.' });
          }
        });
        if (!ths) add({ rule: 'table-header', severity: 'error', sectionId: next.section, message: 'A table has no header cells.' });
      }
      visit(el.children, next);
    }
  };
  visit(nodes, { fg: '#000000', bg: '#ffffff', px: 16, bold: false, bgImage: false, hidden: false });
}

/**
 * Canvas's RCE checker rules (contrast with inherited colours, heading order, h1, img alt, adjacent links,
 * list structure, tables) on any HTML fragment — for pages RMT Studio didn't make. Assumes Canvas's
 * default black-on-white text at 16px where the HTML sets nothing.
 */
export function checkHtmlAccessibility(html: string): A11yIssue[] {
  const issues: A11yIssue[] = [];
  const seen = new Map<string, number>();
  scanRendered(parseHtml(html), i => {
    const base = `${i.rule}${i.sectionId ? `:${i.sectionId}` : ''}`;
    const n = (seen.get(base) ?? 0) + 1;
    seen.set(base, n);
    const { rule, ...rest } = i;
    void rule;
    issues.push({ id: n > 1 ? `${base}:${n}` : base, ...rest });
  });
  return issues;
}

/** Exposed for tests. */
export const __test = { scanRendered, fixLists, isAllCaps };
export type { RenderOptions };
