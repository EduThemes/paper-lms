import React, { useEffect, useRef } from 'react';

/**
 * Single button in the math palette.
 *
 * Renders the LaTeX `displayName` (or `command` if no display) via KaTeX so
 * teachers see exactly what symbol they'll insert. Falls back to plain text
 * if KaTeX hasn't loaded yet.
 */
export default function PaletteButton({ entry, katex, onInsert }) {
  const ref = useRef(null);
  const display = entry.displayName || entry.command;

  useEffect(() => {
    if (!ref.current || !katex) return;
    try {
      katex.render(display, ref.current, {
        throwOnError: false,
        displayMode: false,
        output: 'html',
      });
    } catch {
      ref.current.textContent = display;
    }
  }, [katex, display]);

  return (
    <button
      type="button"
      onClick={() => onInsert(entry)}
      title={`${entry.label} — LaTeX: ${entry.command}`}
      aria-label={`${entry.label}, LaTeX: ${entry.command}`}
      className="h-10 min-w-[2.5rem] px-2 flex items-center justify-center rounded-md border border-border-default bg-surface-0 text-text-primary hover:bg-brand-50 hover:border-brand-300 hover:text-brand-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 focus-visible:ring-offset-1 transition-colors"
    >
      <span ref={ref} className="text-sm leading-none">
        {!katex ? display : null}
      </span>
    </button>
  );
}
