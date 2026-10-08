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
 * @property {(href: string, label: string) => void} onInsert
 * @property {() => void} onClose
 */

const TYPES = [
  { key: 'pages', label: 'Pages' },
  { key: 'assignments', label: 'Assignments' },
  { key: 'modules', label: 'Modules' },
  { key: 'quizzes', label: 'Quizzes' },
  { key: 'files', label: 'Files' },
];

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
    case 'modules': {
      const r = await fetchAllPages((page, per) => api.getModules(courseId, page, per, false));
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
            Pick a page, assignment, module, quiz or file to insert as a link.
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
            <button
              key={it.id}
              role="option"
              type="button"
              onClick={() => onInsert(it.href, it.label)}
              className="block w-full px-3 py-2 text-left text-sm hover:bg-accent focus:bg-accent focus:outline-none"
            >
              {it.label}
            </button>
          ))}
        </div>
      </DialogContent>
    </Dialog>
  );
}
