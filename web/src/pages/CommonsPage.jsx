import React, { useState, useEffect, useCallback } from 'react';
import { Search, Star, Download, X } from 'lucide-react';
import { api } from '../services/api';
import Layout from '../components/Layout';
import CommonsCard from '../components/CommonsCard';

const RESOURCE_TYPES = [
  { value: '', label: 'All' },
  { value: 'course', label: 'Courses' },
  { value: 'assignment', label: 'Assignments' },
  { value: 'page', label: 'Pages' },
  { value: 'quiz', label: 'Quizzes' },
  { value: 'module', label: 'Modules' },
  { value: 'discussion_topic', label: 'Discussions' },
];

const SUBJECTS = ['', 'Math', 'ELA', 'Science', 'Social Studies', 'Art', 'Music', 'PE', 'World Languages'];
const GRADE_LEVELS = ['', 'K-2', '3-5', '6-8', '9-12'];

const Spinner = () => (
  <svg className="animate-spin h-6 w-6 text-indigo-600" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" aria-hidden="true">
    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z"></path>
  </svg>
);

/**
 * CommonsPage — district-wide Canvas-Commons-equivalent catalog page.
 * Filters by resource type, subject, grade level, and free-text search.
 * Cards open a detail dialog with an "Import to course..." action.
 */
const CommonsPage = () => {
  const [items, setItems] = useState([]);
  const [favoriteIds, setFavoriteIds] = useState(new Set());
  const [showFavorites, setShowFavorites] = useState(false);
  const [resourceType, setResourceType] = useState('');
  const [subject, setSubject] = useState('');
  const [gradeLevel, setGradeLevel] = useState('');
  const [search, setSearch] = useState('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [selected, setSelected] = useState(null);
  const [courses, setCourses] = useState([]);
  const [importTargetCourseId, setImportTargetCourseId] = useState('');
  const [importing, setImporting] = useState(false);
  const [importMessage, setImportMessage] = useState('');

  const loadItems = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      if (showFavorites) {
        const result = await api.listCommonsFavorites();
        const data = result.data || [];
        setItems(data);
        setFavoriteIds(new Set(data.map((d) => d.id)));
      } else {
        const result = await api.browseCommons({
          resource_type: resourceType,
          subject,
          grade_level: gradeLevel,
          q: search,
        });
        setItems(result.data || []);
        // Fetch favorites in parallel to highlight stars correctly.
        try {
          const favs = await api.listCommonsFavorites();
          setFavoriteIds(new Set((favs.data || []).map((f) => f.id)));
        } catch {
          /* non-fatal */
        }
      }
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [resourceType, subject, gradeLevel, search, showFavorites]);

  useEffect(() => {
    loadItems();
  }, [loadItems]);

  useEffect(() => {
    // Pre-load courses for the import dropdown.
    api
      .getCourses?.()
      ?.then((r) => setCourses(r.data || []))
      .catch(() => setCourses([]));
  }, []);

  const handleFavorite = async (item) => {
    try {
      const result = await api.toggleCommonsFavorite(item.id);
      const favorited = result.data?.favorited;
      setFavoriteIds((prev) => {
        const next = new Set(prev);
        if (favorited) next.add(item.id);
        else next.delete(item.id);
        return next;
      });
      // Optimistically bump the count in-place.
      setItems((prev) =>
        prev.map((it) =>
          it.id === item.id
            ? { ...it, favorite_count: Math.max(0, (it.favorite_count || 0) + (favorited ? 1 : -1)) }
            : it,
        ),
      );
    } catch (err) {
      setError(err.message);
    }
  };

  const handleOpen = async (item) => {
    setImportMessage('');
    setImportTargetCourseId('');
    try {
      const result = await api.getCommonsItem(item.id);
      setSelected(result.data);
    } catch (err) {
      setError(err.message);
    }
  };

  const handleImport = async () => {
    if (!selected || !importTargetCourseId) return;
    setImporting(true);
    setImportMessage('');
    try {
      await api.importCommons(selected.id, Number(importTargetCourseId));
      setImportMessage('Imported successfully.');
    } catch (err) {
      setImportMessage(err.message);
    } finally {
      setImporting(false);
    }
  };

  return (
    <Layout>
      <div className="p-6 max-w-7xl mx-auto">
        <div className="mb-6">
          <h1 className="text-2xl font-bold text-slate-900">Commons</h1>
          <p className="text-sm text-slate-500 mt-1">
            Browse and import shared content from teachers across your district.
          </p>
        </div>

        <div className="mb-4 bg-surface-0 border border-slate-200 rounded-lg p-4 space-y-3">
          <div className="flex flex-wrap gap-2">
            {RESOURCE_TYPES.map((rt) => (
              <button
                key={rt.value || 'all'}
                onClick={() => setResourceType(rt.value)}
                className={`px-3 py-1 text-xs font-medium rounded-full border transition ${
                  resourceType === rt.value
                    ? 'bg-indigo-600 text-white border-indigo-600'
                    : 'bg-surface-0 text-slate-700 border-slate-200 hover:border-indigo-400'
                }`}
              >
                {rt.label}
              </button>
            ))}
          </div>
          <div className="flex flex-wrap gap-3 items-center">
            <label className="flex items-center gap-2 text-sm">
              <span className="text-slate-600">Subject</span>
              <select
                value={subject}
                onChange={(e) => setSubject(e.target.value)}
                className="px-2 py-1 border border-slate-200 rounded text-sm bg-surface-0"
              >
                {SUBJECTS.map((s) => (
                  <option key={s || 'all'} value={s}>
                    {s || 'All'}
                  </option>
                ))}
              </select>
            </label>
            <label className="flex items-center gap-2 text-sm">
              <span className="text-slate-600">Grade</span>
              <select
                value={gradeLevel}
                onChange={(e) => setGradeLevel(e.target.value)}
                className="px-2 py-1 border border-slate-200 rounded text-sm bg-surface-0"
              >
                {GRADE_LEVELS.map((g) => (
                  <option key={g || 'all'} value={g}>
                    {g || 'All'}
                  </option>
                ))}
              </select>
            </label>
            <div className="flex items-center gap-1 flex-1 min-w-[200px]">
              <Search className="w-4 h-4 text-slate-400" aria-hidden="true" />
              <input
                type="search"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder="Search title or description"
                className="flex-1 px-2 py-1 border border-slate-200 rounded text-sm"
              />
            </div>
            <label className="inline-flex items-center gap-2 text-sm text-slate-700">
              <input
                type="checkbox"
                checked={showFavorites}
                onChange={(e) => setShowFavorites(e.target.checked)}
              />
              <Star className="w-3.5 h-3.5" aria-hidden="true" /> Favorites only
            </label>
          </div>
        </div>

        {loading ? (
          <div className="flex justify-center py-12"><Spinner /></div>
        ) : error ? (
          <div className="bg-accent-danger/10 border border-accent-danger/30 text-accent-danger p-4 rounded-lg">
            <p>{error}</p>
            <button
              onClick={loadItems}
              className="mt-2 px-3 py-1 text-sm bg-accent-danger text-white rounded hover:bg-accent-danger/90"
            >
              Try Again
            </button>
          </div>
        ) : items.length === 0 ? (
          <div className="bg-slate-50 border border-slate-200 rounded-lg p-12 text-center text-slate-500">
            No content found. Try adjusting your filters.
          </div>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4">
            {items.map((item) => (
              <CommonsCard
                key={item.id}
                item={item}
                isFavorited={favoriteIds.has(item.id)}
                onFavorite={handleFavorite}
                onClick={handleOpen}
              />
            ))}
          </div>
        )}
      </div>

      {selected && (
        <div
          className="fixed inset-0 bg-black/50 z-50 flex items-center justify-center p-4"
          role="dialog"
          aria-modal="true"
          onClick={() => setSelected(null)}
        >
          <div
            className="bg-surface-0 rounded-lg shadow-xl max-w-2xl w-full max-h-[90vh] overflow-y-auto"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="p-6">
              <div className="flex items-start justify-between gap-4">
                <h2 className="text-xl font-semibold text-slate-900">{selected.title}</h2>
                <button
                  onClick={() => setSelected(null)}
                  className="p-1 hover:bg-slate-100 rounded"
                  aria-label="Close"
                >
                  <X className="w-5 h-5" />
                </button>
              </div>
              <div className="mt-2 flex flex-wrap gap-2 text-xs">
                <span className="px-2 py-0.5 bg-indigo-100 text-indigo-700 rounded">
                  {selected.resource_type}
                </span>
                {selected.subject && (
                  <span className="px-2 py-0.5 bg-slate-100 text-slate-700 rounded">{selected.subject}</span>
                )}
                {selected.grade_level && (
                  <span className="px-2 py-0.5 bg-slate-100 text-slate-700 rounded">{selected.grade_level}</span>
                )}
                {(selected.tags || []).map((t) => (
                  <span key={t} className="px-2 py-0.5 bg-accent-warning/10 text-accent-warning rounded">#{t}</span>
                ))}
              </div>
              <p className="mt-4 text-sm text-slate-600 whitespace-pre-wrap">
                {selected.description || 'No description provided.'}
              </p>
              <div className="mt-4 flex items-center gap-4 text-xs text-slate-500">
                <span className="flex items-center gap-1"><Download className="w-3.5 h-3.5" /> {selected.download_count || 0} imports</span>
                <span className="flex items-center gap-1"><Star className="w-3.5 h-3.5" /> {selected.favorite_count || 0} favorites</span>
              </div>

              <div className="mt-6 border-t border-slate-200 pt-4">
                <label className="block text-sm font-medium text-slate-700 mb-1">
                  Import to course
                </label>
                <div className="flex gap-2">
                  <select
                    value={importTargetCourseId}
                    onChange={(e) => setImportTargetCourseId(e.target.value)}
                    className="flex-1 px-2 py-1.5 border border-slate-200 rounded text-sm bg-surface-0"
                  >
                    <option value="">Select a course...</option>
                    {courses.map((c) => (
                      <option key={c.id} value={c.id}>
                        {c.name}
                      </option>
                    ))}
                  </select>
                  <button
                    onClick={handleImport}
                    disabled={!importTargetCourseId || importing}
                    className="px-4 py-1.5 bg-indigo-600 text-white text-sm rounded hover:bg-indigo-700 disabled:opacity-50"
                  >
                    {importing ? 'Importing...' : 'Import'}
                  </button>
                </div>
                {importMessage && (
                  <p className="mt-2 text-sm text-slate-700">{importMessage}</p>
                )}
              </div>
            </div>
          </div>
        </div>
      )}
    </Layout>
  );
};

export default CommonsPage;
