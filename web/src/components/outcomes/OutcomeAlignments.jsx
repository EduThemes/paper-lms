import React, { useState } from 'react';
import { Link } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Link2, X } from 'lucide-react';
import { api } from '../../services/api';
import { outcomesApi } from '../../services/api_learner_outcomes';

/**
 * Aligned assignments of one outcome (G151). Grading an aligned assignment
 * records an outcome result, which feeds the mastery views. Staff can align
 * and unalign here; everyone else sees the list.
 */
export default function OutcomeAlignments({ courseId, outcomeId, canManage }) {
  const qc = useQueryClient();
  const [pick, setPick] = useState('');
  const key = ['outcomes', String(courseId), 'alignments'];
  const alignments = useQuery({ queryKey: key, queryFn: () => outcomesApi.alignments(courseId) });
  const assignments = useQuery({
    queryKey: ['outcomes', String(courseId), 'assignments'],
    queryFn: async () => (await api.getAssignments(courseId, 1, 500))?.data || [],
  });
  const align = useMutation({
    mutationFn: (assignmentId) => outcomesApi.align(courseId, outcomeId, Number(assignmentId)),
    onSuccess: () => { setPick(''); qc.invalidateQueries({ queryKey: key }); },
  });
  const unalign = useMutation({
    mutationFn: (alignmentId) => outcomesApi.unalign(courseId, alignmentId),
    onSuccess: () => qc.invalidateQueries({ queryKey: key }),
  });

  const mine = (alignments.data || []).filter((a) => Number(a.learning_outcome_id) === Number(outcomeId));
  const byId = new Map((assignments.data || []).map((a) => [Number(a.id), a]));
  const alignedIds = new Set(mine.map((a) => Number(a.assignment_id)));
  const available = (assignments.data || []).filter((a) => !alignedIds.has(Number(a.id)));
  const err = align.error || unalign.error;

  return (
    <div>
      <p className="text-xs font-medium text-text-tertiary uppercase mb-2 flex items-center gap-1">
        <Link2 className="w-3.5 h-3.5" aria-hidden="true" /> Aligned assignments
      </p>
      {alignments.isLoading ? (
        <p className="text-sm text-text-tertiary">Loading…</p>
      ) : mine.length === 0 ? (
        <p className="text-sm text-text-tertiary">Not aligned to any assignment yet.</p>
      ) : (
        <ul className="space-y-1">
          {mine.map((a) => (
            <li key={a.id} className="flex items-center justify-between text-sm bg-surface-1 rounded px-2 py-1">
              <Link to={`/courses/${courseId}/assignments/${a.assignment_id}`} className="text-brand-600 hover:underline truncate">
                {byId.get(Number(a.assignment_id))?.name || `Assignment ${a.assignment_id}`}
              </Link>
              {canManage && (
                <button type="button" aria-label="Remove alignment" onClick={() => unalign.mutate(a.id)} className="p-0.5 text-text-tertiary hover:text-accent-danger">
                  <X className="w-3.5 h-3.5" />
                </button>
              )}
            </li>
          ))}
        </ul>
      )}
      {canManage && available.length > 0 && (
        <div className="flex gap-2 mt-2">
          <select aria-label="Assignment to align" value={pick} onChange={(e) => setPick(e.target.value)} className="flex-1 border border-border-strong rounded px-2 py-1 text-sm">
            <option value="">Align an assignment…</option>
            {available.map((a) => <option key={a.id} value={a.id}>{a.name}</option>)}
          </select>
          <button type="button" disabled={!pick || align.isPending} onClick={() => align.mutate(pick)} className="px-3 py-1 text-sm rounded bg-brand-600 text-on-brand disabled:opacity-50">Align</button>
        </div>
      )}
      {err && <p role="alert" className="text-xs text-accent-danger mt-1">{err.message}</p>}
    </div>
  );
}
