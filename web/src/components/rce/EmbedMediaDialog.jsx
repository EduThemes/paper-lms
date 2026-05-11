import React, { useState } from 'react';
import {
  Dialog, DialogContent, DialogHeader, DialogTitle,
  DialogDescription, DialogFooter,
} from '../ui/dialog';
import { Button } from '../ui/button';
import { Input } from '../ui/input';

/**
 * @typedef {Object} EmbedMediaDialogProps
 * @property {boolean} open
 * @property {() => void} onClose
 * @property {(iframeHTML: string) => void} onInsert
 */

const ALLOWED_HOSTS = new Set([
  'youtube.com', 'www.youtube.com', 'youtu.be',
  'vimeo.com', 'player.vimeo.com',
]);

/**
 * Convert a YouTube/Vimeo URL into a safe embed src.
 * Returns null when the host is not allow-listed or the URL is malformed.
 */
function toEmbedSrc(raw) {
  let u;
  try { u = new URL(raw.trim()); } catch { return null; }
  if (u.protocol !== 'https:' && u.protocol !== 'http:') return null;
  const host = u.hostname.toLowerCase();
  if (!ALLOWED_HOSTS.has(host)) return null;

  if (host === 'youtu.be') {
    const id = u.pathname.replace(/^\//, '');
    if (!/^[\w-]{6,}$/.test(id)) return null;
    return `https://www.youtube.com/embed/${id}`;
  }
  if (host.endsWith('youtube.com')) {
    if (u.pathname === '/watch') {
      const id = u.searchParams.get('v');
      if (!id || !/^[\w-]{6,}$/.test(id)) return null;
      return `https://www.youtube.com/embed/${id}`;
    }
    if (u.pathname.startsWith('/embed/')) return `https://www.youtube.com${u.pathname}`;
  }
  if (host === 'vimeo.com') {
    const id = u.pathname.split('/').filter(Boolean)[0];
    if (!/^\d+$/.test(id || '')) return null;
    return `https://player.vimeo.com/video/${id}`;
  }
  if (host === 'player.vimeo.com') return u.toString();
  return null;
}

/**
 * Paste-a-URL embed dialog. Builds a sanitized iframe and hands it back.
 * @param {EmbedMediaDialogProps} props
 */
export default function EmbedMediaDialog({ open, onClose, onInsert }) {
  const [url, setUrl] = useState('');
  const [error, setError] = useState(null);

  const submit = () => {
    const src = toEmbedSrc(url);
    if (!src) {
      setError('URL must be a YouTube or Vimeo link.');
      return;
    }
    const html =
      `<div class="rce-embed" style="position:relative;padding-bottom:56.25%;height:0;overflow:hidden;">` +
      `<iframe src="${src}" title="Embedded media" frameborder="0" ` +
      `allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" ` +
      `allowfullscreen style="position:absolute;top:0;left:0;width:100%;height:100%;"></iframe></div>`;
    onInsert(html);
    setUrl('');
    setError(null);
  };

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onClose(); }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Embed media</DialogTitle>
          <DialogDescription>
            Paste a YouTube or Vimeo URL. We&rsquo;ll embed it as a responsive iframe.
          </DialogDescription>
        </DialogHeader>

        <Input
          autoFocus
          type="url"
          value={url}
          onChange={(e) => { setUrl(e.target.value); setError(null); }}
          placeholder="https://www.youtube.com/watch?v=…"
          aria-label="Media URL"
          onKeyDown={(e) => { if (e.key === 'Enter') submit(); }}
        />
        {error && <p className="text-xs text-destructive">{error}</p>}

        <DialogFooter>
          <Button variant="outline" type="button" onClick={onClose}>Cancel</Button>
          <Button type="button" onClick={submit} disabled={!url.trim()}>Embed</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
