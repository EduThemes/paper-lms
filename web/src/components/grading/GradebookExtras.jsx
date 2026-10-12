import React, { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { Filter, User, EyeOff, Columns, Plus } from 'lucide-react';

// Gradebook additions: the filter bar (section / grading period /
// module), the "hide totals from students" switch, the link to the
// individual view, and the custom (notes) column cells and headers.

export const GradebookFilterBar = ({
  courseId,
  sections = [],
  periods = [],
  modules = [],
  filters,
  onFiltersChange,
  hideTotals,
  onToggleHideTotals,
  onAddCustomColumn,
}) => {
  const select = 'border border-border-strong rounded-md px-2 py-1 text-sm bg-surface-0';
  const set = (patch) => onFiltersChange({ ...filters, ...patch });
  const active = filters.sectionId || filters.gradingPeriodId || filters.moduleId;
  return (
    <div className="mb-3 flex flex-wrap items-center gap-3 text-sm" role="group" aria-label="Gradebook filters">
      <span className="inline-flex items-center gap-1 text-text-secondary"><Filter className="w-4 h-4" /> Filters</span>
      {sections.length > 0 && (
        <select aria-label="Section" value={filters.sectionId || ''} onChange={(e) => set({ sectionId: e.target.value })} className={select}>
          <option value="">All sections</option>
          {sections.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
        </select>
      )}
      {periods.length > 0 && (
        <select aria-label="Grading period" value={filters.gradingPeriodId || ''} onChange={(e) => set({ gradingPeriodId: e.target.value })} className={select}>
          <option value="">All grading periods</option>
          {periods.map((p) => <option key={p.id} value={p.id}>{p.title}</option>)}
        </select>
      )}
      {modules.length > 0 && (
        <select aria-label="Module" value={filters.moduleId || ''} onChange={(e) => set({ moduleId: e.target.value })} className={select}>
          <option value="">All modules</option>
          {modules.map((m) => <option key={m.id} value={m.id}>{m.name}</option>)}
        </select>
      )}
      {active && (
        <button type="button" onClick={() => onFiltersChange({})} className="text-brand-600 hover:underline text-xs">Clear filters</button>
      )}
      <span className="flex-1" />
      {onAddCustomColumn && (
        <button type="button" onClick={onAddCustomColumn} className="inline-flex items-center gap-1 text-text-secondary hover:text-text-primary">
          <Columns className="w-4 h-4" /> <Plus className="w-3 h-3" /> Notes column
        </button>
      )}
      <label className="inline-flex items-center gap-1 text-text-secondary" title="Students won't see their course total or final grade">
        <input type="checkbox" checked={!!hideTotals} onChange={(e) => onToggleHideTotals(e.target.checked)} />
        <EyeOff className="w-4 h-4" /> Hide totals from students
      </label>
      <Link to={`/courses/${courseId}/gradebook/individual`} className="inline-flex items-center gap-1 text-brand-600 hover:underline">
        <User className="w-4 h-4" /> Individual view
      </Link>
    </div>
  );
};

// CustomColumnCell edits one student's text in a custom column.
export const CustomColumnCell = ({ style, column, value, onSave, rowIndex, columnIndex, studentName }) => {
  const [text, setText] = useState(value || '');
  useEffect(() => { setText(value || ''); }, [value]);
  const commit = () => {
    if ((value || '') !== text) onSave(text);
  };
  return (
    <div
      role="gridcell"
      aria-rowindex={rowIndex + 2}
      aria-colindex={columnIndex + 3}
      data-cell-row={rowIndex}
      data-cell-col={columnIndex}
      style={style}
      className="flex items-center border-b border-r border-border-subtle px-1 bg-amber-50/40"
    >
      {column.read_only ? (
        <span className="text-xs text-text-secondary truncate px-1" title={value}>{value}</span>
      ) : (
        <input
          type="text"
          value={text}
          onChange={(e) => setText(e.target.value)}
          onBlur={commit}
          onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); e.currentTarget.blur(); } }}
          aria-label={`${column.title} for ${studentName}`}
          className="w-full text-xs bg-transparent border border-transparent hover:border-border-default focus:border-brand-500 rounded px-1 py-0.5"
        />
      )}
    </div>
  );
};

export const CustomColumnHeader = ({ column, width, height, onHide }) => (
  <div
    role="columnheader"
    className="flex flex-col items-center justify-center px-2 py-2 text-xs border-r border-border-default bg-amber-50 relative"
    style={{ width, minWidth: width, height }}
  >
    <span className="font-medium text-text-secondary truncate w-full text-center" title={column.title}>{column.title}</span>
    <span className="text-text-tertiary">{column.teacher_notes ? 'Notes' : 'Custom'}</span>
    {onHide && (
      <button type="button" onClick={() => onHide(column)} className="text-[10px] text-text-tertiary hover:underline mt-0.5">Hide</button>
    )}
  </div>
);
