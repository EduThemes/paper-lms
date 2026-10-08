import React, { useEffect, useState } from 'react';
import { Search } from 'lucide-react';
import { ltiApi, ltiLaunchUrl } from '../../services/api_lti';
import LtiDeepLinkModal from './LtiDeepLinkModal';

const fieldCls = 'border border-border-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500';

/**
 * Extra fields of the module "add item" form for an External Tool item
 * (G131): pick an installed tool and optional launch URL, or let a tool
 * with a link_selection / module_index_menu placement add content
 * through deep linking (the server creates the module items).
 *
 * value: the form state ({ tool_id, external_url, new_tab, ... });
 * onChange(next); onDeepLinked() after a deep link added items.
 */
export default function ModuleExternalToolFields({ courseId, moduleId, value, onChange, onDeepLinked }) {
  const [tools, setTools] = useState([]);
  const [selectionTools, setSelectionTools] = useState([]);
  const [picking, setPicking] = useState(null);
  const [error, setError] = useState(null);

  useEffect(() => {
    let live = true;
    ltiApi.courseTools(courseId).then((t) => live && setTools(t)).catch(() => {});
    Promise.all([
      ltiApi.courseTools(courseId, 'link_selection').catch(() => []),
      ltiApi.courseTools(courseId, 'module_index_menu').catch(() => []),
    ]).then(([a, b]) => {
      if (!live) return;
      const seen = new Set();
      setSelectionTools([...a, ...b].filter((t) => (seen.has(t.id) ? false : seen.add(t.id))));
    });
    return () => {
      live = false;
    };
  }, [courseId]);

  return (
    <div className="space-y-2">
      <div className="flex flex-wrap items-center gap-3">
        <select
          aria-label="External tool"
          className={fieldCls}
          value={value.tool_id || ''}
          onChange={(e) => onChange({ ...value, tool_id: e.target.value })}
        >
          <option value="">Choose a tool…</option>
          {tools.map((t) => (
            <option key={t.id} value={t.id}>{t.name}</option>
          ))}
        </select>
        <input
          type="url"
          aria-label="Launch URL (optional)"
          placeholder="Launch URL (optional)"
          className={`flex-1 ${fieldCls}`}
          value={value.external_url || ''}
          onChange={(e) => onChange({ ...value, external_url: e.target.value })}
        />
        <label className="flex items-center gap-2 whitespace-nowrap text-sm text-text-secondary">
          <input
            type="checkbox"
            checked={Boolean(value.new_tab)}
            onChange={(e) => onChange({ ...value, new_tab: e.target.checked })}
            className="rounded border-border-strong"
          />
          New tab
        </label>
      </div>
      {selectionTools.length > 0 && (
        <div className="flex flex-wrap gap-2">
          {selectionTools.map((t) => (
            <button
              key={t.id}
              type="button"
              onClick={() => {
                setError(null);
                setPicking(t);
              }}
              className="inline-flex items-center gap-1 rounded-md border border-border-strong px-3 py-1.5 text-sm hover:bg-surface-1"
            >
              <Search className="h-3.5 w-3.5" /> Find content in {t.text || t.name}
            </button>
          ))}
        </div>
      )}
      {error && <p role="alert" className="text-sm text-accent-danger">{error}</p>}
      <LtiDeepLinkModal
        open={Boolean(picking)}
        title={picking ? `Select content: ${picking.text || picking.name}` : ''}
        src={picking ? ltiLaunchUrl(courseId, picking.id, { placement: picking.placement || 'link_selection', deep_link: 'module', module_id: moduleId }) : ''}
        width={picking?.selection_width}
        height={picking?.selection_height}
        onClose={() => setPicking(null)}
        onResult={({ error: err }) => {
          setPicking(null);
          if (err) setError(err);
          else onDeepLinked?.();
        }}
      />
    </div>
  );
}
