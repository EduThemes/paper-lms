import React, { useCallback, useEffect, useState } from 'react';
import { Puzzle, Search, Trash2 } from 'lucide-react';
import { ltiApi, ltiLaunchUrl } from '../../services/api_lti';
import LtiLaunchFrame from './LtiLaunchFrame';
import LtiDeepLinkModal from './LtiDeepLinkModal';

export function assignmentUsesExternalTool(assignment) {
  const types = Array.isArray(assignment?.submission_types)
    ? assignment.submission_types
    : String(assignment?.submission_types || '').split(',').map((t) => t.trim());
  return types.includes('external_tool');
}

/**
 * External tool section of an assignment (G131 assignment_selection, G128).
 * Teachers pick the tool (or find content in it through deep linking);
 * the tool then gets the assignment's AGS line item. Everyone with access
 * launches the tool in place; a student's launch is graded.
 *
 * Plain fetches (not react-query): AssignmentPage predates the query
 * client in its tests.
 */
export default function AssignmentExternalToolPanel({ courseId, assignment, isTeacher, onChanged }) {
  const assignmentId = assignment?.id;
  const [config, setConfig] = useState(null);
  const [loaded, setLoaded] = useState(false);
  const [tools, setTools] = useState([]);
  const [selectionTools, setSelectionTools] = useState([]);
  const [editing, setEditing] = useState(false);
  const [toolId, setToolId] = useState('');
  const [url, setUrl] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(null);
  const [picking, setPicking] = useState(null);

  const load = useCallback(async () => {
    try {
      setConfig(await ltiApi.getAssignmentTool(courseId, assignmentId));
    } catch {
      // 404: the assignment has no tool. Any other failure also leaves
      // the panel in its "no tool" state.
      setConfig(null);
    } finally {
      setLoaded(true);
    }
  }, [courseId, assignmentId]);

  useEffect(() => {
    if (courseId && assignmentId) load();
  }, [courseId, assignmentId, load]);

  useEffect(() => {
    if (!isTeacher || !courseId) return;
    ltiApi.courseTools(courseId).then(setTools).catch(() => setTools([]));
    ltiApi.courseTools(courseId, 'assignment_selection').then(setSelectionTools).catch(() => setSelectionTools([]));
  }, [isTeacher, courseId]);

  const uses = assignmentUsesExternalTool(assignment);
  if (!loaded || (!config && !uses && !isTeacher)) return null;

  const save = async (e) => {
    e.preventDefault();
    if (!toolId) return;
    setBusy(true);
    setError(null);
    try {
      setConfig(await ltiApi.setAssignmentTool(courseId, assignmentId, { tool_id: Number(toolId), url: url.trim() }));
      setEditing(false);
      onChanged?.();
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  };

  const remove = async () => {
    if (!window.confirm('Remove the external tool from this assignment?')) return;
    setBusy(true);
    try {
      await ltiApi.removeAssignmentTool(courseId, assignmentId);
      setConfig(null);
      onChanged?.();
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  };

  const showForm = isTeacher && (editing || (!config && uses));

  return (
    <section className="mb-6 rounded-lg bg-surface-0 p-4 shadow" aria-labelledby="assignment-tool-heading">
      <div className="mb-3 flex items-center justify-between">
        <h3 id="assignment-tool-heading" className="flex items-center gap-2 text-sm font-semibold text-text-secondary">
          <Puzzle className="h-4 w-4 text-purple-500" />
          External tool{config?.tool_name ? `: ${config.tool_name}` : ''}
        </h3>
        {isTeacher && !showForm && (
          <div className="flex items-center gap-2 text-xs">
            <button type="button" className="text-brand-600 hover:underline" onClick={() => setEditing(true)}>
              {config ? 'Change tool' : 'Use an external tool'}
            </button>
            {config && (
              <button type="button" aria-label="Remove external tool" className="text-accent-danger" disabled={busy} onClick={remove}>
                <Trash2 className="h-4 w-4" />
              </button>
            )}
          </div>
        )}
      </div>

      {error && <p role="alert" className="mb-2 text-sm text-accent-danger">{error}</p>}

      {showForm && (
        <form onSubmit={save} className="mb-3 space-y-2">
          <div className="flex flex-wrap gap-2">
            <select aria-label="Tool" className="rounded-md border border-border-strong px-3 py-2 text-sm" value={toolId} onChange={(e) => setToolId(e.target.value)} required>
              <option value="">Choose a tool…</option>
              {tools.map((t) => (
                <option key={t.id} value={t.id}>{t.name}</option>
              ))}
            </select>
            <input
              aria-label="Launch URL (optional)"
              type="url"
              className="min-w-64 flex-1 rounded-md border border-border-strong px-3 py-2 text-sm"
              placeholder="Launch URL (optional, defaults to the tool's)"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
            />
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <button type="submit" disabled={busy || !toolId} className="rounded-md bg-brand-600 px-3 py-1.5 text-sm text-white hover:bg-brand-700 disabled:opacity-50">
              {busy ? 'Saving…' : 'Save'}
            </button>
            {selectionTools.map((t) => (
              <button
                key={t.id}
                type="button"
                onClick={() => setPicking(t)}
                className="inline-flex items-center gap-1 rounded-md border border-border-strong px-3 py-1.5 text-sm hover:bg-surface-1"
              >
                <Search className="h-3.5 w-3.5" /> Find content in {t.text || t.name}
              </button>
            ))}
            {editing && (
              <button type="button" className="px-2 text-sm text-text-tertiary" onClick={() => setEditing(false)}>Cancel</button>
            )}
          </div>
        </form>
      )}

      {config && !showForm && (
        <LtiLaunchFrame
          src={ltiLaunchUrl(courseId, config.tool_id, { assignment_id: assignmentId })}
          title={config.title || config.tool_name || 'External tool'}
          height={640}
        />
      )}

      <LtiDeepLinkModal
        open={Boolean(picking)}
        title={picking ? `Select content: ${picking.text || picking.name}` : ''}
        src={picking ? ltiLaunchUrl(courseId, picking.id, { placement: 'assignment_selection', deep_link: 'assignment', assignment_id: assignmentId }) : ''}
        width={picking?.selection_width}
        height={picking?.selection_height}
        onClose={() => setPicking(null)}
        onResult={({ error: err }) => {
          setPicking(null);
          if (err) {
            setError(err);
            return;
          }
          setEditing(false);
          load();
          onChanged?.();
        }}
      />
    </section>
  );
}
