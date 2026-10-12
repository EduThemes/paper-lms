import React, { useState } from 'react';
import {
  Dialog, DialogContent, DialogHeader, DialogTitle,
  DialogDescription, DialogFooter,
} from '../ui/dialog';
import { Button } from '../ui/button';
import { toEmbedAttrs } from '../../utils/iframePolicy';

/**
 * @typedef {Object} EmbedMediaDialogProps
 * @property {boolean} open
 * @property {() => void} onClose
 * @property {(attrs: {src: string, title: string, width?: string, height?: string, allow?: string}) => void} onInsert
 */

/**
 * Paste-a-link or paste-embed-code dialog (G001). Accepts share links of the
 * common providers (YouTube, Vimeo, Loom, Google Docs/Slides/Forms/Drive),
 * any URL on the education embed allowlist (Panopto, Kaltura, Desmos, PhET,
 * GeoGebra, Padlet, ...), or a whole <iframe> embed code.
 * @param {EmbedMediaDialogProps} props
 */
export default function EmbedMediaDialog({ open, onClose, onInsert }) {
  const [input, setInput] = useState('');
  const [title, setTitle] = useState('');
  const [error, setError] = useState(null);

  const submit = () => {
    const attrs = toEmbedAttrs(input);
    if (!attrs) {
      setError('That site is not on the embed allowlist. Paste a link or embed code from YouTube, Vimeo, Google Docs/Slides/Forms, Panopto, Kaltura, Desmos, PhET, GeoGebra, Padlet, Quizlet or another supported tool.');
      return;
    }
    // Hand back node attributes; the editor inserts an `iframe` schema node
    // (extensions/Iframe.js), which serializes to a responsive .rce-embed.
    onInsert({ ...attrs, title: title.trim() || attrs.title || 'Embedded content' });
    setInput('');
    setTitle('');
    setError(null);
  };

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onClose(); }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Embed content</DialogTitle>
          <DialogDescription>
            Paste a link or the embed code from a video, document or classroom tool.
          </DialogDescription>
        </DialogHeader>

        <label className="text-sm font-medium" htmlFor="rce-embed-input">Link or embed code</label>
        <textarea
          id="rce-embed-input"
          autoFocus
          rows={3}
          value={input}
          onChange={(e) => { setInput(e.target.value); setError(null); }}
          placeholder="https://www.youtube.com/watch?v=…  or  <iframe src=…></iframe>"
          className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
          onKeyDown={(e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); submit(); } }}
        />
        <label className="text-sm font-medium" htmlFor="rce-embed-title">Title (for screen readers)</label>
        <input
          id="rce-embed-title"
          type="text"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder="e.g. Photosynthesis explainer video"
          className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
        />
        {error && <p role="alert" className="text-xs text-destructive">{error}</p>}

        <DialogFooter>
          <Button variant="outline" type="button" onClick={onClose}>Cancel</Button>
          <Button type="button" onClick={submit} disabled={!input.trim()}>Embed</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
