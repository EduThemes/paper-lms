import React, { useEffect, useState, useMemo } from 'react';
import { api } from '../../services/api';
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription,
} from '../ui/dialog';
import { Input } from '../ui/input';
import { Button } from '../ui/button';

/**
 * @typedef {Object} ContentPickerProps
 * @property {(string|number)} courseId
 * @property {(href: string, label: string, opts?: {embed?: 'image'|'video'|'audio'}) => void} onInsert
 * @property {() => void} onClose
 */

const TYPES = [
  { key: 'pages', label: 'Pages' },
  { key: 'assignments', label: 'Assignments' },
  { key: 'discussions', label: 'Discussions' },
  { key: 'announcements', label: 'Announcements' },
  { key: 'modules', label: 'Modules' },
  { key: 'quizzes', label: 'Quizzes' },
  { key: 'files', label: 'Files' },
];

/**
 * How a course file can be embedded rather than linked (G011): images as
 * an <img>, audio/video as a player. Other files (PDF, Office, ...) are
 * links; the download route serves them as attachments by policy.
 */
export function fileEmbedKind(file) {
  const type = String(file?.content_type || '').toLowerCase();
  const name = String(file?.filename || file?.display_name || '').toLowerCase();
  if (/^image\/(png|jpe?g|gif|webp|avif)$/.test(type)) return 'image';
  if (type.startsWith('video/') || /\.(mp4|m4v|webm|ogv|mov)$/.test(name)) return 'video';
  if (type.startsWith('audio/') || /\.(mp3|m4a|wav|ogg|oga|aac)$/.test(name)) return 'audio';
  return null;
}

const PICKER_PER_PAGE = 100;
const PICKER_MAX_PAGES = 20;

/**
 * Fetch every page of a paginated list endpoint. The list APIs default to 10
 * rows per page and the picker filters client-side, so loading only the first
 * page hid everything past item 10 (B106). Resilient to either {data:[]} or
 * [] returns; follows the Link rel="next" header, or keeps going while full
 * pages come back.
 */
export async function fetchAllPages(fetchPage) {
  const all = [];
  for (let page = 1; page <= PICKER_MAX_PAGES; page += 1) {
    const r = await fetchPage(page, PICKER_PER_PAGE);
    const rows = Array.isArray(r) ? r : (r?.data ?? []);
    all.push(...rows);
    const hasNext = Boolean(r?.pagination?.next) || rows.length === PICKER_PER_PAGE;
    if (!hasNext || rows.length === 0) break;
  }
  return all;
}

/** Map type → API call + result-shaping. */
export async function loadType(type, courseId) {
  switch (type) {
    case 'pages': {
      const r = await fetchAllPages((page, per) => api.getPages(courseId, page, per));
      return r.map((p) => ({
        id: p.page_id || p.url || p.id,
        label: p.title || p.url || `Page ${p.id}`,
        href: `/courses/${courseId}/pages/${p.url || p.page_id || p.id}`,
      }));
    }
    case 'assignments': {
      const r = await fetchAllPages((page, per) => api.getAssignments(courseId, page, per));
      return r.map((a) => ({
        id: a.id, label: a.name || `Assignment ${a.id}`,
        href: `/courses/${courseId}/assignments/${a.id}`,
      }));
    }
    case 'discussions': {
      const r = await fetchAllPages((page, per) => api.getDiscussionTopics(courseId, page, per));
      return r.map((d) => ({
        id: d.id, label: d.title || `Discussion ${d.id}`,
        href: `/courses/${courseId}/discussions/${d.id}`,
      }));
    }
    case 'announcements': {
      const r = await fetchAllPages((page, per) => api.getCourseAnnouncements(courseId, page, per));
      // The announcements page gives every announcement an
      // id="announcement-N" anchor and scrolls to the URL hash.
      return r.map((a) => ({
        id: a.id, label: a.title || `Announcement ${a.id}`,
        href: `/courses/${courseId}/announcements#announcement-${a.id}`,
      }));
    }
    case 'modules': {
      const r = await fetchAllPages((page, per) => api.getModules(courseId, page, per, false));
      // ModulesPage renders id="module-N" on each module and scrolls to it.
      return r.map((m) => ({
        id: m.id, label: m.name || `Module ${m.id}`,
        href: `/courses/${courseId}/modules#module-${m.id}`,
      }));
    }
    case 'quizzes': {
      const r = await fetchAllPages((page, per) => api.getQuizzes(courseId, page, per));
      // Canvas-style quiz URL; App.jsx routes quizzes/:quizId to the quiz
      // landing (take) page, so the link works for students and imported
      // Canvas links resolve too (B106).
      return r.map((q) => ({
        id: q.id, label: q.title || `Quiz ${q.id}`,
        href: `/courses/${courseId}/quizzes/${q.id}`,
      }));
    }
    case 'files': {
      const r = await fetchAllPages((page, per) => api.getCourseFiles(courseId, page, per));
      return r.map((f) => ({
        id: f.id, label: f.display_name || f.filename || `File ${f.id}`,
        href: api.getFileDownloadUrl(f.id),
        embed: fileEmbedKind(f),
      }));
    }
    default:
      return [];
  }
}

/**
 * Side-panel that lists course content for insertion.
 * @param {ContentPickerProps} props
 */
export default function ContentPicker({ courseId, onInsert, onClose }) {
  const [activeType, setActiveType] = useState('pages');
  const [items, setItems] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const [query, setQuery] = useState('');

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    loadType(activeType, courseId)
      .then((rows) => { if (!cancelled) setItems(rows); })
      .catch((e) => { if (!cancelled) setError(e?.message || 'Failed to load'); })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [activeType, courseId]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return items;
    return items.filter((it) => (it.label || '').toLowerCase().includes(q));
  }, [items, query]);

  return (
    <Dialog open={true} onOpenChange={(o) => { if (!o) onClose(); }}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>Insert course content link</DialogTitle>
          <DialogDescription>
            Pick course content to link. Selected text becomes the link; images, audio and video files can also be embedded.
          </DialogDescription>
        </DialogHeader>

        <div role="tablist" aria-label="Content type" className="flex flex-wrap gap-1 border-b">
          {TYPES.map((t) => (
            <button
              key={t.key}
              role="tab"
              aria-selected={activeType === t.key}
              type="button"
              onClick={() => setActiveType(t.key)}
              className={`px-3 py-1.5 text-sm border-b-2 -mb-px ${activeType === t.key ? 'border-primary text-foreground' : 'border-transparent text-muted-foreground hover:text-foreground'}`}
            >
              {t.label}
            </button>
          ))}
        </div>

        <Input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Filter…"
          aria-label="Filter content"
          autoFocus
        />

        <div role="listbox" aria-label={`${activeType} results`} className="max-h-72 overflow-auto rounded-md border">
          {loading && <div className="p-3 text-sm text-muted-foreground">Loading…</div>}
          {!loading && error && (
            <div className="p-3 text-sm text-destructive">
              {error}{' '}
              <Button size="sm" variant="outline" onClick={() => setActiveType((t) => t)}>Try Again</Button>
            </div>
          )}
          {!loading && !error && filtered.length === 0 && (
            <div className="p-3 text-sm text-muted-foreground">No items.</div>
          )}
          {!loading && !error && filtered.map((it) => (
            <div key={it.id} className="flex items-center gap-1 pr-2 hover:bg-accent">
              <button
                role="option"
                aria-selected="false"
                type="button"
                onClick={() => onInsert(it.href, it.label)}
                className="block min-w-0 flex-1 truncate px-3 py-2 text-left text-sm focus:bg-accent focus:outline-none"
              >
                {it.label}
              </button>
              {it.embed && (
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  aria-label={`Embed ${it.label}`}
                  onClick={() => onInsert(it.href, it.label, { embed: it.embed })}
                >
                  {it.embed === 'image' ? 'Embed image' : 'Embed player'}
                </Button>
              )}
            </div>
          ))}
        </div>
      </DialogContent>
    </Dialog>
  );
}
