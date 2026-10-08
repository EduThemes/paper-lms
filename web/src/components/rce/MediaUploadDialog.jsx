import React, { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Upload, Mic, Library } from 'lucide-react';
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter,
} from '../ui/dialog';
import { Button } from '../ui/button';
import { rceMediaApi, rceMediaKeys, uploadMedia } from '../../services/api_rce_media';
import MediaRecorderPanel from './MediaRecorderPanel';
import {
  mediaNodeAttrs, isMediaFile, recordingFilename, CAPTION_LANGUAGES, captionLabel,
} from './mediaHelpers';

const TABS = [
  { key: 'upload', label: 'Upload', icon: Upload },
  { key: 'record', label: 'Record', icon: Mic },
  { key: 'library', label: 'My media', icon: Library },
];

function MyMediaList({ onPick }) {
  const query = useQuery({ queryKey: rceMediaKeys.mine(), queryFn: () => rceMediaApi.listMyMedia(1, 50) });
  if (query.isLoading) return <p className="text-sm text-muted-foreground">Loading…</p>;
  if (query.isError) return <p role="alert" className="text-sm text-destructive">Could not load your media.</p>;
  const items = query.data || [];
  if (!items.length) return <p className="text-sm text-muted-foreground">You have not uploaded or recorded any media yet.</p>;
  return (
    <ul className="max-h-64 divide-y overflow-auto rounded-md border" aria-label="My media">
      {items.map((m) => (
        <li key={m.media_id}>
          <button type="button" onClick={() => onPick(m)} className="flex w-full items-center justify-between px-3 py-2 text-left text-sm hover:bg-accent focus:bg-accent focus:outline-none">
            <span className="truncate">{m.title || m.media_id}</span>
            <span className="ml-2 shrink-0 text-xs text-muted-foreground">{m.media_type}</span>
          </button>
        </li>
      ))}
    </ul>
  );
}

/**
 * Upload, record or re-use audio/video for the rich content editor (G002).
 * Inserts a `media` node (extensions/Media.js) that plays the media object.
 *
 * @param {{open: boolean, onClose: () => void, onInsert: (attrs: object) => void, courseId?: string|number}} props
 */
export default function MediaUploadDialog({ open, onClose, onInsert, courseId }) {
  const queryClient = useQueryClient();
  const [tab, setTab] = useState('upload');
  const [file, setFile] = useState(null);
  const [title, setTitle] = useState('');
  const [captionFile, setCaptionFile] = useState(null);
  const [captionLocale, setCaptionLocale] = useState('en');
  const [progress, setProgress] = useState(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(null);

  const reset = () => {
    setFile(null); setTitle(''); setCaptionFile(null); setProgress(null); setBusy(false); setError(null);
  };
  const close = () => { reset(); onClose(); };

  // Insert the media; keep the dialog open only to show a caption problem.
  const finish = (mediaObject, captionError = null) => {
    const attrs = mediaNodeAttrs(mediaObject);
    queryClient.invalidateQueries({ queryKey: rceMediaKeys.mine() });
    if (attrs) onInsert(attrs);
    if (captionError) {
      setBusy(false);
      setProgress(null);
      setFile(null);
      setCaptionFile(null);
      setError(captionError);
      return;
    }
    close();
  };

  const send = async (blob, filename) => {
    setBusy(true);
    setError(null);
    setProgress(0);
    try {
      const created = await uploadMedia(blob, {
        title: title.trim() || undefined, filename, courseId, onProgress: setProgress,
      });
      let captionError = null;
      if (captionFile) {
        try {
          await rceMediaApi.saveTrack(created.media_id, {
            locale: captionLocale, kind: 'subtitles', label: captionLabel(captionLocale), content: await captionFile.text(),
          });
        } catch (e) {
          // The media is saved and inserted; the captions need another try.
          captionError = `Media inserted, but the captions were not saved: ${e?.message || 'invalid caption file'}. Use the Captions button on the player to try again.`;
        }
      }
      finish(created, captionError);
    } catch (e) {
      setBusy(false);
      setProgress(null);
      setError(e?.message || 'Upload failed.');
    }
  };

  const submitUpload = () => {
    if (!file) return;
    if (!isMediaFile(file)) {
      setError('Choose an audio or video file.');
      return;
    }
    send(file, file.name);
  };

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o && !busy) close(); }}>
      <DialogContent className="max-w-xl">
        <DialogHeader>
          <DialogTitle>Add audio or video</DialogTitle>
          <DialogDescription>Upload a file, record from your camera or microphone, or reuse media you added before.</DialogDescription>
        </DialogHeader>

        <div role="tablist" aria-label="Media source" className="flex gap-1 border-b">
          {TABS.map((t) => {
            const Icon = t.icon;
            return (
              <button
                key={t.key}
                type="button"
                role="tab"
                aria-selected={tab === t.key}
                onClick={() => { setTab(t.key); setError(null); }}
                className={`-mb-px inline-flex items-center gap-1.5 border-b-2 px-3 py-1.5 text-sm ${tab === t.key ? 'border-primary text-foreground' : 'border-transparent text-muted-foreground hover:text-foreground'}`}
              >
                <Icon className="size-4" aria-hidden="true" /> {t.label}
              </button>
            );
          })}
        </div>

        {tab !== 'library' && (
          <label className="block text-sm">
            <span className="mb-1 block font-medium">Title</span>
            <input
              type="text"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="e.g. Lab safety walkthrough"
              className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
            />
          </label>
        )}

        {tab === 'upload' && (
          <div className="space-y-3">
            <label className="block text-sm">
              <span className="mb-1 block font-medium">Audio or video file</span>
              <input
                type="file"
                accept="audio/*,video/*"
                onChange={(e) => { setFile(e.target.files?.[0] || null); setError(null); }}
                className="w-full text-sm"
                disabled={busy}
              />
            </label>
            <fieldset className="grid gap-2 rounded-md border p-2 sm:grid-cols-2">
              <legend className="px-1 text-xs text-muted-foreground">Captions (optional)</legend>
              <input
                type="file"
                accept=".vtt,.srt,text/vtt"
                aria-label="Caption file"
                onChange={(e) => setCaptionFile(e.target.files?.[0] || null)}
                className="text-sm"
                disabled={busy}
              />
              <select
                aria-label="Caption language"
                value={captionLocale}
                onChange={(e) => setCaptionLocale(e.target.value)}
                className="rounded-md border border-input bg-background px-2 py-1.5 text-sm"
                disabled={busy}
              >
                {CAPTION_LANGUAGES.map((l) => <option key={l.code} value={l.code}>{l.label}</option>)}
              </select>
            </fieldset>
          </div>
        )}

        {tab === 'record' && (
          <MediaRecorderPanel
            disabled={busy}
            onRecorded={(blob, mimeType, kind) => send(blob, recordingFilename(mimeType, kind))}
          />
        )}

        {tab === 'library' && <MyMediaList onPick={(m) => finish(m)} />}

        {progress != null && (
          <div className="space-y-1" aria-live="polite">
            <div className="h-2 w-full overflow-hidden rounded bg-muted">
              <div className="h-full bg-primary transition-all" style={{ width: `${progress}%` }} />
            </div>
            <p className="text-xs text-muted-foreground">{progress < 100 ? `Uploading… ${progress}%` : 'Processing…'}</p>
          </div>
        )}
        {error && <p role="alert" className="text-sm text-destructive">{error}</p>}

        <DialogFooter>
          <Button type="button" variant="outline" onClick={close} disabled={busy}>Cancel</Button>
          {tab === 'upload' && (
            <Button type="button" onClick={submitUpload} disabled={!file || busy}>
              {busy ? 'Uploading…' : 'Upload and insert'}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
