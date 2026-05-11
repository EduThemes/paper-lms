import React, { useState, useEffect, useCallback } from 'react';
import { CheckCircle, Clock, AlertCircle, Plus, Trash2, Pencil } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from '@/components/ui/dialog';

// DiscussionCheckpointsPanel
// ---------------------------------------------------------------------------
// Renders the Canvas-compatible checkpoint list for a graded discussion topic.
// Teacher view: add / edit / delete the checkpoint set (must include both
// reply_to_topic and reply_to_entry; points sum to the parent assignment).
// Student view: shows the user's progress (initial post done? replies X/Y).
//
// Wiring: this component expects a small `api` adapter passed in via props
// (so that the integration agent can choose between the existing services/api
// client and a slim inline fetch wrapper). The expected shape is:
//   api.list(topicId)               -> Promise<Checkpoint[]>
//   api.replace(topicId, payload)   -> Promise<Checkpoint[]>
//   api.update(topicId, id, body)   -> Promise<Checkpoint>
//   api.remove(topicId, id)         -> Promise<void>
//   api.progress(topicId, userId?)  -> Promise<Progress[]>
// ---------------------------------------------------------------------------

const formatDate = (iso) => {
  if (!iso) return 'No due date';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return 'No due date';
  return d.toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  });
};

const typeLabel = (t) =>
  t === 'reply_to_topic' ? 'Initial post' : t === 'reply_to_entry' ? 'Required replies' : t;

const StatusIcon = ({ status }) => {
  if (status === 'completed')
    return <CheckCircle className="h-5 w-5 text-accent-success" aria-label="Completed" />;
  if (status === 'in_progress')
    return <Clock className="h-5 w-5 text-amber-500" aria-label="In progress" />;
  return <AlertCircle className="h-5 w-5 text-slate-400" aria-label="Not started" />;
};

const emptyForm = {
  checkpoint_type: 'reply_to_topic',
  due_at: '',
  points_possible: 0,
  required_replies: 1,
};

const DiscussionCheckpointsPanel = ({
  topicId,
  isTeacher = false,
  userId,
  api,
}) => {
  const [checkpoints, setCheckpoints] = useState([]);
  const [progress, setProgress] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  const [editorOpen, setEditorOpen] = useState(false);
  const [editorRows, setEditorRows] = useState([
    { ...emptyForm, checkpoint_type: 'reply_to_topic', required_replies: 1 },
    { ...emptyForm, checkpoint_type: 'reply_to_entry', required_replies: 2 },
  ]);
  const [saving, setSaving] = useState(false);

  const load = useCallback(async () => {
    if (!api) return;
    setLoading(true);
    setError(null);
    try {
      const list = await api.list(topicId);
      setCheckpoints(list || []);
      if (!isTeacher && userId) {
        const prog = await api.progress(topicId, userId);
        setProgress(prog || []);
      }
    } catch (err) {
      setError(err?.message || 'Failed to load checkpoints');
    } finally {
      setLoading(false);
    }
  }, [api, topicId, isTeacher, userId]);

  useEffect(() => {
    load();
  }, [load]);

  const openEditor = () => {
    if (checkpoints.length >= 2) {
      setEditorRows(
        checkpoints.map((c) => ({
          checkpoint_type: c.checkpoint_type,
          due_at: c.due_at ? c.due_at.slice(0, 16) : '',
          points_possible: c.points_possible || 0,
          required_replies: c.required_replies || 1,
        })),
      );
    }
    setEditorOpen(true);
  };

  const updateRow = (idx, patch) =>
    setEditorRows((rows) => rows.map((r, i) => (i === idx ? { ...r, ...patch } : r)));

  const handleSave = async () => {
    setSaving(true);
    setError(null);
    try {
      const payload = {
        checkpoints: editorRows.map((r) => ({
          checkpoint_type: r.checkpoint_type,
          due_at: r.due_at ? new Date(r.due_at).toISOString() : null,
          points_possible: Number(r.points_possible) || 0,
          required_replies: Number(r.required_replies) || 1,
        })),
      };
      await api.replace(topicId, payload);
      setEditorOpen(false);
      await load();
    } catch (err) {
      setError(err?.message || 'Failed to save checkpoints');
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (id) => {
    try {
      await api.remove(topicId, id);
      await load();
    } catch (err) {
      setError(err?.message || 'Failed to delete checkpoint');
    }
  };

  if (loading) {
    return (
      <Card>
        <CardContent className="flex items-center justify-center p-8">
          <svg
            className="h-6 w-6 animate-spin text-slate-400"
            viewBox="0 0 24 24"
            fill="none"
            aria-label="Loading"
          >
            <circle cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" opacity="0.25" />
            <path d="M22 12a10 10 0 0 1-10 10" stroke="currentColor" strokeWidth="4" />
          </svg>
        </CardContent>
      </Card>
    );
  }

  const progressByCheckpoint = new Map(
    (progress || []).map((p) => [p.checkpoint?.id, p]),
  );

  return (
    <Card>
      <CardHeader className="flex flex-row items-start justify-between gap-2">
        <div>
          <CardTitle className="text-lg">Checkpoints</CardTitle>
          <CardDescription>
            Multi-deadline participation: post by one date, reply by another.
          </CardDescription>
        </div>
        {isTeacher && (
          <Button onClick={openEditor} size="sm" variant="outline">
            <Pencil className="mr-1 h-4 w-4" />
            {checkpoints.length ? 'Edit' : 'Add checkpoints'}
          </Button>
        )}
      </CardHeader>
      <CardContent className="space-y-3">
        {error && (
          <div className="flex items-center gap-2 rounded border border-accent-danger/30 bg-accent-danger/10 p-2 text-sm text-accent-danger">
            <AlertCircle className="h-4 w-4" />
            <span>{error}</span>
            <Button onClick={load} variant="ghost" size="sm" className="ml-auto">
              Try again
            </Button>
          </div>
        )}

        {checkpoints.length === 0 && !error && (
          <p className="text-sm text-slate-500">
            No checkpoints configured for this discussion.
          </p>
        )}

        {checkpoints.map((cp) => {
          const prog = progressByCheckpoint.get(cp.id);
          const status = prog?.status || 'not_started';
          const replyCount = prog?.reply_count ?? 0;
          const required =
            prog?.required ??
            (cp.checkpoint_type === 'reply_to_topic' ? 1 : cp.required_replies || 1);

          return (
            <div
              key={cp.id}
              className="flex items-center justify-between gap-3 rounded-md border p-3"
            >
              <div className="flex items-center gap-3">
                {!isTeacher && <StatusIcon status={status} />}
                <div>
                  <div className="text-sm font-medium">{typeLabel(cp.checkpoint_type)}</div>
                  <div className="text-xs text-slate-500">
                    Due {formatDate(cp.due_at)} - {cp.points_possible} pts
                    {cp.checkpoint_type === 'reply_to_entry' && (
                      <> - {cp.required_replies} replies required</>
                    )}
                  </div>
                  {!isTeacher && (
                    <div className="mt-1 text-xs text-slate-600">
                      {cp.checkpoint_type === 'reply_to_topic'
                        ? status === 'completed'
                          ? 'Initial post: done'
                          : 'Initial post: not yet'
                        : `Replies: ${replyCount}/${required}`}
                    </div>
                  )}
                </div>
              </div>
              {isTeacher && (
                <Button
                  onClick={() => handleDelete(cp.id)}
                  variant="ghost"
                  size="sm"
                  aria-label={`Delete ${typeLabel(cp.checkpoint_type)} checkpoint`}
                >
                  <Trash2 className="h-4 w-4 text-accent-danger" />
                </Button>
              )}
            </div>
          );
        })}
      </CardContent>

      <Dialog open={editorOpen} onOpenChange={setEditorOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Configure checkpoints</DialogTitle>
            <DialogDescription>
              The checkpoint set must include one initial-post checkpoint and one
              required-replies checkpoint. Their points must sum to the parent
              assignment&apos;s points possible.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4">
            {editorRows.map((row, idx) => (
              <div key={idx} className="rounded-md border p-3 space-y-2">
                <div className="flex items-center justify-between">
                  <label className="text-sm font-medium">
                    {typeLabel(row.checkpoint_type)}
                  </label>
                  <select
                    value={row.checkpoint_type}
                    onChange={(e) => updateRow(idx, { checkpoint_type: e.target.value })}
                    className="rounded border px-2 py-1 text-sm"
                  >
                    <option value="reply_to_topic">Initial post</option>
                    <option value="reply_to_entry">Required replies</option>
                  </select>
                </div>
                <div className="grid grid-cols-2 gap-2">
                  <label className="text-xs">
                    Due at
                    <input
                      type="datetime-local"
                      value={row.due_at}
                      onChange={(e) => updateRow(idx, { due_at: e.target.value })}
                      className="mt-1 w-full rounded border px-2 py-1 text-sm"
                    />
                  </label>
                  <label className="text-xs">
                    Points
                    <input
                      type="number"
                      min="0"
                      step="0.5"
                      value={row.points_possible}
                      onChange={(e) => updateRow(idx, { points_possible: e.target.value })}
                      className="mt-1 w-full rounded border px-2 py-1 text-sm"
                    />
                  </label>
                  {row.checkpoint_type === 'reply_to_entry' && (
                    <label className="text-xs col-span-2">
                      Required replies
                      <input
                        type="number"
                        min="1"
                        value={row.required_replies}
                        onChange={(e) => updateRow(idx, { required_replies: e.target.value })}
                        className="mt-1 w-full rounded border px-2 py-1 text-sm"
                      />
                    </label>
                  )}
                </div>
              </div>
            ))}
            {editorRows.length < 2 && (
              <Button
                variant="outline"
                size="sm"
                onClick={() =>
                  setEditorRows((rows) => [
                    ...rows,
                    { ...emptyForm, checkpoint_type: 'reply_to_entry', required_replies: 2 },
                  ])
                }
              >
                <Plus className="mr-1 h-4 w-4" />
                Add checkpoint
              </Button>
            )}
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setEditorOpen(false)} disabled={saving}>
              Cancel
            </Button>
            <Button onClick={handleSave} disabled={saving}>
              {saving ? 'Saving...' : 'Save checkpoints'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  );
};

export default DiscussionCheckpointsPanel;
