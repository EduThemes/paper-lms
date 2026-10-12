import React, { useEffect, useMemo, useState } from 'react';
import { Send, Trash2, UserPlus, Star, MessageCircle } from 'lucide-react';
import gradingApi from '../../services/api_grading_ui';

// Peer-review pieces: the reviewer's form (plain score or rubric), the
// reviewee's view of the feedback received, and the instructor's manual
// assignment controls.

// parseRubricCriteria reads the criteria array from an assignment
// rubric payload ({ rubric: { data } }).
export const parseRubricCriteria = (rubricPayload) => {
  try {
    const data = rubricPayload?.rubric?.data ?? rubricPayload?.data;
    const criteria = typeof data === 'string' ? JSON.parse(data) : data;
    return Array.isArray(criteria) ? criteria : [];
  } catch {
    return [];
  }
};

const criterionKey = (c) => String(c.id || c.description);

export const PeerReviewForm = ({ review, pointsPossible, criteria = [], onSubmitted }) => {
  const [score, setScore] = useState('');
  const [comments, setComments] = useState('');
  const [rubric, setRubric] = useState({});
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState(null);
  const useRubric = criteria.length > 0;

  const rubricTotal = useMemo(
    () => Object.values(rubric).reduce((s, r) => s + (parseFloat(r.points) || 0), 0),
    [rubric]
  );

  const canSubmit = useRubric
    ? criteria.every((c) => rubric[criterionKey(c)]?.points !== undefined && rubric[criterionKey(c)]?.points !== '')
    : score !== '';

  const submit = async () => {
    setSubmitting(true);
    setError(null);
    try {
      const rubricData = useRubric
        ? Object.fromEntries(Object.entries(rubric).map(([k, v]) => [k, { points: parseFloat(v.points) || 0, ...(v.comments ? { comments: v.comments } : {}) }]))
        : undefined;
      const saved = await gradingApi.submitPeerReview(review.id, {
        score: useRubric ? undefined : score,
        comments,
        rubricData,
      });
      onSubmitted?.(saved);
    } catch (err) {
      setError(err.message);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="mt-3 space-y-3 border-t pt-3">
      {useRubric ? (
        <div className="space-y-3">
          {criteria.map((c) => {
            const k = criterionKey(c);
            const cur = rubric[k] || {};
            return (
              <div key={k} className="border border-border-default rounded p-2">
                <div className="flex items-center justify-between text-sm">
                  <span className="font-medium text-text-secondary">{c.description}</span>
                  <span className="text-xs text-text-tertiary">{c.points} pts</span>
                </div>
                {Array.isArray(c.ratings) && c.ratings.length > 0 && (
                  <div className="flex flex-wrap gap-1 mt-1">
                    {c.ratings.map((r) => (
                      <button
                        key={r.id || r.description}
                        type="button"
                        onClick={() => setRubric((p) => ({ ...p, [k]: { ...p[k], points: r.points } }))}
                        className={`text-xs px-2 py-1 rounded border ${
                          parseFloat(cur.points) === r.points ? 'bg-purple-100 border-purple-400 text-purple-800' : 'border-border-default text-text-secondary'
                        }`}
                      >
                        {r.points} - {r.description}
                      </button>
                    ))}
                  </div>
                )}
                <div className="flex gap-2 mt-1">
                  <input
                    type="number"
                    min="0"
                    max={c.points}
                    step="any"
                    aria-label={`Points for ${c.description}`}
                    value={cur.points ?? ''}
                    onChange={(e) => setRubric((p) => ({ ...p, [k]: { ...p[k], points: e.target.value } }))}
                    className="w-20 border border-border-strong rounded px-2 py-1 text-xs"
                  />
                  <input
                    type="text"
                    aria-label={`Comment for ${c.description}`}
                    placeholder="Comment..."
                    value={cur.comments ?? ''}
                    onChange={(e) => setRubric((p) => ({ ...p, [k]: { ...p[k], comments: e.target.value } }))}
                    className="flex-1 border border-border-strong rounded px-2 py-1 text-xs"
                  />
                </div>
              </div>
            );
          })}
          <p className="text-sm text-text-secondary">Rubric total: {rubricTotal}</p>
        </div>
      ) : (
        <div>
          <label htmlFor={`peer-score-${review.id}`} className="block text-sm font-medium text-text-secondary mb-1">
            Score {pointsPossible ? `(out of ${pointsPossible})` : ''}
          </label>
          <input
            id={`peer-score-${review.id}`}
            type="number"
            min="0"
            max={pointsPossible || undefined}
            value={score}
            onChange={(e) => setScore(e.target.value)}
            className="w-32 border border-border-strong rounded-md px-3 py-2 text-sm"
            placeholder="0"
          />
        </div>
      )}
      <div>
        <label htmlFor={`peer-comments-${review.id}`} className="block text-sm font-medium text-text-secondary mb-1">Comments</label>
        <textarea
          id={`peer-comments-${review.id}`}
          value={comments}
          onChange={(e) => setComments(e.target.value)}
          className="w-full border border-border-strong rounded-md px-3 py-2 text-sm"
          rows={3}
          placeholder="Provide feedback on this submission..."
        />
      </div>
      {error && <p className="text-sm text-accent-danger">{error}</p>}
      <button
        type="button"
        onClick={submit}
        disabled={submitting || !canSubmit}
        className="inline-flex items-center gap-1.5 px-4 py-2 bg-purple-600 text-white rounded-md hover:bg-purple-700 text-sm font-medium disabled:opacity-50"
      >
        <Send className="w-4 h-4" />
        {submitting ? 'Submitting...' : 'Submit Review'}
      </button>
    </div>
  );
};

const RubricDataSummary = ({ data, criteria }) => {
  if (!data || typeof data !== 'object') return null;
  const byKey = Object.fromEntries((criteria || []).map((c) => [criterionKey(c), c]));
  return (
    <ul className="mt-1 text-xs text-text-secondary space-y-0.5">
      {Object.entries(data).map(([k, v]) => (
        <li key={k}>
          <span className="font-medium">{byKey[k]?.description || k}:</span> {v?.points ?? '—'}
          {byKey[k]?.points != null ? ` / ${byKey[k].points}` : ''}
          {v?.comments ? ` — ${v.comments}` : ''}
        </li>
      ))}
    </ul>
  );
};

// ReceivedPeerReviews shows a student the completed reviews of their work.
export const ReceivedPeerReviews = ({ courseId, assignmentId, pointsPossible, criteria = [] }) => {
  const [reviews, setReviews] = useState([]);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    let cancelled = false;
    gradingApi
      .listReceivedPeerReviews(courseId, assignmentId)
      .then((list) => { if (!cancelled) setReviews(Array.isArray(list) ? list : []); })
      .catch(() => { if (!cancelled) setReviews([]); })
      .finally(() => { if (!cancelled) setLoaded(true); });
    return () => { cancelled = true; };
  }, [courseId, assignmentId]);

  if (!loaded || reviews.length === 0) return null;
  return (
    <div className="bg-surface-0 rounded-lg shadow">
      <div className="p-4 border-b">
        <h3 className="font-semibold flex items-center gap-2">
          <MessageCircle className="w-5 h-5 text-purple-600" />
          Feedback from Peer Reviews ({reviews.length})
        </h3>
      </div>
      <ul className="divide-y">
        {reviews.map((r) => (
          <li key={r.id} className="p-4 text-sm">
            <div className="flex items-center justify-between">
              <span className="font-medium text-text-primary">{r.reviewer_name || 'Reviewer'}</span>
              {r.score != null && (
                <span className="inline-flex items-center gap-1 text-accent-success">
                  <Star className="w-4 h-4" /> {r.score}{pointsPossible ? ` / ${pointsPossible}` : ''}
                </span>
              )}
            </div>
            <RubricDataSummary data={r.rubric_data} criteria={criteria} />
            {r.comments && <p className="mt-1 text-text-secondary whitespace-pre-wrap">{r.comments}</p>}
          </li>
        ))}
      </ul>
    </div>
  );
};

// PeerReviewManager lets the instructor assign one review by hand and
// remove assigned reviews (beside the automatic assignment).
export const PeerReviewManager = ({ courseId, assignmentId, students = [], reviews = [], onChanged }) => {
  const [reviewer, setReviewer] = useState('');
  const [reviewee, setReviewee] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(null);

  const assign = async () => {
    if (!reviewer || !reviewee) return;
    if (reviewer === reviewee) {
      setError('A student cannot review their own work.');
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await gradingApi.assignPeerReview(courseId, assignmentId, Number(reviewer), Number(reviewee));
      setReviewer('');
      setReviewee('');
      await onChanged?.();
    } catch (err) {
      setError(err.status === 409 ? 'That review is already assigned.' : err.message);
    } finally {
      setBusy(false);
    }
  };

  const select = 'border border-border-strong rounded-md px-2 py-1.5 text-sm';
  return (
    <div className="space-y-2">
      <div className="flex flex-wrap items-center gap-2">
        <label htmlFor="pr-reviewer" className="text-sm text-text-secondary">Reviewer</label>
        <select id="pr-reviewer" value={reviewer} onChange={(e) => setReviewer(e.target.value)} className={select}>
          <option value="">Choose...</option>
          {students.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
        </select>
        <label htmlFor="pr-reviewee" className="text-sm text-text-secondary">reviews</label>
        <select id="pr-reviewee" value={reviewee} onChange={(e) => setReviewee(e.target.value)} className={select}>
          <option value="">Choose...</option>
          {students.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
        </select>
        <button
          type="button"
          onClick={assign}
          disabled={busy || !reviewer || !reviewee}
          className="inline-flex items-center gap-1.5 px-3 py-1.5 bg-surface-2 text-text-secondary rounded-md hover:bg-border-default text-sm disabled:opacity-50"
        >
          <UserPlus className="w-4 h-4" /> Assign
        </button>
      </div>
      {error && <p className="text-sm text-accent-danger">{error}</p>}
      {reviews.length > 0 && (
        <p className="text-xs text-text-tertiary">Use the trash icon on a review below to remove it.</p>
      )}
    </div>
  );
};

export const DeletePeerReviewButton = ({ courseId, assignmentId, review, onChanged }) => {
  const [busy, setBusy] = useState(false);
  return (
    <button
      type="button"
      disabled={busy}
      aria-label="Remove peer review"
      onClick={async () => {
        if (!window.confirm('Remove this peer review?')) return;
        setBusy(true);
        try {
          await gradingApi.deletePeerReview(courseId, assignmentId, review.id);
          await onChanged?.();
        } finally {
          setBusy(false);
        }
      }}
      className="text-text-disabled hover:text-accent-danger disabled:opacity-50"
    >
      <Trash2 className="w-4 h-4" />
    </button>
  );
};
