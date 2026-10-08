import React from 'react';

/**
 * ActivityChart — vertical bars of page views and participations per
 * period (week or day). Each period shows two bars side by side; the data
 * is also rendered as a visually hidden table for screen readers.
 *
 * Props:
 *   rows   — [{ label, views, participations }]
 *   title  — caption for the hidden table
 *   height — chart height in px (default 160)
 */
export default function ActivityChart({ rows, title = 'Activity', height = 160 }) {
  const data = Array.isArray(rows) ? rows : [];
  const max = Math.max(1, ...data.map((r) => Math.max(r.views || 0, r.participations || 0)));
  const showEvery = data.length > 16 ? Math.ceil(data.length / 8) : 1;

  return (
    <div>
      <div className="flex items-center gap-4 text-xs text-text-secondary mb-2" aria-hidden="true">
        <span className="flex items-center gap-1.5"><span className="inline-block w-3 h-3 rounded-sm bg-indigo-500" /> Page views</span>
        <span className="flex items-center gap-1.5"><span className="inline-block w-3 h-3 rounded-sm bg-amber-500" /> Participations</span>
      </div>
      <div className="flex items-end gap-1 overflow-x-auto pb-1" style={{ height: height + 24 }} aria-hidden="true">
        {data.map((r, i) => (
          <div key={r.label} className="flex flex-col items-center flex-1 min-w-[18px]" title={`${r.label}: ${r.views || 0} views, ${r.participations || 0} participations`}>
            <div className="flex items-end gap-px w-full justify-center" style={{ height }}>
              <div className="bg-indigo-500 rounded-t w-1/2 max-w-[14px]" style={{ height: `${((r.views || 0) / max) * 100}%`, minHeight: r.views ? 2 : 0 }} />
              <div className="bg-amber-500 rounded-t w-1/2 max-w-[14px]" style={{ height: `${((r.participations || 0) / max) * 100}%`, minHeight: r.participations ? 2 : 0 }} />
            </div>
            <span className="text-[10px] text-text-tertiary mt-1 whitespace-nowrap h-4">
              {i % showEvery === 0 ? r.label.slice(5) : ''}
            </span>
          </div>
        ))}
      </div>
      <table className="sr-only">
        <caption>{title}</caption>
        <thead>
          <tr><th scope="col">Period</th><th scope="col">Page views</th><th scope="col">Participations</th></tr>
        </thead>
        <tbody>
          {data.map((r) => (
            <tr key={r.label}><th scope="row">{r.label}</th><td>{r.views || 0}</td><td>{r.participations || 0}</td></tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
