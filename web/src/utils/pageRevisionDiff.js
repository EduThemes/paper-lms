// Word-level diff for page revisions (G065). Revisions store HTML; the
// history view compares their readable text, so markup-only changes
// (an added class, re-indented tags) don't drown out the real edits.

// revisionText turns a revision's HTML into readable text, keeping one
// line per block (paragraph, list item, heading) so the diff reads like
// the page. Parsed with DOMParser: inert, nothing runs or loads.
export function revisionText(html) {
  if (!html) return '';
  const marked = String(html)
    .replace(/<\s*br\s*\/?>/gi, '\n')
    .replace(/<\/\s*(p|div|li|h[1-6]|tr|blockquote|pre|ul|ol|table)\s*>/gi, (m) => `${m}\n`);
  let text;
  if (typeof DOMParser === 'undefined') {
    text = marked.replace(/<[^>]+>/g, '');
  } else {
    const body = new DOMParser().parseFromString(`<!DOCTYPE html><body>${marked}</body>`, 'text/html').body;
    text = body.textContent || '';
  }
  return text
    .split('\n')
    .map((line) => line.replace(/[ \t ]+/g, ' ').trim())
    .filter((line, i, all) => line || (i > 0 && all[i - 1]))
    .join('\n')
    .trim();
}

// tokenize splits text into words and the whitespace between them, so
// joining the tokens back reproduces the text exactly.
export function tokenize(text) {
  if (!text) return [];
  return text.match(/\s+|[^\s]+/g) || [];
}

// Above this many token pairs the LCS table gets too big for the browser;
// fall back to "everything removed, everything added".
const MAX_CELLS = 4_000_000;

// diffTokens returns [{type: 'same'|'add'|'del', text}] turning `before`
// into `after`, with neighbouring parts of the same type merged.
export function diffTokens(before, after) {
  const a = before || [];
  const b = after || [];
  const parts = [];
  const push = (type, text) => {
    if (!text) return;
    const last = parts[parts.length - 1];
    if (last && last.type === type) last.text += text;
    else parts.push({ type, text });
  };

  // Trim the common prefix / suffix first: most edits are local.
  let start = 0;
  while (start < a.length && start < b.length && a[start] === b[start]) start += 1;
  let endA = a.length;
  let endB = b.length;
  while (endA > start && endB > start && a[endA - 1] === b[endB - 1]) {
    endA -= 1;
    endB -= 1;
  }

  push('same', a.slice(0, start).join(''));

  const midA = a.slice(start, endA);
  const midB = b.slice(start, endB);
  if (midA.length * midB.length > MAX_CELLS) {
    push('del', midA.join(''));
    push('add', midB.join(''));
  } else {
    // lcs[i][j] = LCS length of midA[i:] and midB[j:].
    const rows = midA.length + 1;
    const cols = midB.length + 1;
    const lcs = new Array(rows);
    for (let i = 0; i < rows; i += 1) lcs[i] = new Uint32Array(cols);
    for (let i = midA.length - 1; i >= 0; i -= 1) {
      for (let j = midB.length - 1; j >= 0; j -= 1) {
        lcs[i][j] = midA[i] === midB[j] ? lcs[i + 1][j + 1] + 1 : Math.max(lcs[i + 1][j], lcs[i][j + 1]);
      }
    }
    let i = 0;
    let j = 0;
    while (i < midA.length && j < midB.length) {
      if (midA[i] === midB[j]) {
        push('same', midA[i]);
        i += 1;
        j += 1;
      } else if (lcs[i + 1][j] >= lcs[i][j + 1]) {
        push('del', midA[i]);
        i += 1;
      } else {
        push('add', midB[j]);
        j += 1;
      }
    }
    push('del', midA.slice(i).join(''));
    push('add', midB.slice(j).join(''));
  }

  push('same', a.slice(endA).join(''));
  return parts;
}

// diffRevisionText diffs the readable text of two HTML bodies.
export function diffRevisionText(beforeHtml, afterHtml) {
  return diffTokens(tokenize(revisionText(beforeHtml)), tokenize(revisionText(afterHtml)));
}

// diffStats counts added / removed words in a diff.
export function diffStats(parts) {
  const count = (type) => parts
    .filter((p) => p.type === type)
    .reduce((n, p) => n + tokenize(p.text).filter((t) => t.trim()).length, 0);
  return { added: count('add'), removed: count('del') };
}
