// @mention helpers for the discussion composer. The server notifies a
// picked user only when "@<their name>" is still in the post, so the
// composer inserts that text and sends the ids that survive editing.

import { htmlToText } from './html';

/** Escape text for insertion into HTML. */
function escapeText(text) {
  return String(text)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}

/**
 * Append "@Name" to the end of the editor's HTML: inside the last
 * paragraph when there is one, otherwise as a new paragraph.
 */
export function appendMention(html, name) {
  const tag = `@${escapeText(name)}&nbsp;`;
  const current = html || '';
  const trimmed = current.trimEnd();
  if (trimmed.endsWith('</p>')) {
    const idx = trimmed.lastIndexOf('</p>');
    const before = trimmed.slice(0, idx);
    const sep = before.endsWith('>') || before.endsWith(' ') || before.endsWith('&nbsp;') ? '' : ' ';
    return `${before}${sep}${tag}</p>`;
  }
  if (!trimmed) return `<p>${tag}</p>`;
  return `${trimmed}<p>${tag}</p>`;
}

/**
 * The picked users still named in the message ("@Name", case-insensitive).
 * `picked` is `[{ id, name }]`; returns their ids.
 */
export function mentionedUserIds(html, picked) {
  if (!picked || picked.length === 0) return [];
  const text = htmlToText(html || '').toLowerCase();
  const ids = [];
  for (const user of picked) {
    if (!user?.name) continue;
    if (text.includes(`@${user.name.toLowerCase()}`) && !ids.includes(user.id)) ids.push(user.id);
  }
  return ids;
}

/** True when the editor holds no visible text. */
export function isBlankMessage(html) {
  if (!html) return true;
  if (/<(img|video|audio|iframe)\b/i.test(html)) return false;
  return htmlToText(html).replace(/\u00a0/g, ' ').trim().length === 0;
}

/** Human-readable file size. */
export function formatFileSize(bytes) {
  const n = Number(bytes) || 0;
  if (n >= 1048576) return `${(n / 1048576).toFixed(1)} MB`;
  if (n >= 1024) return `${Math.round(n / 1024)} KB`;
  return `${n} B`;
}
