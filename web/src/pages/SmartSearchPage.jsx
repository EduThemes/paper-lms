import React, { useState, useEffect, useMemo, useRef } from 'react';
import { useParams, Link } from 'react-router-dom';
import {
  Search,
  FileText,
  MessageSquare,
  Megaphone,
  BookOpen,
  Sparkles,
  AlertTriangle,
} from 'lucide-react';
import { api } from '../services/api';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import { Input } from '@/components/ui/input';
import { Card } from '@/components/ui/card';

// Lucide icon + Tailwind palette per content type. Anything else falls back
// to a generic Sparkles + slate palette.
const TYPE_META = {
  announcement: {
    label: 'Announcement',
    Icon: Megaphone,
    color: 'bg-accent-warning/10 text-accent-warning ring-amber-200',
    href: (courseId, id) => `/courses/${courseId}/announcements`,
  },
  assignment: {
    label: 'Assignment',
    Icon: FileText,
    color: 'bg-brand-50 text-brand-700 ring-blue-200',
    href: (courseId, id) => `/courses/${courseId}/assignments/${id}`,
  },
  page: {
    label: 'Page',
    Icon: BookOpen,
    color: 'bg-accent-success/10 text-emerald-700 ring-emerald-200',
    href: (courseId, id) => `/courses/${courseId}/pages`,
  },
  discussion_topic: {
    label: 'Discussion',
    Icon: MessageSquare,
    color: 'bg-purple-50 text-purple-700 ring-purple-200',
    href: (courseId, id) => `/courses/${courseId}/discussions`,
  },
};

const FALLBACK_META = {
  label: 'Result',
  Icon: Sparkles,
  color: 'bg-slate-50 text-slate-700 ring-slate-200',
  href: (courseId) => `/courses/${courseId}`,
};

// Small inline animated SVG spinner (PROJECT.md convention — no plain "Loading..." text).
const Spinner = () => (
  <svg
    className="animate-spin h-5 w-5 text-slate-500"
    xmlns="http://www.w3.org/2000/svg"
    fill="none"
    viewBox="0 0 24 24"
    aria-hidden="true"
  >
    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
    <path
      className="opacity-75"
      fill="currentColor"
      d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z"
    />
  </svg>
);

const ScoreChip = ({ score }) => {
  // Score is cosine similarity in [-1, 1]; show as percent for human eyes.
  const pct = Math.max(0, Math.min(100, Math.round(((score ?? 0) + 1) * 50)));
  return (
    <span className="inline-flex items-center rounded-full bg-slate-100 px-2 py-0.5 text-xs font-medium text-slate-600 ring-1 ring-inset ring-slate-200">
      {pct}% match
    </span>
  );
};

const SmartSearchPage = () => {
  const { courseId } = useParams();
  const [query, setQuery] = useState('');
  const [debounced, setDebounced] = useState('');
  const [results, setResults] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const [hasSearched, setHasSearched] = useState(false);
  const lastReqId = useRef(0);

  // Debounce the query (300ms) so each keystroke doesn't fire a request.
  useEffect(() => {
    const t = setTimeout(() => setDebounced(query.trim()), 300);
    return () => clearTimeout(t);
  }, [query]);

  useEffect(() => {
    if (!debounced) {
      setResults([]);
      setHasSearched(false);
      return;
    }
    const reqId = ++lastReqId.current;
    setLoading(true);
    setError(null);
    const fetch = async () => {
      try {
        // The integration agent will add api.smartSearch — fall back to
        // raw request() so this page works the moment the route is wired.
        const fn =
          typeof api.smartSearch === 'function'
            ? api.smartSearch
            : (cid, q) => api.request(`/courses/${cid}/smart_search?q=${encodeURIComponent(q)}`);
        const result = await fn(courseId, debounced);
        if (reqId !== lastReqId.current) return; // stale response
        setResults(result?.results || []);
        setHasSearched(true);
      } catch (err) {
        if (reqId !== lastReqId.current) return;
        setError(err.message || 'Search failed');
      } finally {
        if (reqId === lastReqId.current) setLoading(false);
      }
    };
    fetch();
  }, [debounced, courseId]);

  // Group results by content type for a tidy ranked list.
  const grouped = useMemo(() => {
    const out = {};
    for (const r of results) {
      const key = r.content_type || 'other';
      if (!out[key]) out[key] = [];
      out[key].push(r);
    }
    return out;
  }, [results]);

  const retry = () => setDebounced((d) => d + '');

  return (
    <Layout>
      <CourseNav courseId={courseId} />
      <div className="p-6 max-w-4xl mx-auto">
        <header className="mb-6">
          <h1 className="text-2xl font-semibold text-slate-900 flex items-center gap-2">
            <Sparkles className="h-6 w-6 text-indigo-500" aria-hidden="true" />
            Smart Search
          </h1>
          <p className="text-sm text-slate-500 mt-1">
            Search announcements, assignments, pages, and discussions in this course.
          </p>
        </header>

        <div className="relative mb-6">
          <Search
            className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-slate-400 pointer-events-none"
            aria-hidden="true"
          />
          <Input
            type="search"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search course content…"
            className="pl-9"
            aria-label="Search course content"
          />
        </div>

        {loading && (
          <div className="flex items-center gap-2 text-slate-500 text-sm">
            <Spinner /> Searching…
          </div>
        )}

        {error && !loading && (
          <Card className="p-4 border-accent-danger/30 bg-accent-danger/10">
            <div className="flex items-start gap-3">
              <AlertTriangle className="h-5 w-5 text-accent-danger mt-0.5" aria-hidden="true" />
              <div className="flex-1">
                <div className="text-sm font-medium text-accent-danger">Search failed</div>
                <div className="text-sm text-accent-danger mt-1">{error}</div>
                <button
                  type="button"
                  onClick={retry}
                  className="mt-2 text-sm font-medium text-accent-danger underline hover:text-red-900"
                >
                  Try Again
                </button>
              </div>
            </div>
          </Card>
        )}

        {!loading && !error && hasSearched && results.length === 0 && (
          <div className="text-center py-12 text-slate-500">
            No matching content found.
          </div>
        )}

        {!loading && !error && results.length > 0 && (
          <div className="space-y-6">
            {Object.entries(grouped).map(([type, items]) => {
              const meta = TYPE_META[type] || FALLBACK_META;
              const { Icon, label, color } = meta;
              return (
                <section key={type} aria-labelledby={`group-${type}`}>
                  <h2
                    id={`group-${type}`}
                    className="flex items-center gap-2 text-sm font-semibold text-slate-700 mb-2"
                  >
                    <span
                      className={`inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs ring-1 ring-inset ${color}`}
                    >
                      <Icon className="h-3.5 w-3.5" aria-hidden="true" />
                      {label}
                    </span>
                    <span className="text-slate-400 font-normal">{items.length}</span>
                  </h2>
                  <ul className="space-y-2">
                    {items.map((r) => (
                      <li key={`${r.content_type}-${r.content_id}`}>
                        <Card className="p-4 hover:shadow-sm transition-shadow">
                          <div className="flex items-start justify-between gap-3">
                            <div className="min-w-0">
                              <Link
                                to={meta.href(courseId, r.content_id)}
                                className="text-base font-medium text-slate-900 hover:text-indigo-600 truncate block"
                              >
                                {r.title || '(untitled)'}
                              </Link>
                              {r.excerpt && (
                                <p className="text-sm text-slate-600 mt-1 line-clamp-2">
                                  {r.excerpt}
                                </p>
                              )}
                            </div>
                            <ScoreChip score={r.score} />
                          </div>
                        </Card>
                      </li>
                    ))}
                  </ul>
                </section>
              );
            })}
          </div>
        )}
      </div>
    </Layout>
  );
};

export default SmartSearchPage;
