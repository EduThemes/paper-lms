import React, { useState, useEffect, useCallback } from 'react';
import { useParams, useLocation } from 'react-router-dom';
import { Plus, Trash2, GripVertical, Save, RotateCcw, Award } from 'lucide-react';
import { api } from '../services/api';
import { useAuth } from '../contexts/AuthContext';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';

const DEFAULT_RATINGS = [
  { description: 'Exceeds Mastery', points: 4, mastery: false, color: '#02672D' },
  { description: 'Mastery', points: 3, mastery: true, color: '#127A1B' },
  { description: 'Near Mastery', points: 2, mastery: false, color: '#C66F00' },
  { description: 'Below Mastery', points: 1, mastery: false, color: '#E62429' },
];

/**
 * OutcomeProficiencyPage edits the proficiency scale for either an Account
 * (admin scope) or a Course. Scope is detected from the URL path.
 */
const OutcomeProficiencyPage = () => {
  const { courseId } = useParams();
  const location = useLocation();
  const { user } = useAuth();
  const isAccountScope = location.pathname.startsWith('/admin');
  const accountId = user?.account_id || 1;

  const [ratings, setRatings] = useState(DEFAULT_RATINGS);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState(null);
  const [success, setSuccess] = useState(null);
  const [dragIndex, setDragIndex] = useState(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = isAccountScope
        ? await api.getAccountOutcomeProficiency(accountId)
        : await api.getCourseOutcomeProficiency(courseId);
      const data = res.data || res;
      if (data && Array.isArray(data.ratings) && data.ratings.length > 0) {
        setRatings(
          data.ratings.map((r) => ({
            description: r.description || '',
            points: typeof r.points === 'number' ? r.points : 0,
            mastery: !!r.mastery,
            color: r.color || '#999999',
          }))
        );
      } else {
        setRatings(DEFAULT_RATINGS);
      }
    } catch (err) {
      setError(err.message || 'Failed to load proficiency scale');
    } finally {
      setLoading(false);
    }
  }, [isAccountScope, accountId, courseId]);

  useEffect(() => {
    load();
  }, [load]);

  const updateRating = (idx, patch) => {
    setRatings((rs) => rs.map((r, i) => (i === idx ? { ...r, ...patch } : r)));
  };

  const addRating = () => {
    setRatings((rs) => [
      ...rs,
      { description: 'New Rating', points: 0, mastery: false, color: '#999999' },
    ]);
  };

  const removeRating = (idx) => {
    setRatings((rs) => rs.filter((_, i) => i !== idx));
  };

  const setMastery = (idx) => {
    setRatings((rs) => rs.map((r, i) => ({ ...r, mastery: i === idx })));
  };

  const onDragStart = (idx) => setDragIndex(idx);
  const onDragOver = (e) => e.preventDefault();
  const onDrop = (idx) => {
    if (dragIndex == null || dragIndex === idx) return;
    setRatings((rs) => {
      const next = [...rs];
      const [moved] = next.splice(dragIndex, 1);
      next.splice(idx, 0, moved);
      return next;
    });
    setDragIndex(null);
  };

  const save = async () => {
    setSaving(true);
    setError(null);
    setSuccess(null);
    try {
      const payload = {
        ratings: ratings.map((r, i) => ({
          description: r.description,
          points: Number(r.points),
          mastery: r.mastery,
          color: r.color,
          position: i + 1,
        })),
      };
      if (isAccountScope) {
        await api.setAccountOutcomeProficiency(accountId, payload);
      } else {
        await api.setCourseOutcomeProficiency(courseId, payload);
      }
      setSuccess('Proficiency scale saved');
    } catch (err) {
      setError(err.message || 'Failed to save');
    } finally {
      setSaving(false);
    }
  };

  const resetToDefault = async () => {
    if (!confirm('Reset to the default proficiency scale? This will fall back to the parent scale.')) return;
    setSaving(true);
    setError(null);
    setSuccess(null);
    try {
      if (isAccountScope) {
        await api.deleteAccountOutcomeProficiency(accountId);
      } else {
        await api.deleteCourseOutcomeProficiency(courseId);
      }
      await load();
      setSuccess('Reset to default');
    } catch (err) {
      setError(err.message || 'Failed to reset');
    } finally {
      setSaving(false);
    }
  };

  const content = (
    <div className="max-w-4xl mx-auto p-6">
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <Award className="w-7 h-7 text-brand-600" />
          <div>
            <h1 className="text-2xl font-bold text-text-primary">Outcome Proficiency Scale</h1>
            <p className="text-sm text-text-secondary">
              {isAccountScope
                ? 'Default scale used by all courses in this account.'
                : 'Course-specific scale (overrides the account default).'}
            </p>
          </div>
        </div>
      </div>

      {error && (
        <div className="mb-4 rounded-md bg-accent-danger/10 border border-accent-danger/30 px-4 py-3 text-sm text-accent-danger">
          {error}
        </div>
      )}
      {success && (
        <div className="mb-4 rounded-md bg-accent-success/10 border border-accent-success/30 px-4 py-3 text-sm text-accent-success">
          {success}
        </div>
      )}

      {loading ? (
        <div className="flex items-center justify-center py-12">
          <svg className="animate-spin h-8 w-8 text-brand-600" viewBox="0 0 24 24" fill="none">
            <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
            <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" />
          </svg>
        </div>
      ) : (
        <div className="bg-surface-0 rounded-lg shadow border border-border-default overflow-hidden">
          <div className="grid grid-cols-12 gap-2 px-4 py-3 bg-surface-1 border-b border-border-default text-xs font-semibold uppercase text-text-secondary">
            <div className="col-span-1" />
            <div className="col-span-4">Description</div>
            <div className="col-span-2">Points</div>
            <div className="col-span-2">Color</div>
            <div className="col-span-2">Mastery</div>
            <div className="col-span-1" />
          </div>
          {ratings.map((r, idx) => (
            <div
              key={idx}
              draggable
              onDragStart={() => onDragStart(idx)}
              onDragOver={onDragOver}
              onDrop={() => onDrop(idx)}
              className="grid grid-cols-12 gap-2 px-4 py-3 border-b border-border-subtle items-center hover:bg-surface-1"
            >
              <div className="col-span-1 flex items-center justify-center cursor-move text-text-disabled">
                <GripVertical className="w-4 h-4" />
              </div>
              <div className="col-span-4">
                <input
                  type="text"
                  value={r.description}
                  onChange={(e) => updateRating(idx, { description: e.target.value })}
                  className="w-full rounded border border-border-strong px-2 py-1 text-sm"
                />
              </div>
              <div className="col-span-2">
                <input
                  type="number"
                  step="0.1"
                  value={r.points}
                  onChange={(e) => updateRating(idx, { points: parseFloat(e.target.value) || 0 })}
                  className="w-full rounded border border-border-strong px-2 py-1 text-sm"
                />
              </div>
              <div className="col-span-2 flex items-center gap-2">
                <input
                  type="color"
                  value={r.color}
                  onChange={(e) => updateRating(idx, { color: e.target.value })}
                  className="w-8 h-8 rounded border border-border-strong"
                />
                <input
                  type="text"
                  value={r.color}
                  onChange={(e) => updateRating(idx, { color: e.target.value })}
                  className="w-full rounded border border-border-strong px-2 py-1 text-xs font-mono"
                />
              </div>
              <div className="col-span-2 flex items-center">
                <label className="inline-flex items-center gap-2 text-sm">
                  <input
                    type="radio"
                    name="mastery"
                    checked={r.mastery}
                    onChange={() => setMastery(idx)}
                  />
                  <span>Mastery</span>
                </label>
              </div>
              <div className="col-span-1 flex items-center justify-center">
                <button
                  onClick={() => removeRating(idx)}
                  className="text-accent-danger hover:text-accent-danger"
                  aria-label="Remove rating"
                  disabled={ratings.length <= 2}
                >
                  <Trash2 className="w-4 h-4" />
                </button>
              </div>
            </div>
          ))}
          <div className="px-4 py-3 bg-surface-1 border-t border-border-default">
            <button
              onClick={addRating}
              className="inline-flex items-center gap-1 px-3 py-1.5 text-sm rounded border border-border-strong hover:bg-surface-0"
            >
              <Plus className="w-4 h-4" /> Add Rating
            </button>
          </div>
        </div>
      )}

      <div className="mt-6 flex items-center gap-3">
        <button
          onClick={save}
          disabled={saving || loading}
          className="inline-flex items-center gap-2 px-4 py-2 rounded bg-brand-600 text-white text-sm font-medium hover:bg-brand-700 disabled:opacity-50"
        >
          <Save className="w-4 h-4" /> {saving ? 'Saving...' : 'Save Scale'}
        </button>
        <button
          onClick={resetToDefault}
          disabled={saving || loading}
          className="inline-flex items-center gap-2 px-4 py-2 rounded border border-border-strong text-sm font-medium hover:bg-surface-1 disabled:opacity-50"
        >
          <RotateCcw className="w-4 h-4" /> Reset to Default
        </button>
      </div>
    </div>
  );

  return (
    <Layout>
      {!isAccountScope && courseId && <CourseNav courseId={courseId} />}
      {content}
    </Layout>
  );
};

export default OutcomeProficiencyPage;
