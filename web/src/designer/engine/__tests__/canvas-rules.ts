/**
 * Static checkers for the page body.
 *
 *   paperViolations   Paper's rich-content allowlist (docs/page-designer/README.md, "Styling model"): every
 *                     inline property in PAPER_INLINE_CSS, no <style>/<svg>/<button>, positions not fixed/sticky.
 *   canvasViolations  docs/research/canvas-html-constraints.md: what the Canvas server sanitizer and the RCE
 *                     (TinyMCE 5) keep, plus our house rules, on the page as Canvas gets it (vars flattened).
 *                     Properties and attributes in CANVAS_DEGRADES are Paper-only polish that Canvas strips
 *                     without harm (the page still reads and looks right there).
 * Both return human-readable violations (empty = clean).
 */
import { attr, parseHtml, parseStyle, textOf, walk, type HElement } from '../html';
import { flattenDesignVars } from '../render/design';

/**
 * Paper's inline-CSS allowlist (contract layer 1): Canvas's list, plus layout, position with offsets and
 * z-index, backgrounds, font-weight, letter-spacing, text-transform, text-shadow, box-shadow, opacity,
 * transform, transition, filter, aspect-ratio, object-fit and logical properties.
 */
export const PAPER_INLINE_CSS = new Set([
  // Canvas's list (below) is a subset of Paper's
  ...`display flex flex-basis flex-direction flex-flow flex-grow flex-shrink flex-wrap gap row-gap column-gap order align-content align-items align-self justify-content justify-items justify-self place-content place-items place-self grid grid-area grid-auto-columns grid-auto-flow grid-auto-rows grid-column grid-gap grid-row grid-template grid-template-areas grid-template-columns grid-template-rows grid-column-start grid-column-end grid-column-gap grid-row-start grid-row-end grid-row-gap
  width height min-width min-height max-width max-height margin margin-top margin-right margin-bottom margin-left padding padding-top padding-right padding-bottom padding-left
  position top left right bottom inset z-index float clear overflow overflow-x overflow-y clip visibility
  border border-radius border-top border-right border-bottom border-left border-collapse border-color border-spacing border-style border-width border-top-color border-top-style border-top-width border-right-color border-right-style border-right-width border-bottom-color border-bottom-style border-bottom-width border-left-color border-left-style border-left-width border-top-left-radius border-top-right-radius border-bottom-left-radius border-bottom-right-radius
  background background-attachment background-color background-image background-position background-position-x background-position-y background-repeat background-size background-clip background-origin
  color direction font font-family font-size font-style font-variant font-weight line-height letter-spacing word-spacing text-align text-decoration text-indent text-transform text-shadow vertical-align white-space list-style list-style-position list-style-type
  box-shadow opacity transform transform-origin transition transition-property transition-duration transition-timing-function transition-delay filter aspect-ratio object-fit object-position
  margin-inline margin-inline-start margin-inline-end margin-block margin-block-start margin-block-end padding-inline padding-inline-start padding-inline-end padding-block padding-block-start padding-block-end inline-size block-size min-inline-size max-inline-size min-block-size max-block-size inset-inline inset-inline-start inset-inline-end inset-block inset-block-start inset-block-end border-inline border-inline-start border-inline-end border-block border-block-start border-block-end
  cursor table-layout`.split(/\s+/),
]);
const PAPER_FORBIDDEN_ELEMENTS = new Set(['style', 'script', 'svg', 'math', 'button', 'form', 'input', 'iframe', 'link', 'meta']);

export function paperViolations(html: string): string[] {
  const v: string[] = [];
  walk(parseHtml(html), (el: HElement) => {
    const where = `<${el.name}${attr(el, 'data-pd-section') ? ` ${attr(el, 'data-pd-section')}` : ''}>`;
    if (PAPER_FORBIDDEN_ELEMENTS.has(el.name)) v.push(`element not allowed in Paper rich content: ${where}`);
    for (const [k] of el.attrs) if (/^on/i.test(k)) v.push(`event handler ${k} on ${where}`);
    for (const [p, val] of parseStyle(attr(el, 'style'))) {
      if (!PAPER_INLINE_CSS.has(p)) v.push(`inline CSS not in Paper's allowlist: ${p} on ${where}`);
      if (p === 'position' && /fixed|sticky/i.test(val)) v.push(`position:${val} on ${where}`);
      if (/url\(/i.test(val)) v.push(`url() in inline ${p} on ${where}`);
      if (/expression\(|javascript:|[<>{}]|!important/i.test(val)) v.push(`unsafe value in ${p} on ${where}`);
      const vars = val.match(/var\([^)]*\)/g) ?? [];
      for (const x of vars) if (!/^var\(--pd-c\d+, #[0-9a-f]{6}\)$/.test(x)) v.push(`var() without a hex fallback (${x}) on ${where}`);
    }
  });
  return v;
}

/** Paper-only polish Canvas strips harmlessly on export. */
export const CANVAS_DEGRADES = {
  css: new Set(['font-weight', 'aspect-ratio', 'object-fit']),
  attrs: new Set(['srcset', 'loading']),
};

/** Server CSS allow-list (canvas_sanitize.rb L674-750), exact property names. */
export const ALLOWED_CSS = new Set(
  `display flex flex-basis flex-direction flex-flow flex-grow flex-shrink flex-wrap gap row-gap column-gap order align-content align-items align-self justify-content justify-items justify-self place-content place-items place-self grid grid-area grid-auto-columns grid-auto-flow grid-auto-rows grid-column grid-gap grid-row grid-template grid-template-areas grid-template-columns grid-template-rows grid-column-start grid-column-end grid-column-gap grid-row-start grid-row-end grid-row-gap
  width height min-width min-height max-width max-height margin margin-top margin-right margin-bottom margin-left padding padding-top padding-right padding-bottom padding-left
  position top left right z-index float clear overflow overflow-x overflow-y clip visibility zoom
  border border-radius border-top border-right border-bottom border-left border-collapse border-color border-spacing border-style border-width border-top-color border-top-style border-top-width border-right-color border-right-style border-right-width border-bottom-color border-bottom-style border-bottom-width border-left-color border-left-style border-left-width
  background background-attachment background-color background-image background-position background-position-x background-position-y background-repeat
  color direction font font-family font-size font-stretch font-style font-variant font-width line-height text-align text-decoration text-indent vertical-align white-space list-style list-style-image list-style-position list-style-type
  cursor table-layout user-select`.split(/\s+/),
);

/** Elements both filters keep (cheat sheet). */
export const ALLOWED_ELEMENTS = new Set(
  'div span p h2 h3 h4 h5 h6 a img ul ol li strong em b i u small sub sup br hr blockquote figure figcaption details summary section article aside header footer nav table caption thead tbody tfoot tr th td picture source mark time abbr code pre'.split(' '),
);
const GLOBAL_ATTRS = new Set(['id', 'class', 'style', 'title', 'role', 'lang', 'dir']);
const ARIA_OK = new Set(['aria-label', 'aria-labelledby', 'aria-describedby', 'aria-hidden', 'aria-expanded', 'aria-controls', 'aria-live', 'aria-level', 'aria-description']);
const ELEMENT_ATTRS: Record<string, Set<string>> = {
  a: new Set(['href', 'target', 'name']),
  img: new Set(['src', 'alt', 'width', 'height']),
  th: new Set(['scope', 'colspan', 'rowspan']),
  td: new Set(['colspan', 'rowspan']),
  ol: new Set(['start', 'type']),
  ul: new Set(['type']),
};
const INLINE_IN_A = new Set(['img', 'span', 'strong', 'em', 'b', 'i', 'br']);
const BG_PROPS = new Set(['background', 'background-color']);
const HEX6 = /^#[0-9a-f]{6}$/;

export function canvasViolations(page: string): string[] {
  const v: string[] = [];
  const html = flattenDesignVars(page);
  if (/<!--/.test(html)) v.push('HTML comment');
  if (/var\(/.test(html)) v.push('custom property left after flattening');
  const tree = parseHtml(html);
  let lastLevel = 1;
  walk(tree, (el: HElement, anc) => {
    const where = `<${el.name}${attr(el, 'data-pd-section') ? ` ${attr(el, 'data-pd-section')}` : ''}>`;
    if (!ALLOWED_ELEMENTS.has(el.name)) v.push(`element not allowed: ${where}`);
    for (const [k, val] of el.attrs) {
      if (GLOBAL_ATTRS.has(k) || ARIA_OK.has(k) || ELEMENT_ATTRS[el.name]?.has(k) || (el.name === 'img' && CANVAS_DEGRADES.attrs.has(k))) continue;
      if (k.startsWith('data-') && !/^data-(xml|kyle-menu|turn-into-dialog|flash-message|popup-within|html-tooltip-title|method)/.test(k)) continue;
      v.push(`attribute stripped by Canvas: ${k}="${val}" on ${where}`);
    }
    const decls = parseStyle(attr(el, 'style'));
    const props = new Set(decls.map(([p]) => p));
    for (const [p, val] of decls) {
      if (p.startsWith('--')) v.push(`custom property ${p} on ${where}`);
      else if (!ALLOWED_CSS.has(p) && !CANVAS_DEGRADES.css.has(p)) v.push(`CSS stripped by Canvas: ${p} on ${where}`);
      if (/url\(\s*['"]?(data:|http:)/i.test(val)) v.push(`insecure/data url in ${p} on ${where}`);
      if (p === 'font-size' && /\dpx/.test(val)) v.push(`px font-size (${val}) on ${where}`);
      if (['width', 'min-width', 'flex-basis'].includes(p) && Number(/^(\d+)px$/.exec(val)?.[1] ?? 0) > 340) v.push(`fixed width over 340px (${p}:${val}) on ${where}`);
      if (p === 'flex') {
        const basis = /(\d+)px/.exec(val)?.[1];
        if (basis && Number(basis) > 440) v.push(`large flex-basis (${val}) on ${where}`);
      }
      const visuallyHidden = props.has('clip') && val === '1px';
      if (p === 'height' && /\d(px|rem|em)/.test(val) && el.name !== 'hr' && !visuallyHidden) v.push(`fixed height (${val}) on ${where}`);
      if ((p === 'color' || BG_PROPS.has(p)) && /#[0-9a-fA-F]+/.test(val)) {
        for (const hex of val.match(/#[0-9a-fA-F]+/g) ?? []) if (!HEX6.test(hex)) v.push(`colour not 6-digit lower-case hex (${hex}) on ${where}`);
      }
    }
    const hasBg = [...props].some(p => BG_PROPS.has(p));
    if (hasBg && !props.has('color')) v.push(`background without color on ${where}`);
    if (props.has('color') && !hasBg) v.push(`color without background on ${where}`);
    if (el.name === 'a') {
      walk(el.children, c => {
        if (!INLINE_IN_A.has(c.name)) v.push(`<a> wraps block <${c.name}> (TinyMCE drops the link)`);
      });
      if (anc.some(a => a.name === 'a')) v.push('nested <a>');
    }
    if ((el.name === 'div' || el.name === 'span') && !el.children.some(c => c.type === 'el' || (c.type === 'text' && c.raw.trim())))
      v.push(`empty ${where} (TinyMCE removes it)`);
    if (el.name === 'img') {
      if (attr(el, 'alt') === undefined) v.push('img without alt');
      const src = attr(el, 'src') ?? '';
      if (!/^https:\/\//.test(src) && !/^\/(?!\/)/.test(src)) v.push(`img src not https: ${src}`);
    }
    if (/^h[1-6]$/.test(el.name)) {
      const level = Number(el.name[1]);
      if (level === 1) v.push('h1 used');
      if (level > lastLevel + 1) v.push(`heading skips from h${lastLevel} to h${level}`);
      if (!textOf(el.children)) v.push('empty heading');
      lastLevel = level;
    }
  });
  return v;
}
