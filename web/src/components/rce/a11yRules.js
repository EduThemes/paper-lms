/**
 * Accessibility checker rules for RichContentEditorV2 (G005).
 *
 * The rule set follows the checks Canvas's RCE runs (images, headings,
 * tables, links, lists, colour contrast); the implementation is original and
 * works on the ProseMirror document so every issue can carry a fix that is a
 * normal, undoable editor transaction.
 *
 * checkDocument(editor) -> Issue[]
 *   Issue = {
 *     id, rule, message, why,
 *     pos,                       // document position to select / scroll to
 *     fix?: {
 *       kind: 'text' | 'button' | 'checkbox-text',
 *       label, buttonLabel?, initial?,
 *       apply(editor, value) -> void
 *     }
 *   }
 */

export const IMG_ALT_MAX_LENGTH = 120;
export const HEADING_MAX_LENGTH = 120;

const FILENAME_RE = /^[^\s]+\.(png|jpe?g|gif|svg|webp|bmp|tiff?|heic)$/i;
const LIST_MARKER_RE = /^\s*(?:([-*•●◦‣⁃])|(\d+|[a-zA-Z])[.)])\s+/;

const NAMED_COLORS = {
  black: [0, 0, 0], white: [255, 255, 255], red: [255, 0, 0], green: [0, 128, 0],
  blue: [0, 0, 255], yellow: [255, 255, 0], gray: [128, 128, 128], grey: [128, 128, 128],
  silver: [192, 192, 192], orange: [255, 165, 0], purple: [128, 0, 128], navy: [0, 0, 128],
  maroon: [128, 0, 0], lime: [0, 255, 0], aqua: [0, 255, 255], cyan: [0, 255, 255],
  teal: [0, 128, 128], olive: [128, 128, 0], fuchsia: [255, 0, 255], magenta: [255, 0, 255],
  pink: [255, 192, 203], lightgray: [211, 211, 211], lightgrey: [211, 211, 211],
  darkgray: [169, 169, 169], darkgrey: [169, 169, 169], gold: [255, 215, 0],
};

/** Parse a CSS colour into [r, g, b], or null when it cannot be judged. */
export function parseColor(value) {
  if (!value) return null;
  const v = String(value).trim().toLowerCase();
  if (NAMED_COLORS[v]) return NAMED_COLORS[v];
  let m = /^#([0-9a-f]{3})$/.exec(v);
  if (m) return m[1].split('').map((c) => parseInt(c + c, 16));
  m = /^#([0-9a-f]{6})([0-9a-f]{2})?$/.exec(v);
  if (m) return [0, 2, 4].map((i) => parseInt(m[1].slice(i, i + 2), 16));
  m = /^rgba?\(\s*(\d{1,3})[\s,]+(\d{1,3})[\s,]+(\d{1,3})(?:[\s,/]+([\d.]+%?))?\s*\)$/.exec(v);
  if (m) {
    // A translucent colour depends on what is behind it; don't guess.
    if (m[4] !== undefined && parseFloat(m[4]) < 1 && !m[4].endsWith('%')) return null;
    return [m[1], m[2], m[3]].map((n) => Math.min(255, Number(n)));
  }
  return null;
}

function luminance([r, g, b]) {
  const ch = [r, g, b].map((c) => {
    const s = c / 255;
    return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * ch[0] + 0.7152 * ch[1] + 0.0722 * ch[2];
}

/** WCAG contrast ratio between two [r,g,b] colours. */
export function contrastRatio(a, b) {
  const la = luminance(a);
  const lb = luminance(b);
  return (Math.max(la, lb) + 0.05) / (Math.min(la, lb) + 0.05);
}

/** Font size in px from a CSS value (pt, px, em/rem against 16px). */
function fontSizePx(value) {
  if (!value) return null;
  const m = /^([\d.]+)(px|pt|em|rem)$/.exec(String(value).trim().toLowerCase());
  if (!m) return null;
  const n = Number(m[1]);
  if (m[2] === 'pt') return (n * 4) / 3;
  if (m[2] === 'px') return n;
  return n * 16;
}

function textOf(node) {
  return (node.textContent || '').trim();
}

function srcBasename(src) {
  try {
    const path = new URL(src, 'https://x.invalid/').pathname;
    return decodeURIComponent(path.split('/').filter(Boolean).pop() || '');
  } catch {
    return '';
  }
}

const updateNode = (pos, attrs) => (editor) => {
  editor.chain().command(({ tr, state }) => {
    const node = state.doc.nodeAt(pos);
    if (!node) return false;
    tr.setNodeMarkup(pos, undefined, { ...node.attrs, ...attrs });
    return true;
  }).run();
};

function imageRules(node, pos, issues) {
  if (node.type.name !== 'image') return;
  const { alt, decorative, src } = node.attrs;
  const altText = (alt || '').trim();
  const altFix = (message) => ({
    kind: 'checkbox-text',
    label: 'Alt text',
    checkboxLabel: 'Decorative image (screen readers skip it)',
    initial: altText,
    apply: (editor, { text, checked }) => updateNode(pos, checked
      ? { decorative: true, alt: '' }
      : { decorative: false, alt: (text || '').trim() || null })(editor),
    message,
  });
  if (decorative) return;
  if (!altText) {
    issues.push({
      rule: 'img-alt', pos,
      message: 'Image is missing alt text.',
      why: 'Screen readers read alt text in place of the image. Describe the image, or mark it decorative if it adds nothing.',
      fix: altFix(),
    });
    return;
  }
  if (FILENAME_RE.test(altText) || (src && altText === srcBasename(src))) {
    issues.push({
      rule: 'img-alt-filename', pos,
      message: 'Image alt text looks like a file name.',
      why: 'A file name tells a screen-reader user nothing about the image. Describe what it shows.',
      fix: altFix(),
    });
  }
  if (altText.length > IMG_ALT_MAX_LENGTH) {
    issues.push({
      rule: 'img-alt-length', pos,
      message: `Alt text is longer than ${IMG_ALT_MAX_LENGTH} characters.`,
      why: 'Long alt text is tiring to listen to. Keep it short and put longer descriptions in the page text.',
      fix: altFix(),
    });
  }
}

function headingRules(node, pos, issues, ctx) {
  if (node.type.name !== 'heading') return;
  const { level } = node.attrs;
  if (level === 1) {
    issues.push({
      rule: 'headings-start-at-h2', pos,
      message: 'Heading 1 is reserved for the page title.',
      why: 'The page title is already the level 1 heading; headings in the content should start at level 2.',
      fix: { kind: 'button', buttonLabel: 'Change to Heading 2', apply: updateNode(pos, { level: 2 }) },
    });
  } else if (ctx.prevHeading && level > ctx.prevHeading + 1) {
    const target = ctx.prevHeading + 1;
    issues.push({
      rule: 'headings-sequence', pos,
      message: `Heading level jumps from ${ctx.prevHeading} to ${level}.`,
      why: 'Skipping heading levels makes the outline confusing to screen-reader users navigating by heading.',
      fix: { kind: 'button', buttonLabel: `Change to Heading ${target}`, apply: updateNode(pos, { level: target }) },
    });
  }
  ctx.prevHeading = level;
  if (textOf(node).length > HEADING_MAX_LENGTH) {
    issues.push({
      rule: 'paragraphs-for-headings', pos,
      message: 'Heading is very long.',
      why: 'Headings should be short labels; long text formatted as a heading is better as a paragraph.',
      fix: {
        kind: 'button',
        buttonLabel: 'Change to paragraph',
        apply: (editor) => {
          editor.chain().command(({ tr, state }) => {
            const n = state.doc.nodeAt(pos);
            if (!n) return false;
            tr.setNodeMarkup(pos, state.schema.nodes.paragraph, {});
            return true;
          }).run();
        },
      },
    });
  }
}

function tableRules(node, pos, issues) {
  if (node.type.name !== 'table') return;
  if (!node.attrs.caption) {
    issues.push({
      rule: 'table-caption', pos,
      message: 'Table has no caption.',
      why: 'A caption tells screen-reader users what the table is about before they read it.',
      fix: {
        kind: 'text',
        label: 'Caption',
        initial: '',
        apply: (editor, text) => {
          editor.chain().command(({ tr, state }) => {
            const n = state.doc.nodeAt(pos);
            if (!n || !(text || '').trim()) return false;
            tr.setNodeMarkup(pos, undefined, { ...n.attrs, caption: text.trim() });
            return true;
          }).run();
        },
      },
    });
  }
  let headers = 0;
  let missingScope = 0;
  node.descendants((child) => {
    if (child.type.name === 'tableHeader') {
      headers += 1;
      if (!child.attrs.scope) missingScope += 1;
    }
    return child.type.name !== 'tableHeader' && child.type.name !== 'tableCell';
  });
  if (headers === 0) {
    issues.push({
      rule: 'table-header', pos,
      message: 'Table has no header cells.',
      why: 'Header cells let screen readers announce which row and column a value belongs to.',
      fix: {
        kind: 'button',
        buttonLabel: 'Make the first row a header row',
        apply: (editor) => {
          editor.chain().command(({ tr, state }) => {
            const table = state.doc.nodeAt(pos);
            if (!table || !table.firstChild) return false;
            const header = state.schema.nodes.tableHeader;
            let offset = pos + 2; // inside table, inside first row
            table.firstChild.forEach((cell) => {
              tr.setNodeMarkup(offset, header, { ...cell.attrs, scope: 'col' });
              offset += cell.nodeSize;
            });
            return true;
          }).run();
        },
      },
    });
  } else if (missingScope > 0) {
    issues.push({
      rule: 'table-header-scope', pos,
      message: 'Table header cells have no scope.',
      why: 'The scope says whether a header labels a column or a row, so screen readers can pair it with the right cells.',
      fix: { kind: 'button', buttonLabel: 'Set header scopes', apply: (editor) => editor.commands.fixTableHeaderScopes() },
    });
  }
}

function textblockRules(node, pos, issues) {
  if (!node.isTextblock) return;
  // Adjacent links to the same place, separated only by whitespace.
  let prev = null; // { href, from, to }
  let gapIsBlank = true;
  node.forEach((child, offset) => {
    const from = pos + 1 + offset;
    const to = from + child.nodeSize;
    const link = child.marks.find((m) => m.type.name === 'link');
    if (link) {
      if (prev && gapIsBlank && prev.href === link.attrs.href && prev.to !== from) {
        const start = prev.from;
        issues.push({
          rule: 'adjacent-links', pos: start,
          message: 'Adjacent links go to the same place.',
          why: 'Two links to the same URL next to each other are announced twice. Merge them into one link.',
          fix: {
            kind: 'button',
            buttonLabel: 'Merge links',
            apply: (editor) => editor.chain().setTextSelection({ from: start, to }).setLink({ ...link.attrs }).run(),
          },
        });
      }
      prev = { href: link.attrs.href, from: prev && prev.href === link.attrs.href && prev.to === from ? prev.from : from, to };
      gapIsBlank = true;
    } else if (prev) {
      if (!child.isText || child.text.trim() !== '') gapIsBlank = false;
    }
    // Colour contrast of styled text.
    const style = child.marks.find((m) => m.type.name === 'textStyle');
    if (child.isText && style && style.attrs.color && child.text.trim()) {
      const fg = parseColor(style.attrs.color);
      const bg = parseColor(style.attrs.backgroundColor) || [255, 255, 255];
      if (fg) {
        const ratio = contrastRatio(fg, bg);
        const px = fontSizePx(style.attrs.fontSize) || 16;
        const bold = child.marks.some((m) => m.type.name === 'bold');
        const large = px >= 24 || (bold && px >= 18.66);
        const needed = large ? 3 : 4.5;
        if (ratio < needed) {
          issues.push({
            rule: large ? 'large-text-contrast' : 'small-text-contrast', pos: from,
            message: `Text colour contrast is ${ratio.toFixed(2)}:1; at least ${needed}:1 is needed.`,
            why: 'Low-contrast text is hard to read for people with low vision or on a bright screen.',
            fix: {
              kind: 'button',
              buttonLabel: 'Remove the text colour',
              apply: (editor) => editor.chain().setTextSelection({ from, to }).unsetColor().run(),
            },
          });
        }
      }
    }
  });
}

/** Consecutive paragraphs that start with list markers should be a list. */
function listStructureRules(parent, parentPos, issues) {
  let run = [];
  const flush = () => {
    if (run.length >= 2) {
      const group = run.slice();
      const ordered = group.every((g) => g.ordered);
      issues.push({
        rule: 'list-structure', pos: group[0].pos,
        message: 'These paragraphs look like a list.',
        why: 'Real list markup lets screen readers announce the number of items and move between them.',
        fix: {
          kind: 'button',
          buttonLabel: ordered ? 'Format as a numbered list' : 'Format as a bulleted list',
          apply: (editor) => {
            editor.chain().command(({ tr, state }) => {
              const { schema } = state;
              const items = group.map((g) => {
                const node = state.doc.nodeAt(g.pos);
                const stripped = node.cut(g.markerLength);
                return schema.nodes.listItem.create(null, schema.nodes.paragraph.create(null, stripped.content));
              });
              const list = (ordered ? schema.nodes.orderedList : schema.nodes.bulletList).create(null, items);
              const from = group[0].pos;
              const last = group[group.length - 1];
              const to = last.pos + state.doc.nodeAt(last.pos).nodeSize;
              tr.replaceWith(from, to, list);
              return true;
            }).run();
          },
        },
      });
    }
    run = [];
  };
  parent.forEach((child, offset) => {
    const pos = parentPos + offset;
    if (child.type.name === 'paragraph') {
      const m = LIST_MARKER_RE.exec(child.textContent);
      if (m && child.firstChild && child.firstChild.isText && child.firstChild.text.length >= m[0].length) {
        run.push({ pos, ordered: !!m[2], markerLength: m[0].length });
        return;
      }
    }
    flush();
  });
  flush();
}

/**
 * Run every rule over the editor's document.
 * @param {import('@tiptap/react').Editor} editor
 */
export function checkDocument(editor) {
  if (!editor) return [];
  const { doc } = editor.state;
  const issues = [];
  const ctx = { prevHeading: null };
  listStructureRules(doc, 0, issues);
  doc.descendants((node, pos) => {
    imageRules(node, pos, issues);
    headingRules(node, pos, issues, ctx);
    tableRules(node, pos, issues);
    textblockRules(node, pos, issues);
    if (node.type.name === 'container' || node.type.name === 'blockquote' || node.type.name === 'listItem' || node.type.name === 'details') {
      listStructureRules(node, pos + 1, issues);
    }
    return true;
  });
  issues.sort((a, b) => a.pos - b.pos);
  return issues.map((issue, i) => ({ ...issue, id: `${issue.rule}-${issue.pos}-${i}` }));
}
