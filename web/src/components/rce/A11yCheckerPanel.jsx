import React, { useEffect, useState, useCallback } from 'react';
import { X, ChevronLeft, ChevronRight, CheckCircle2 } from 'lucide-react';
import { Button } from '../ui/button';
import { Input } from '../ui/input';
import { checkDocument } from './a11yRules';

/**
 * Accessibility checker panel (G005). Walks the issues one at a time like
 * Canvas's checker: selects the offending content, explains the problem and
 * offers a fix. The list is recomputed after every edit, so applying a fix
 * moves on to the next remaining issue.
 */
function selectIssue(editor, issue) {
  if (!editor || !issue) return;
  try {
    const node = editor.state.doc.nodeAt(issue.pos);
    const chain = editor.chain();
    if (node && (node.isAtom || node.type.name === 'table')) chain.setNodeSelection(issue.pos);
    else chain.setTextSelection(issue.pos + (node && !node.isText ? 1 : 0));
    chain.scrollIntoView().run();
  } catch {
    /* position no longer valid; the next recompute fixes it */
  }
}

export default function A11yCheckerPanel({ editor, onClose }) {
  const [issues, setIssues] = useState(() => checkDocument(editor));
  const [index, setIndex] = useState(0);
  const [text, setText] = useState('');
  const [checked, setChecked] = useState(false);

  useEffect(() => {
    if (!editor) return undefined;
    const recompute = () => setIssues(checkDocument(editor));
    editor.on('update', recompute);
    return () => editor.off('update', recompute);
  }, [editor]);

  const safeIndex = Math.min(index, Math.max(issues.length - 1, 0));
  const issue = issues[safeIndex];

  useEffect(() => {
    setText(issue?.fix?.initial || '');
    setChecked(false);
    // Only when moving to a different issue.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [issue?.id]);

  const go = useCallback((delta) => {
    if (issues.length === 0) return;
    const next = (safeIndex + delta + issues.length) % issues.length;
    setIndex(next);
    selectIssue(editor, issues[next]);
  }, [issues, safeIndex, editor]);

  const apply = () => {
    if (!issue?.fix) return;
    if (issue.fix.kind === 'text') issue.fix.apply(editor, text);
    else if (issue.fix.kind === 'checkbox-text') issue.fix.apply(editor, { text, checked });
    else issue.fix.apply(editor);
  };

  const canApply = !issue?.fix ? false
    : issue.fix.kind === 'text' ? !!text.trim()
      : issue.fix.kind === 'checkbox-text' ? (checked || !!text.trim())
        : true;

  return (
    <section
      role="region"
      aria-label="Accessibility checker"
      className="border-b border-input bg-muted/20 px-3 py-2 text-sm"
    >
      <div className="flex items-center justify-between gap-2">
        <h3 className="font-semibold">Accessibility checker</h3>
        <button type="button" onClick={onClose} aria-label="Close accessibility checker" className="rounded p-1 hover:bg-accent">
          <X className="size-4" />
        </button>
      </div>
      <div aria-live="polite">
        {issues.length === 0 ? (
          <p className="mt-1 flex items-center gap-2 text-green-700 dark:text-green-400">
            <CheckCircle2 className="size-4" aria-hidden="true" /> No accessibility issues found.
          </p>
        ) : (
          <div className="mt-1 space-y-2">
            <p className="text-xs text-muted-foreground">Issue {safeIndex + 1} of {issues.length}</p>
            <p className="font-medium">{issue.message}</p>
            <p className="text-muted-foreground">{issue.why}</p>
            {issue.fix && (issue.fix.kind === 'text' || issue.fix.kind === 'checkbox-text') && (
              <div className="space-y-1">
                <label htmlFor="rce-a11y-fix" className="text-xs font-medium">{issue.fix.label}</label>
                <Input id="rce-a11y-fix" value={text} disabled={checked} onChange={(e) => setText(e.target.value)} />
              </div>
            )}
            {issue.fix?.kind === 'checkbox-text' && (
              <label className="flex items-center gap-2 text-xs">
                <input type="checkbox" checked={checked} onChange={(e) => setChecked(e.target.checked)} />
                {issue.fix.checkboxLabel}
              </label>
            )}
          </div>
        )}
      </div>
      {issues.length > 0 && (
        <div className="mt-2 flex flex-wrap items-center gap-2">
          <Button type="button" variant="outline" size="sm" onClick={() => go(-1)} aria-label="Previous issue"><ChevronLeft className="size-4" /></Button>
          <Button type="button" variant="outline" size="sm" onClick={() => go(1)} aria-label="Next issue"><ChevronRight className="size-4" /></Button>
          <Button type="button" variant="outline" size="sm" onClick={() => selectIssue(editor, issue)}>Show in editor</Button>
          {issue.fix && (
            <Button type="button" size="sm" disabled={!canApply} onClick={apply}>
              {issue.fix.buttonLabel || 'Apply'}
            </Button>
          )}
        </div>
      )}
    </section>
  );
}
