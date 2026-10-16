import { useId, useState } from 'react';
import PropTypes from 'prop-types';
import { ChevronDown, ChevronRight, Code2 } from 'lucide-react';

// "Advanced → Page CSS" for an ordinary wiki page (teachers, when the
// account allows hand-written CSS: course.custom_css_enabled). The CSS is
// saved as wiki_page.css; the server sanitizes it and scopes it to this page
// (docs/page-designer/README.md, "Page stylesheet rules").

const EXAMPLE = `:root { --accent: #0f766e; }
h2 { color: var(--accent); border-bottom: 2px solid currentColor; }
.callout { padding: 1rem; border-radius: .75rem; background: #f0fdfa; }
@media (prefers-color-scheme: dark) {
  :root { --accent: #5eead4; }
  .callout { background: #134e4a; }
}`;

export default function PageCssPanel({ value, onChange, defaultOpen = false }) {
  const [open, setOpen] = useState(defaultOpen || Boolean(value));
  const id = useId();
  const panelId = `${id}-panel`;
  const fieldId = `${id}-css`;
  const helpId = `${id}-help`;

  return (
    <div className="rounded-md border border-border-default">
      <button
        type="button"
        aria-expanded={open}
        aria-controls={panelId}
        onClick={() => setOpen((o) => !o)}
        className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm font-medium text-text-secondary hover:text-text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded-md"
      >
        {open ? <ChevronDown className="h-4 w-4" aria-hidden="true" /> : <ChevronRight className="h-4 w-4" aria-hidden="true" />}
        Advanced
        {!open && value && <span className="ml-1 text-xs font-normal text-text-tertiary">(page CSS set)</span>}
      </button>
      {open && (
        <div id={panelId} className="space-y-2 border-t border-border-default px-3 pb-3 pt-3">
          <label htmlFor={fieldId} className="flex items-center gap-1.5 text-sm font-medium text-text-secondary">
            <Code2 className="h-4 w-4" aria-hidden="true" /> Page CSS
          </label>
          <textarea
            id={fieldId}
            value={value}
            onChange={(e) => onChange(e.target.value)}
            aria-describedby={helpId}
            spellCheck={false}
            autoCapitalize="off"
            autoCorrect="off"
            rows={10}
            placeholder={EXAMPLE}
            className="w-full rounded-md border border-border-strong bg-surface-0 px-3 py-2 font-mono text-xs leading-relaxed text-text-primary"
          />
          <div id={helpId} className="space-y-1 text-xs text-text-tertiary">
            <p>
              Styles apply to this page only. <code className="font-mono">:root</code> means the page itself, so
              custom properties set there are available everywhere on it.
            </p>
            <p>
              <code className="font-mono">@media (prefers-color-scheme: dark)</code> follows Paper&apos;s dark mode
              toggle, not just the device setting.
            </p>
            <p>
              Saved CSS is cleaned: <code className="font-mono">@import</code>, web fonts, scripts and images from
              other sites are removed. Fixed positioning stays inside the page.
            </p>
          </div>
        </div>
      )}
    </div>
  );
}
PageCssPanel.propTypes = {
  value: PropTypes.string.isRequired,
  onChange: PropTypes.func.isRequired,
  defaultOpen: PropTypes.bool,
};
