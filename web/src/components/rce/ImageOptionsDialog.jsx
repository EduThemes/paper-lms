import React, { useEffect, useRef, useState } from 'react';
import { Upload } from 'lucide-react';
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter,
} from '../ui/dialog';
import { Button } from '../ui/button';
import { Input } from '../ui/input';
import { api } from '../../services/api';
import { fetchAllPages } from './ContentPicker';

/**
 * Insert / edit an image (G006): upload from the computer, pick an image
 * already in the course files, or link one by URL; then set alt text, mark
 * it decorative, and choose a display size. Editing an inserted image opens
 * the same dialog on the options step.
 *
 * @param {{
 *   open: boolean,
 *   mode: 'insert'|'edit',
 *   initial?: {src?: string, alt?: string, decorative?: boolean, width?: (string|number)},
 *   courseId?: (string|number),
 *   onClose: () => void,
 *   onSubmit: (attrs: {src: string, alt: string|null, decorative: boolean, width: string|null}) => void,
 * }} props
 */

export const IMAGE_SIZES = [
  { key: 'original', label: 'Original size', width: null },
  { key: 'small', label: 'Small (200px)', width: '200' },
  { key: 'medium', label: 'Medium (320px)', width: '320' },
  { key: 'large', label: 'Large (480px)', width: '480' },
  { key: 'custom', label: 'Custom width', width: null },
];

function sizeKeyFor(width) {
  if (!width) return 'original';
  const found = IMAGE_SIZES.find((s) => s.width && String(s.width) === String(width));
  return found ? found.key : 'custom';
}

const isImageFile = (f) => /^image\//.test(f?.['content-type'] || f?.content_type || f?.contentType || '');

/** URL schemes allowed for an image src (data: is rejected by the server). */
export function isAllowedImageSrc(src) {
  const s = (src || '').trim();
  if (!s) return false;
  if (s.startsWith('/')) return !s.startsWith('//');
  return /^https?:\/\//i.test(s);
}

export default function ImageOptionsDialog({ open, mode = 'insert', initial, courseId, onClose, onSubmit }) {
  const [tab, setTab] = useState(courseId != null ? 'upload' : 'url');
  const [src, setSrc] = useState('');
  const [alt, setAlt] = useState('');
  const [decorative, setDecorative] = useState(false);
  const [sizeKey, setSizeKey] = useState('original');
  const [customWidth, setCustomWidth] = useState('');
  const [uploading, setUploading] = useState(false);
  const [progress, setProgress] = useState(0);
  const [error, setError] = useState(null);
  const [files, setFiles] = useState(null);
  const fileRef = useRef(null);

  useEffect(() => {
    if (!open) return;
    setTab(mode === 'edit' ? 'options' : (courseId != null ? 'upload' : 'url'));
    setSrc(initial?.src || '');
    setAlt(initial?.alt || '');
    setDecorative(!!initial?.decorative);
    setSizeKey(sizeKeyFor(initial?.width));
    setCustomWidth(initial?.width && sizeKeyFor(initial.width) === 'custom' ? String(initial.width) : '');
    setUploading(false);
    setProgress(0);
    setError(null);
  }, [open, mode, initial, courseId]);

  useEffect(() => {
    if (!open || tab !== 'course' || courseId == null || files) return;
    let cancelled = false;
    fetchAllPages((page, per) => api.getCourseFiles(courseId, page, per))
      .then((rows) => { if (!cancelled) setFiles(rows.filter(isImageFile)); })
      .catch((e) => { if (!cancelled) { setFiles([]); setError(e?.message || 'Could not load course files'); } });
    return () => { cancelled = true; };
  }, [open, tab, courseId, files]);

  const handleFile = async (file) => {
    if (!file) return;
    if (!/^image\//.test(file.type)) {
      setError('Choose an image file.');
      return;
    }
    setUploading(true);
    setError(null);
    try {
      const attachment = await api.uploadCourseFileWithProgress(courseId, file, setProgress);
      if (!attachment?.id) throw new Error('Upload failed');
      setSrc(api.getFileDownloadUrl(attachment.id));
      if (!alt) setAlt('');
      setTab('options');
    } catch (e) {
      setError(e?.message || 'Upload failed');
    } finally {
      setUploading(false);
    }
  };

  const width = sizeKey === 'custom'
    ? (/^\d{1,4}$/.test(customWidth.trim()) ? customWidth.trim() : null)
    : (IMAGE_SIZES.find((s) => s.key === sizeKey)?.width || null);

  const canSubmit = isAllowedImageSrc(src) && (decorative || alt.trim()) && !uploading
    && (sizeKey !== 'custom' || width);

  const submit = () => {
    if (!canSubmit) return;
    onSubmit({ src: src.trim(), alt: decorative ? '' : alt.trim(), decorative, width });
  };

  const tabs = [
    courseId != null && { key: 'upload', label: 'Upload' },
    courseId != null && { key: 'course', label: 'Course images' },
    { key: 'url', label: 'URL' },
  ].filter(Boolean);

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onClose(); }}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>{mode === 'edit' ? 'Image options' : 'Insert image'}</DialogTitle>
          <DialogDescription>
            Describe the image for screen-reader users, or mark it decorative if it adds no information.
          </DialogDescription>
        </DialogHeader>

        {mode === 'insert' && tab !== 'options' && (
          <div role="tablist" aria-label="Image source" className="flex gap-1 border-b">
            {tabs.map((t) => (
              <button
                key={t.key}
                type="button"
                role="tab"
                aria-selected={tab === t.key}
                onClick={() => { setTab(t.key); setError(null); }}
                className={`-mb-px border-b-2 px-3 py-1.5 text-sm ${tab === t.key ? 'border-primary font-medium' : 'border-transparent text-muted-foreground'}`}
              >
                {t.label}
              </button>
            ))}
          </div>
        )}

        {tab === 'upload' && (
          <div className="space-y-2">
            <input ref={fileRef} type="file" accept="image/*" className="hidden" aria-label="Image file"
              onChange={(e) => handleFile(e.target.files?.[0])} />
            <button
              type="button"
              disabled={uploading}
              onClick={() => fileRef.current?.click()}
              className="flex w-full flex-col items-center rounded-md border-2 border-dashed p-6 text-sm text-muted-foreground hover:bg-accent"
            >
              <Upload className="mb-2 size-6" aria-hidden="true" />
              {uploading ? `Uploading… ${Math.round(progress * 100)}%` : 'Choose an image from your computer'}
              <span className="mt-1 text-xs">It is saved to the course files. You can also paste or drop images into the editor.</span>
            </button>
          </div>
        )}

        {tab === 'course' && (
          <div className="max-h-64 overflow-y-auto">
            {files === null && <p className="text-sm text-muted-foreground">Loading…</p>}
            {files && files.length === 0 && <p className="text-sm text-muted-foreground">No images in this course yet.</p>}
            {files && files.length > 0 && (
              <ul className="grid grid-cols-3 gap-2">
                {files.map((f) => (
                  <li key={f.id}>
                    <button
                      type="button"
                      onClick={() => { setSrc(api.getFileDownloadUrl(f.id)); setTab('options'); }}
                      className="flex w-full flex-col items-center gap-1 rounded border p-1 text-xs hover:bg-accent"
                    >
                      <img src={api.getFileDownloadUrl(f.id)} alt="" className="h-16 w-full object-contain" loading="lazy" />
                      <span className="w-full truncate">{f.display_name || f.filename || `File ${f.id}`}</span>
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </div>
        )}

        {tab === 'url' && (
          <div className="space-y-1">
            <label htmlFor="rce-image-url" className="text-sm font-medium">Image URL</label>
            <Input id="rce-image-url" type="url" value={src} placeholder="https://example.com/diagram.png"
              onChange={(e) => { setSrc(e.target.value); setError(null); }} />
            {src && !isAllowedImageSrc(src) && <p className="text-xs text-destructive">Use an http(s) address or a course file link.</p>}
          </div>
        )}

        {(tab === 'options' || tab === 'url') && (
          <div className="space-y-3">
            {tab === 'options' && src && (
              <div className="flex justify-center rounded border bg-muted/30 p-2">
                <img src={src} alt="" className="max-h-32 max-w-full" />
              </div>
            )}
            <div className="space-y-1">
              <label htmlFor="rce-image-alt" className="text-sm font-medium">Alt text</label>
              <textarea
                id="rce-image-alt"
                value={alt}
                disabled={decorative}
                maxLength={500}
                rows={2}
                onChange={(e) => setAlt(e.target.value)}
                className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm disabled:opacity-50"
                placeholder="What does the image show?"
                aria-describedby="rce-image-alt-help"
              />
              <p id="rce-image-alt-help" className="text-xs text-muted-foreground">
                {alt.length > 120 ? 'Keep alt text under 120 characters; put long descriptions in the page text.' : 'Required unless the image is decorative.'}
              </p>
            </div>
            <label className="flex items-center gap-2 text-sm">
              <input type="checkbox" checked={decorative} onChange={(e) => setDecorative(e.target.checked)} />
              Decorative image (screen readers skip it)
            </label>
            <div className="flex flex-wrap items-end gap-2">
              <div className="space-y-1">
                <label htmlFor="rce-image-size" className="text-sm font-medium">Size</label>
                <select id="rce-image-size" value={sizeKey} onChange={(e) => setSizeKey(e.target.value)}
                  className="block rounded-md border border-input bg-background px-2 py-1.5 text-sm">
                  {IMAGE_SIZES.map((s) => <option key={s.key} value={s.key}>{s.label}</option>)}
                </select>
              </div>
              {sizeKey === 'custom' && (
                <div className="space-y-1">
                  <label htmlFor="rce-image-width" className="text-sm font-medium">Width (px)</label>
                  <Input id="rce-image-width" inputMode="numeric" className="w-24" value={customWidth}
                    onChange={(e) => setCustomWidth(e.target.value.replace(/[^\d]/g, ''))} />
                </div>
              )}
            </div>
          </div>
        )}

        {error && <p role="alert" className="text-sm text-destructive">{error}</p>}

        <DialogFooter>
          <Button type="button" variant="outline" onClick={onClose}>Cancel</Button>
          {(tab === 'options' || tab === 'url') && (
            <Button type="button" onClick={submit} disabled={!canSubmit}>
              {mode === 'edit' ? 'Save' : 'Insert'}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
