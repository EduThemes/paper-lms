import { useEffect, useRef, useState, useCallback, useMemo } from 'react';
import {
  Dialog, DialogContent, DialogHeader, DialogTitle,
  DialogDescription, DialogFooter,
} from '../ui/dialog';
import { Button } from '../ui/button';
import { PALETTE, requiresAdvancedMode } from './math/palette';
import PaletteButton from './math/PaletteButton';

/**
 * @typedef {Object} MathInputDialogProps
 * @property {boolean} open
 * @property {() => void} onClose
 * @property {(latex: string) => void} onInsert
 * @property {string} [initialLatex]   — for future "edit existing equation" wiring
 */

/* ---------- Lazy loaders ---------- */

let _katexPromise = null;
function loadKatex() {
  if (!_katexPromise) {
    _katexPromise = Promise.all([
      import('katex'),
      import('katex/dist/katex.min.css'),
    ]).then(([mod]) => mod.default || mod);
  }
  return _katexPromise;
}

let _mathlivePromise = null;
function loadMathlive() {
  if (!_mathlivePromise) {
    _mathlivePromise = import('mathlive');
  }
  return _mathlivePromise;
}

/* ---------- Dialog ---------- */

export default function MathInputDialog({ open, onClose, onInsert, initialLatex = '' }) {
  const [advanced, setAdvanced] = useState(false);
  const [rawLatex, setRawLatex] = useState(initialLatex);
  const [activeTab, setActiveTab] = useState(PALETTE[0].name);
  const [katex, setKatex] = useState(null);
  const [mathliveReady, setMathliveReady] = useState(false);
  const [error, setError] = useState(null);

  const mathFieldRef = useRef(null);
  const mathHostRef = useRef(null);
  const previewRef = useRef(null);

  // Reset every open
  useEffect(() => {
    if (!open) return;
    const initial = initialLatex || '';
    setRawLatex(initial);
    setActiveTab(PALETTE[0].name);
    setError(null);
    // If migrated content uses syntax mathlive can't render, default to Advanced.
    setAdvanced(requiresAdvancedMode(initial));
  }, [open, initialLatex]);

  // Lazy KaTeX (preview + palette button labels)
  useEffect(() => {
    if (!open) return;
    loadKatex().then(setKatex).catch((e) => setError(e?.message || 'Preview engine failed to load'));
  }, [open]);

  // Lazy MathLive (only if WYSIWYG mode)
  useEffect(() => {
    if (!open || advanced || mathliveReady) return;
    loadMathlive()
      .then(() => setMathliveReady(true))
      .catch((e) => {
        // Fall back to Advanced if MathLive can't load — never strand the user.
        setError(e?.message || 'Math editor failed to load — using LaTeX mode');
        setAdvanced(true);
      });
  }, [open, advanced, mathliveReady]);

  // Mount <math-field> imperatively once mathlive is loaded and the host div is in the DOM.
  // Doing this outside React's render avoids React 18 quirks with custom-element refs/props.
  useEffect(() => {
    if (!open || advanced || !mathliveReady) return;
    const host = mathHostRef.current;
    if (!host) return;
    // Don't double-mount across re-renders
    let mf = mathFieldRef.current;
    if (!mf || mf.parentNode !== host) {
      host.replaceChildren();
      mf = document.createElement('math-field');
      mf.setAttribute('aria-label', 'Equation editor');
      mf.style.display = 'block';
      mf.style.fontSize = '1.5rem';
      mf.style.minHeight = '3rem';
      mf.style.setProperty('--primary', 'rgb(var(--color-brand-600))');
      mf.style.setProperty('--caret-color', 'rgb(var(--color-text-primary))');
      mf.style.setProperty('--text-font-family', 'inherit');
      mf.addEventListener('input', () => setRawLatex(mf.value || ''));
      host.appendChild(mf);
      mathFieldRef.current = mf;
    }
    mf.value = rawLatex || '';
    const t = setTimeout(() => { try { mf.focus(); } catch { /* ignore */ } }, 50);
    return () => clearTimeout(t);
    // We deliberately don't depend on rawLatex — the input event keeps state in sync; we only
    // (re)mount on open/mode/ready changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, advanced, mathliveReady]);

  // Render preview whenever the LaTeX changes
  useEffect(() => {
    if (!previewRef.current || !katex) return;
    if (!rawLatex.trim()) {
      previewRef.current.textContent = '';
      setError(null);
      return;
    }
    try {
      katex.render(rawLatex, previewRef.current, {
        throwOnError: false,
        displayMode: true,
        output: 'htmlAndMathml',
      });
      setError(null);
    } catch (e) {
      setError(e?.message || 'Invalid LaTeX');
    }
  }, [rawLatex, katex]);

  /* ---------- Insert from palette ---------- */

  const insertFromPalette = useCallback((entry) => {
    if (advanced) {
      // In Advanced mode, splice the raw command at the cursor of the textarea.
      const ta = previewRef.current?.parentElement?.querySelector('textarea');
      const cmd = entry.advancedCommand || entry.command;
      if (ta) {
        const start = ta.selectionStart ?? rawLatex.length;
        const end = ta.selectionEnd ?? rawLatex.length;
        const next = rawLatex.slice(0, start) + cmd + rawLatex.slice(end);
        setRawLatex(next);
        // Restore caret just after inserted command
        requestAnimationFrame(() => {
          ta.focus();
          const pos = start + cmd.length;
          ta.setSelectionRange(pos, pos);
        });
      } else {
        setRawLatex(rawLatex + cmd);
      }
      return;
    }
    // WYSIWYG: ask mathlive to insert with placeholder selection.
    const mf = mathFieldRef.current;
    if (!mf) return;
    try {
      mf.executeCommand(['insert', entry.command, { selectionMode: 'placeholder' }]);
      mf.focus();
    } catch {
      // Fallback: blunt-append (mathlive >= 0.99 also supports `insert(latex)`)
      try { mf.insert(entry.command, { selectionMode: 'placeholder' }); } catch { /* ignore */ }
    }
  }, [advanced, rawLatex]);

  /* ---------- Mode toggle ---------- */

  const toggleMode = useCallback(() => {
    if (advanced) {
      // Advanced → WYSIWYG: the math-field effect will (re)mount and pick up rawLatex.
      setAdvanced(false);
    } else {
      // WYSIWYG → Advanced: pull the current value out of the math-field.
      const mf = mathFieldRef.current;
      const v = mf ? (mf.value || '') : rawLatex;
      mathFieldRef.current = null;
      setRawLatex(v);
      setAdvanced(true);
    }
  }, [advanced, rawLatex]);

  /* ---------- Submit ---------- */

  const submit = useCallback(() => {
    const latex = (advanced
      ? rawLatex
      : (mathFieldRef.current?.value ?? rawLatex)
    ).trim();
    if (!latex) return;
    onInsert(latex);
  }, [advanced, rawLatex, onInsert]);

  /* ---------- Layout ---------- */

  const activeGroup = useMemo(
    () => PALETTE.find((g) => g.name === activeTab) || PALETTE[0],
    [activeTab],
  );

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onClose(); }}>
      <DialogContent className="max-w-3xl">
        <DialogHeader>
          <DialogTitle>Math equation</DialogTitle>
          <DialogDescription>
            Build your equation visually, or switch to LaTeX for fine control.
          </DialogDescription>
        </DialogHeader>

        {/* ---------- Editor ---------- */}
        {advanced ? (
          <textarea
            value={rawLatex}
            onChange={(e) => setRawLatex(e.target.value)}
            placeholder="\frac{a}{b}"
            aria-label="LaTeX source"
            rows={4}
            autoFocus
            className="w-full rounded-md border border-border-default bg-surface-0 px-3 py-2 font-mono text-sm text-text-primary placeholder:text-text-disabled focus:outline-none focus:ring-2 focus:ring-brand-500"
            onKeyDown={(e) => { if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) submit(); }}
          />
        ) : (
          <div className="math-field-frame rounded-md border border-border-default bg-surface-0 px-3 py-2 focus-within:ring-2 focus-within:ring-brand-500">
            {mathliveReady ? (
              <div ref={mathHostRef} />
            ) : (
              <p className="text-sm text-text-tertiary py-3">Loading equation editor…</p>
            )}
          </div>
        )}

        {/* ---------- Palette ---------- */}
        <div className="mt-2">
          <div role="tablist" aria-label="Symbol categories" className="flex flex-wrap gap-1 border-b border-border-default mb-2">
            {PALETTE.map((g) => {
              const active = g.name === activeTab;
              return (
                <button
                  key={g.name}
                  type="button"
                  role="tab"
                  aria-selected={active}
                  onClick={() => setActiveTab(g.name)}
                  className={`px-3 py-1.5 text-xs font-medium border-b-2 -mb-px transition-colors ${
                    active
                      ? 'border-brand-600 text-brand-600'
                      : 'border-transparent text-text-tertiary hover:text-text-secondary hover:border-border-strong'
                  }`}
                >
                  {g.name}
                </button>
              );
            })}
          </div>
          <div
            role="tabpanel"
            aria-label={`${activeGroup.name} symbols`}
            className="paper-scroll grid grid-cols-[repeat(auto-fill,minmax(2.75rem,1fr))] gap-1.5 max-h-44 overflow-y-auto pr-1"
          >
            {activeGroup.commands.map((entry, i) => (
              <PaletteButton
                key={`${entry.command}-${i}`}
                entry={entry}
                katex={katex}
                onInsert={insertFromPalette}
              />
            ))}
          </div>
        </div>

        {/* ---------- Live preview ---------- */}
        <div className="mt-2">
          <div className="text-[11px] font-semibold uppercase tracking-wider text-text-tertiary mb-1">Preview</div>
          <div className="rounded-md border border-border-subtle bg-surface-1 p-3 min-h-[3rem]" aria-live="polite">
            {!katex ? (
              <span className="text-sm text-text-tertiary">Loading preview…</span>
            ) : !rawLatex.trim() ? (
              <span className="text-sm text-text-tertiary">Equation preview will appear here.</span>
            ) : (
              <div ref={previewRef} className="text-text-primary" />
            )}
            {error && <p className="text-xs text-accent-danger mt-1">{error}</p>}
          </div>
        </div>

        <DialogFooter className="flex sm:justify-between items-center">
          <Button
            variant="ghost"
            type="button"
            onClick={toggleMode}
            className="text-text-secondary"
          >
            {advanced ? 'Switch to visual editor' : 'Switch to LaTeX'}
          </Button>
          <div className="flex gap-2">
            <Button variant="outline" type="button" onClick={onClose}>Cancel</Button>
            <Button type="button" onClick={submit} disabled={!rawLatex.trim()}>Insert</Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
