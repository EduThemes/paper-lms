import React, { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Trash2 } from 'lucide-react';
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter,
} from '../ui/dialog';
import { Button } from '../ui/button';
import { rceMediaApi, rceMediaKeys } from '../../services/api_rce_media';
import { CAPTION_LANGUAGES, captionLabel } from './mediaHelpers';

const KINDS = [
  { value: 'subtitles', label: 'Subtitles' },
  { value: 'captions', label: 'Captions (includes sounds)' },
  { value: 'descriptions', label: 'Audio descriptions' },
  { value: 'chapters', label: 'Chapters' },
];

/**
 * Caption manager for one media object (Canvas's media tracks, G002): lists
 * the caption languages and uploads a WebVTT or SRT file per language. An
 * upload for a language that already has captions replaces them.
 */
export default function MediaCaptionsDialog({ mediaId, title, onClose, onChanged }) {
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: rceMediaKeys.detail(mediaId),
    queryFn: () => rceMediaApi.getMediaObject(mediaId),
    enabled: !!mediaId,
  });
  const [locale, setLocale] = useState('en');
  const [kind, setKind] = useState('subtitles');
  const [file, setFile] = useState(null);
  const [error, setError] = useState(null);

  const afterChange = () => {
    queryClient.invalidateQueries({ queryKey: rceMediaKeys.detail(mediaId) });
    onChanged?.();
  };

  const save = useMutation({
    mutationFn: async () => {
      if (!file) throw new Error('Choose a caption file (.vtt or .srt).');
      if (file.size > 1024 * 1024) throw new Error('Caption files are limited to 1 MB.');
      const content = await file.text();
      return rceMediaApi.saveTrack(mediaId, { locale, kind, label: captionLabel(locale), content });
    },
    onSuccess: () => { setFile(null); setError(null); afterChange(); },
    onError: (e) => setError(e?.message || 'Could not save captions.'),
  });

  const remove = useMutation({
    mutationFn: (trackId) => rceMediaApi.deleteTrack(mediaId, trackId),
    onSuccess: afterChange,
    onError: (e) => setError(e?.message || 'Could not delete captions.'),
  });

  const media = query.data;
  const tracks = media?.media_tracks || [];
  const canEdit = media?.can_edit !== false;

  return (
    <Dialog open onOpenChange={(o) => { if (!o) onClose(); }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Captions</DialogTitle>
          <DialogDescription>
            {title ? `Caption files for “${title}”. ` : ''}Upload a WebVTT (.vtt) or SubRip (.srt) file per language.
          </DialogDescription>
        </DialogHeader>

        {query.isLoading && <p className="text-sm text-muted-foreground">Loading…</p>}
        {query.isError && <p role="alert" className="text-sm text-destructive">Could not load this media.</p>}

        {media && (
          <ul className="divide-y rounded-md border text-sm" aria-label="Caption languages">
            {tracks.length === 0 && <li className="p-2 text-muted-foreground">No captions yet.</li>}
            {tracks.map((t) => (
              <li key={t.id} className="flex items-center justify-between gap-2 p-2">
                <span>
                  {t.label || captionLabel(t.locale)} <span className="text-muted-foreground">({t.locale}, {t.kind})</span>
                </span>
                {canEdit && (
                  <Button
                    type="button" variant="ghost" size="sm"
                    aria-label={`Delete ${t.label || t.locale} captions`}
                    disabled={remove.isPending}
                    onClick={() => remove.mutate(t.id)}
                  >
                    <Trash2 className="size-4" />
                  </Button>
                )}
              </li>
            ))}
          </ul>
        )}

        {media && canEdit && (
          <div className="grid gap-2 sm:grid-cols-2">
            <label className="text-sm">
              <span className="mb-1 block font-medium">Language</span>
              <select value={locale} onChange={(e) => setLocale(e.target.value)} className="w-full rounded-md border border-input bg-background px-2 py-1.5 text-sm">
                {CAPTION_LANGUAGES.map((l) => <option key={l.code} value={l.code}>{l.label}</option>)}
              </select>
            </label>
            <label className="text-sm">
              <span className="mb-1 block font-medium">Type</span>
              <select value={kind} onChange={(e) => setKind(e.target.value)} className="w-full rounded-md border border-input bg-background px-2 py-1.5 text-sm">
                {KINDS.map((k) => <option key={k.value} value={k.value}>{k.label}</option>)}
              </select>
            </label>
            <label className="text-sm sm:col-span-2">
              <span className="mb-1 block font-medium">Caption file</span>
              <input
                type="file"
                accept=".vtt,.srt,text/vtt"
                onChange={(e) => { setFile(e.target.files?.[0] || null); setError(null); }}
                className="w-full text-sm"
              />
            </label>
          </div>
        )}
        {media && !canEdit && (
          <p className="text-sm text-muted-foreground">Only the person who added this media, or a teacher of its course, can change its captions.</p>
        )}
        {error && <p role="alert" className="text-sm text-destructive">{error}</p>}

        <DialogFooter>
          <Button type="button" variant="outline" onClick={onClose}>Done</Button>
          {media && canEdit && (
            <Button type="button" onClick={() => save.mutate()} disabled={!file || save.isPending}>
              {save.isPending ? 'Uploading…' : 'Add captions'}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
