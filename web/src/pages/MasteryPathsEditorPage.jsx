import React, { useEffect, useState, useCallback } from 'react';
import { useParams, Link } from 'react-router-dom';
import { Plus, Save, ArrowLeft } from 'lucide-react';
import { api } from '../services/api';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import ScoreRangeRow from '../components/mastery/ScoreRangeRow';
import { Button } from '../components/ui/button';
import { Card } from '../components/ui/card';

const DEFAULT_RANGES = [
  { lower_bound: 0, upper_bound: 70, assignment_ids: [] },
  { lower_bound: 70, upper_bound: 90, assignment_ids: [] },
  { lower_bound: 90, upper_bound: 100, assignment_ids: [] },
];

/**
 * Per-assignment Mastery Paths editor.
 *
 * Lets a teacher define 2-3 score ranges and pick the follow-up assignments
 * each range unlocks. Persists via the /mastery_paths/rules API.
 */
const MasteryPathsEditorPage = () => {
  const { courseId, assignmentId } = useParams();

  const [assignment, setAssignment] = useState(null);
  const [allAssignments, setAllAssignments] = useState([]);
  const [ranges, setRanges] = useState(DEFAULT_RANGES);
  const [existingRule, setExistingRule] = useState(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState(null);
  const [savedAt, setSavedAt] = useState(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [assn, list, rule] = await Promise.all([
        api.getAssignment(courseId, assignmentId),
        api.getAssignments(courseId, 1, 100),
        api.getMasteryPathRule(courseId, assignmentId).catch(() => null),
      ]);
      setAssignment(assn?.data || assn);
      const items = list?.data || list || [];
      // Exclude the trigger from selectable assignments.
      setAllAssignments(items.filter((a) => String(a.id) !== String(assignmentId)));

      if (rule && rule.scoring_ranges?.length) {
        setExistingRule(rule);
        setRanges(
          rule.scoring_ranges.map((sr) => ({
            lower_bound: Math.round(sr.lower_bound * 100),
            upper_bound: Math.round(sr.upper_bound * 100),
            assignment_ids: (sr.assignment_sets?.[0]?.assignment_set_associations || []).map(
              (a) => a.assignment_id,
            ),
          })),
        );
      }
    } catch (err) {
      setError(err.message || 'Failed to load mastery paths');
    } finally {
      setLoading(false);
    }
  }, [courseId, assignmentId]);

  useEffect(() => {
    load();
  }, [load]);

  const updateRange = (i, next) => {
    setRanges((prev) => prev.map((r, idx) => (idx === i ? next : r)));
  };

  const removeRange = (i) => {
    if (ranges.length <= 2) return;
    setRanges((prev) => prev.filter((_, idx) => idx !== i));
  };

  const addRange = () => {
    if (ranges.length >= 3) return;
    setRanges((prev) => [
      ...prev,
      { lower_bound: 0, upper_bound: 100, assignment_ids: [] },
    ]);
  };

  const handleSave = async () => {
    setSaving(true);
    setError(null);
    try {
      const payload = {
        trigger_assignment_id: parseInt(assignmentId, 10),
        scoring_ranges: ranges.map((r, idx) => ({
          lower_bound: r.lower_bound,
          upper_bound: r.upper_bound,
          position: idx + 1,
          assignment_ids: r.assignment_ids,
        })),
      };
      let saved;
      if (existingRule?.id) {
        saved = await api.updateMasteryPathRule(courseId, existingRule.id, {
          scoring_ranges: payload.scoring_ranges,
        });
      } else {
        saved = await api.createMasteryPathRule(courseId, payload);
      }
      setExistingRule(saved);
      setSavedAt(new Date());
    } catch (err) {
      setError(err.message || 'Failed to save mastery paths');
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    if (!existingRule?.id) return;
    if (!window.confirm('Delete this Mastery Paths rule? Students will no longer have follow-up assignments unlocked by this rule.')) {
      return;
    }
    setSaving(true);
    try {
      await api.deleteMasteryPathRule(courseId, existingRule.id);
      setExistingRule(null);
      setRanges(DEFAULT_RANGES);
    } catch (err) {
      setError(err.message || 'Failed to delete rule');
    } finally {
      setSaving(false);
    }
  };

  return (
    <Layout>
      <CourseNav courseId={courseId} />
      <div className="max-w-4xl mx-auto p-6">
        <div className="flex items-center gap-3 mb-2">
          <Link
            to={`/courses/${courseId}/assignments/${assignmentId}`}
            className="text-brand-600 hover:underline inline-flex items-center text-sm"
          >
            <ArrowLeft size={14} className="mr-1" />
            Back to assignment
          </Link>
        </div>
        <h1 className="text-2xl font-bold text-text-primary mb-1">Mastery Paths</h1>
        <p className="text-text-secondary mb-6">
          {assignment?.name
            ? `Conditional follow-ups for "${assignment.name}".`
            : 'Conditional follow-ups for this assignment.'}{' '}
          When a student is graded on this assignment, they'll automatically be
          assigned the work in the matching score band.
        </p>

        {loading ? (
          <div className="flex items-center justify-center py-12">
            <svg className="animate-spin h-6 w-6 text-brand-600" viewBox="0 0 24 24">
              <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
              <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" />
            </svg>
          </div>
        ) : (
          <>
            {error && (
              <Card className="p-4 mb-4 bg-accent-danger/10 border-accent-danger/30 text-accent-danger">
                <p>{error}</p>
                <Button size="sm" variant="secondary" className="mt-2" onClick={load}>
                  Try Again
                </Button>
              </Card>
            )}

            <div className="space-y-4">
              {ranges.map((range, i) => (
                <ScoreRangeRow
                  key={i}
                  index={i}
                  range={range}
                  allAssignments={allAssignments}
                  onChange={(next) => updateRange(i, next)}
                  onRemove={() => removeRange(i)}
                  canRemove={ranges.length > 2}
                />
              ))}
            </div>

            <div className="flex items-center gap-3 mt-6">
              {ranges.length < 3 && (
                <Button type="button" variant="secondary" onClick={addRange}>
                  <Plus size={14} className="mr-1" />
                  Add range
                </Button>
              )}
              <div className="flex-1" />
              {existingRule?.id && (
                <Button
                  type="button"
                  variant="ghost"
                  onClick={handleDelete}
                  disabled={saving}
                >
                  Delete rule
                </Button>
              )}
              <Button type="button" onClick={handleSave} disabled={saving}>
                <Save size={14} className="mr-1" />
                {saving ? 'Saving…' : 'Save Mastery Paths'}
              </Button>
            </div>
            {savedAt && !saving && (
              <p className="text-sm text-accent-success mt-2">
                Saved at {savedAt.toLocaleTimeString()}.
              </p>
            )}
          </>
        )}
      </div>
    </Layout>
  );
};

export default MasteryPathsEditorPage;
