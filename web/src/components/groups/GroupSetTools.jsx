import React, { useRef, useState } from 'react';
import { Shuffle, Upload, Download, UserX, Plus } from 'lucide-react';
import {
  useUnassignedStudents,
  useGroupSetMutation,
  assignUnassignedMembers,
  createGroupsInSet,
  importGroupSetCSV,
  downloadGroupSetCSV,
  importSummary,
} from '../../services/api_groups_conferences';

// GroupSetTools is the instructor toolbar of one group set (G100): the
// unassigned students, auto-assign (optionally keeping each group to one
// section), bulk group creation, and CSV import / export.
export default function GroupSetTools({ category, onChanged }) {
  const unassignedQuery = useUnassignedStudents(category.id);
  const [showUnassigned, setShowUnassigned] = useState(false);
  const [bySection, setBySection] = useState(false);
  const [groupCount, setGroupCount] = useState('');
  const [message, setMessage] = useState(null);
  const [error, setError] = useState(null);
  const fileRef = useRef(null);

  const report = (text) => { setError(null); setMessage(text); };
  const fail = (err) => { setMessage(null); setError(err?.message || 'Something went wrong'); };

  const assign = useGroupSetMutation(
    category.id,
    () => assignUnassignedMembers(category.id, { groupBySection: bySection }),
    (data) => { report(`Assigned ${data?.assigned ?? 0} student${data?.assigned === 1 ? '' : 's'}.`); onChanged?.(); },
  );
  const create = useGroupSetMutation(
    category.id,
    (count) => createGroupsInSet(category.id, { count }),
    (data) => { report(`Created ${data?.groups?.length ?? 0} group${data?.groups?.length === 1 ? '' : 's'}.`); setGroupCount(''); onChanged?.(); },
  );
  const importer = useGroupSetMutation(
    category.id,
    (file) => importGroupSetCSV(category.id, file),
    (data) => { report(importSummary(data)); onChanged?.(); },
  );

  const unassigned = unassignedQuery.data || [];
  const busy = assign.isPending || create.isPending || importer.isPending;

  const handleImport = (e) => {
    const file = e.target.files?.[0];
    e.target.value = '';
    if (file) importer.mutate(file, { onError: fail });
  };

  const handleExport = async () => {
    try {
      const safe = (category.name || 'groups').replace(/[^A-Za-z0-9._-]+/g, '_');
      await downloadGroupSetCSV(category.id, `${safe}.csv`);
    } catch (err) {
      fail(err);
    }
  };

  return (
    <div className="px-4 py-3 border-b bg-surface-1 text-sm space-y-2" data-testid={`group-set-tools-${category.id}`}>
      <div className="flex flex-wrap items-center gap-2">
        <button
          type="button"
          onClick={() => setShowUnassigned((v) => !v)}
          className="inline-flex items-center gap-1 text-text-secondary hover:text-text-primary"
          aria-expanded={showUnassigned}
        >
          <UserX className="w-4 h-4" />
          {unassignedQuery.isLoading ? 'Loading students…' : `${unassigned.length} unassigned`}
        </button>
        <span className="text-border-strong" aria-hidden="true">|</span>
        <button
          type="button"
          onClick={() => assign.mutate(undefined, { onError: fail })}
          disabled={busy || unassigned.length === 0}
          className="inline-flex items-center gap-1 text-brand-600 hover:text-brand-800 disabled:opacity-50"
        >
          <Shuffle className="w-4 h-4" />
          Auto-assign
        </button>
        <label className="inline-flex items-center gap-1 text-text-secondary">
          <input type="checkbox" checked={bySection} onChange={(e) => setBySection(e.target.checked)} />
          Keep each group to one section
        </label>
        <span className="text-border-strong" aria-hidden="true">|</span>
        <form
          className="inline-flex items-center gap-1"
          onSubmit={(e) => {
            e.preventDefault();
            const n = parseInt(groupCount, 10);
            if (n > 0) create.mutate(n, { onError: fail });
          }}
        >
          <label htmlFor={`create-groups-${category.id}`} className="text-text-secondary">Create</label>
          <input
            id={`create-groups-${category.id}`}
            type="number"
            min="1"
            max="200"
            value={groupCount}
            onChange={(e) => setGroupCount(e.target.value)}
            className="w-16 border border-border-strong rounded px-1 py-0.5"
          />
          <button type="submit" disabled={busy || !(parseInt(groupCount, 10) > 0)} className="inline-flex items-center gap-1 text-brand-600 hover:text-brand-800 disabled:opacity-50">
            <Plus className="w-4 h-4" />
            groups
          </button>
        </form>
        <span className="text-border-strong" aria-hidden="true">|</span>
        <button
          type="button"
          onClick={() => fileRef.current?.click()}
          disabled={busy}
          className="inline-flex items-center gap-1 text-brand-600 hover:text-brand-800 disabled:opacity-50"
        >
          <Upload className="w-4 h-4" />
          Import CSV
        </button>
        <input
          ref={fileRef}
          type="file"
          accept=".csv,text/csv"
          className="hidden"
          aria-label={`Import CSV for ${category.name}`}
          onChange={handleImport}
        />
        <button type="button" onClick={handleExport} className="inline-flex items-center gap-1 text-brand-600 hover:text-brand-800">
          <Download className="w-4 h-4" />
          Export CSV
        </button>
      </div>

      {message && <p role="status" className="text-accent-success">{message}</p>}
      {error && <p role="alert" className="text-accent-danger">{error}</p>}
      {importer.data?.skipped?.length > 0 && (
        <ul className="text-xs text-text-tertiary list-disc pl-5">
          {importer.data.skipped.slice(0, 10).map((s) => (
            <li key={`${s.row}-${s.reason}`}>Row {s.row}: {s.reason}</li>
          ))}
        </ul>
      )}

      {showUnassigned && (
        unassigned.length === 0 ? (
          <p className="text-text-tertiary">Every student is in a group.</p>
        ) : (
          <ul className="flex flex-wrap gap-2" aria-label="Unassigned students">
            {unassigned.map((u) => (
              <li key={u.id} className="bg-surface-0 border border-border-subtle rounded-full px-2 py-0.5 text-xs text-text-secondary">
                {u.name}
              </li>
            ))}
          </ul>
        )
      )}
      <p className="text-xs text-text-tertiary">
        CSV columns: a student column (<code>login_id</code>, <code>canvas_user_id</code>, <code>user_id</code> or <code>email</code>) and a group column (<code>group_name</code> or <code>canvas_group_id</code>).
      </p>
    </div>
  );
}
