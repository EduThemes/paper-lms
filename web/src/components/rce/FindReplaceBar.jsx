import React, { useEffect, useMemo, useRef, useState } from 'react';
import { X, ChevronUp, ChevronDown } from 'lucide-react';
import { Button } from '../ui/button';
import { Input } from '../ui/input';
import { findMatches, replaceMatch, replaceAll } from './findReplace';

/**
 * Find and replace bar (G016). Enter / Shift+Enter step through matches;
 * Escape closes the bar and returns focus to the editor.
 */
export default function FindReplaceBar({ editor, onClose }) {
  const [query, setQuery] = useState('');
  const [replacement, setReplacement] = useState('');
  const [caseSensitive, setCaseSensitive] = useState(false);
  const [current, setCurrent] = useState(0);
  const [version, setVersion] = useState(0);
  const [status, setStatus] = useState('');
  const findRef = useRef(null);

  useEffect(() => { findRef.current?.focus(); }, []);

  useEffect(() => {
    if (!editor) return undefined;
    const bump = () => setVersion((v) => v + 1);
    editor.on('update', bump);
    return () => editor.off('update', bump);
  }, [editor]);

  const matches = useMemo(
    () => (editor ? findMatches(editor.state.doc, query, { caseSensitive }) : []),
    // version: recompute after edits
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [editor, query, caseSensitive, version],
  );
  const idx = matches.length ? Math.min(current, matches.length - 1) : -1;

  const select = (i) => {
    const m = matches[i];
    if (!m) return;
    setCurrent(i);
    editor.chain().setTextSelection(m).scrollIntoView().run();
  };
  const step = (d) => {
    if (!matches.length) return;
    select((idx + d + matches.length) % matches.length);
  };

  const close = () => {
    onClose();
    editor?.commands.focus();
  };

  const onKeyDown = (e) => {
    if (e.key === 'Escape') { e.preventDefault(); close(); }
    if (e.key === 'Enter' && e.target === findRef.current) { e.preventDefault(); step(e.shiftKey ? -1 : 1); }
  };

  return (
    <div role="search" aria-label="Find and replace" onKeyDown={onKeyDown}
      className="flex flex-wrap items-center gap-2 border-b border-input bg-muted/20 px-2 py-1.5 text-sm">
      <Input ref={findRef} aria-label="Find" placeholder="Find" value={query} className="h-8 w-40"
        onChange={(e) => { setQuery(e.target.value); setCurrent(0); setStatus(''); }} />
      <span className="min-w-[4.5rem] text-xs text-muted-foreground" aria-live="polite">
        {query ? (matches.length ? `${idx + 1} of ${matches.length}` : 'No results') : ''}
      </span>
      <Button type="button" variant="ghost" size="icon" className="h-8 w-8" aria-label="Previous match" onClick={() => step(-1)} disabled={!matches.length}><ChevronUp /></Button>
      <Button type="button" variant="ghost" size="icon" className="h-8 w-8" aria-label="Next match" onClick={() => step(1)} disabled={!matches.length}><ChevronDown /></Button>
      <Input aria-label="Replace with" placeholder="Replace with" value={replacement} className="h-8 w-40"
        onChange={(e) => setReplacement(e.target.value)} />
      <Button type="button" variant="outline" size="sm" disabled={idx < 0}
        onClick={() => { replaceMatch(editor, matches[idx], replacement); setStatus('Replaced 1'); }}>Replace</Button>
      <Button type="button" variant="outline" size="sm" disabled={!matches.length}
        onClick={() => { const n = replaceAll(editor, query, replacement, { caseSensitive }); setStatus(`Replaced ${n}`); }}>Replace all</Button>
      <label className="flex items-center gap-1 text-xs">
        <input type="checkbox" checked={caseSensitive} onChange={(e) => setCaseSensitive(e.target.checked)} /> Match case
      </label>
      {status && <span className="text-xs text-muted-foreground" role="status">{status}</span>}
      <button type="button" onClick={close} aria-label="Close find and replace" className="ml-auto rounded p-1 hover:bg-accent">
        <X className="size-4" />
      </button>
    </div>
  );
}
