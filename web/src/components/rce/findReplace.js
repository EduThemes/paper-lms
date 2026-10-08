/**
 * Search and replace over a ProseMirror document (G016).
 *
 * Matches are found per textblock on its text content, so a match may span
 * differently formatted runs ("bold" + "face"). Non-text inline nodes (hard
 * breaks, inline atoms) break a match. Replacing keeps the formatting of the
 * first matched character.
 */

/**
 * @param {import('@tiptap/pm/model').Node} doc
 * @param {string} query
 * @param {{caseSensitive?: boolean}} opts
 * @returns {{from: number, to: number}[]}
 */
export function findMatches(doc, query, { caseSensitive = false } = {}) {
  if (!query) return [];
  const needle = caseSensitive ? query : query.toLowerCase();
  const matches = [];
  doc.descendants((node, pos) => {
    if (!node.isTextblock) return true;
    let text = '';
    const positions = [];
    node.forEach((child, offset) => {
      const start = pos + 1 + offset;
      if (child.isText) {
        for (let i = 0; i < child.text.length; i += 1) positions.push(start + i);
        text += child.text;
      } else {
        positions.push(-1);
        text += '￼';
      }
    });
    const hay = caseSensitive ? text : text.toLowerCase();
    let idx = hay.indexOf(needle);
    while (idx !== -1) {
      const span = positions.slice(idx, idx + needle.length);
      if (!span.includes(-1)) {
        matches.push({ from: span[0], to: span[span.length - 1] + 1 });
      }
      idx = hay.indexOf(needle, idx + Math.max(needle.length, 1));
    }
    return false;
  });
  return matches;
}

/** Replace one match. Returns true when the document changed. */
export function replaceMatch(editor, match, replacement) {
  if (!editor || !match) return false;
  return editor.chain().focus().command(({ tr }) => {
    if (replacement) tr.insertText(replacement, match.from, match.to);
    else tr.delete(match.from, match.to);
    return true;
  }).run();
}

/** Replace every match (last to first so positions stay valid). */
export function replaceAll(editor, query, replacement, opts) {
  if (!editor || !query) return 0;
  const matches = findMatches(editor.state.doc, query, opts);
  if (matches.length === 0) return 0;
  editor.chain().command(({ tr }) => {
    for (let i = matches.length - 1; i >= 0; i -= 1) {
      const m = matches[i];
      if (replacement) tr.insertText(replacement, m.from, m.to);
      else tr.delete(m.from, m.to);
    }
    return true;
  }).run();
  return matches.length;
}

/** Words in plain text (Canvas counts whitespace-separated words). */
export function countWords(text) {
  const t = (text || '').trim();
  return t ? t.split(/\s+/).length : 0;
}
