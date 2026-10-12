import React, { useEffect, useState } from 'react';
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter,
} from '../ui/dialog';
import { Button } from '../ui/button';
import { Input } from '../ui/input';

/**
 * Link options (G014): URL, link text (when nothing is selected), and
 * whether the link opens in a new tab. Supports http(s), mailto:, tel: and
 * course-relative links.
 */

/** Normalise what the user typed into an href, or null if not allowed. */
export function normalizeHref(raw) {
  const v = (raw || '').trim();
  if (!v) return null;
  if (/^(https?:|mailto:|tel:)/i.test(v)) {
    if (/^tel:/i.test(v)) return /^tel:\+?[\d\s().-]{3,}$/i.test(v) ? v.replace(/\s+/g, '') : null;
    return v;
  }
  if (v.startsWith('//')) return `https:${v}`;
  if (v.startsWith('/')) return v;
  if (v.startsWith('#')) return v;
  if (/^[\w.+-]+@[\w-]+\.[\w.-]+$/.test(v)) return `mailto:${v}`;
  if (/^\+?[\d\s().-]{7,}$/.test(v)) return `tel:${v.replace(/[\s().-]/g, '')}`;
  if (/^[a-z][a-z0-9+.-]*:/i.test(v)) return null; // javascript:, data:, ...
  return `https://${v}`;
}

/**
 * @param {{
 *   open: boolean,
 *   initial?: {href?: string, newTab?: boolean, text?: string},
 *   needsText: boolean,
 *   isEditing: boolean,
 *   onClose: () => void,
 *   onSubmit: (v: {href: string, newTab: boolean, text: string}) => void,
 *   onRemove: () => void,
 * }} props
 */
export default function LinkDialog({ open, initial, needsText, isEditing, onClose, onSubmit, onRemove }) {
  const [url, setUrl] = useState('');
  const [text, setText] = useState('');
  const [newTab, setNewTab] = useState(false);
  const [error, setError] = useState(null);

  useEffect(() => {
    if (!open) return;
    setUrl(initial?.href || '');
    setText(initial?.text || '');
    setNewTab(!!initial?.newTab);
    setError(null);
  }, [open, initial]);

  const submit = () => {
    const href = normalizeHref(url);
    if (!href) {
      setError('Enter a web address, email address, phone number or course link.');
      return;
    }
    onSubmit({ href, newTab, text: text.trim() });
  };

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onClose(); }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{isEditing ? 'Edit link' : 'Insert link'}</DialogTitle>
          <DialogDescription>Links can go to a web page, an email address (mailto:) or a phone number (tel:).</DialogDescription>
        </DialogHeader>
        <form className="space-y-3" onSubmit={(e) => { e.preventDefault(); submit(); }}>
          {needsText && (
            <div className="space-y-1">
              <label htmlFor="rce-link-text" className="text-sm font-medium">Text</label>
              <Input id="rce-link-text" value={text} onChange={(e) => setText(e.target.value)} placeholder="Link text" />
            </div>
          )}
          <div className="space-y-1">
            <label htmlFor="rce-link-url" className="text-sm font-medium">Link</label>
            <Input id="rce-link-url" autoFocus value={url} placeholder="https://example.com"
              onChange={(e) => { setUrl(e.target.value); setError(null); }} />
          </div>
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" checked={newTab} onChange={(e) => setNewTab(e.target.checked)} />
            Open in a new tab
          </label>
          {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
          <DialogFooter>
            {isEditing && <Button type="button" variant="outline" onClick={onRemove}>Remove link</Button>}
            <Button type="button" variant="outline" onClick={onClose}>Cancel</Button>
            <Button type="submit" disabled={!url.trim()}>{isEditing ? 'Save' : 'Insert'}</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
