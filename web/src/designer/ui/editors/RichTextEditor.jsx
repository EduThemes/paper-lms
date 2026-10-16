/* eslint-disable react/prop-types -- Paper JSX components don't declare propTypes */
/**
 * A tiny rich-text editor: bold, italic, link, bulleted and numbered lists. Output is limited to the
 * RichText subset (p, strong, em, a, ul, ol, li, br); teachers never see HTML. Pasted content (Word,
 * Google Docs) is cleaned to the same subset.
 */
import { useEffect, useId, useLayoutEffect, useRef, useState } from 'react';
import { Bold, Italic, Link as LinkIcon, List, ListOrdered } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';
import { normalizeRich } from '../lib/richtext';
import { sanitizeRich } from '../lib/engine';
import { useDesignerT } from '../i18n';

const escText = (s) => s.replace(/[&<>]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;' })[c]);

export function RichTextEditor({ label, value, onChange }) {
  const { t } = useDesignerT();
  const id = useId();
  const ref = useRef(null);
  const lastEmitted = useRef(value);
  const savedRange = useRef(null);
  const [linkOpen, setLinkOpen] = useState(false);
  const [linkUrl, setLinkUrl] = useState('https://');
  const [active, setActive] = useState({ bold: false, italic: false, insertUnorderedList: false, insertOrderedList: false });

  // The value can come from a stored design (any teacher or TA can write one through the API) and this
  // editor lives in Paper's own document, so it is always written through sanitizeRich: only p, strong, em,
  // a[href], ul, ol, li and br ever reach the DOM, never an <img onerror> or an <iframe>.
  useLayoutEffect(() => {
    if (ref.current) ref.current.innerHTML = sanitizeRich(value ?? '');
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    // Only rewrite the DOM when the value changed from outside (undo, a fix) — never while typing.
    if (ref.current && value !== lastEmitted.current) {
      ref.current.innerHTML = sanitizeRich(value ?? '');
      lastEmitted.current = value;
    }
  }, [value]);

  function emit() {
    if (!ref.current) return;
    const html = normalizeRich(ref.current.innerHTML);
    lastEmitted.current = html;
    onChange(html);
  }

  function refreshState() {
    try {
      setActive({
        bold: document.queryCommandState('bold'),
        italic: document.queryCommandState('italic'),
        insertUnorderedList: document.queryCommandState('insertUnorderedList'),
        insertOrderedList: document.queryCommandState('insertOrderedList'),
      });
    } catch {
      /* not supported (tests) */
    }
  }

  function run(cmd) {
    ref.current?.focus();
    try {
      document.execCommand('styleWithCSS', false, 'false');
      document.execCommand(cmd);
    } catch {
      /* ignore */
    }
    emit();
    refreshState();
  }

  function openLink() {
    const sel = window.getSelection();
    savedRange.current = sel && sel.rangeCount && ref.current?.contains(sel.anchorNode) ? sel.getRangeAt(0).cloneRange() : null;
    setLinkUrl('https://');
    setLinkOpen(true);
  }

  function applyLink() {
    const url = linkUrl.trim();
    if (!/^(https?:\/\/.+|mailto:.+|\/.+)/i.test(url)) return;
    ref.current?.focus();
    const sel = window.getSelection();
    if (savedRange.current && sel) {
      sel.removeAllRanges();
      sel.addRange(savedRange.current);
    }
    try {
      if (!sel || sel.isCollapsed) {
        const a = document.createElement('a');
        a.href = url;
        a.textContent = url.replace(/^mailto:/, '');
        document.execCommand('insertHTML', false, a.outerHTML);
      } else {
        document.execCommand('createLink', false, url);
      }
    } catch {
      /* ignore */
    }
    setLinkOpen(false);
    emit();
  }

  function onPaste(e) {
    const html = e.clipboardData.getData('text/html');
    const text = e.clipboardData.getData('text/plain');
    e.preventDefault();
    const clean = html
      ? normalizeRich(html)
      : text
          .split(/\n{2,}/)
          .map((p) => `<p>${escText(p).replace(/\n/g, '<br>')}</p>`)
          .join('');
    try {
      document.execCommand('insertHTML', false, clean);
    } catch {
      /* ignore */
    }
    emit();
  }

  const tool = (cmd, labelText, Icon) => (
    <button
      type="button"
      aria-pressed={active[cmd]}
      aria-label={labelText}
      title={labelText}
      onMouseDown={(e) => e.preventDefault()}
      onClick={() => run(cmd)}
      className={cn(
        'inline-flex h-8 w-8 items-center justify-center rounded text-text-secondary hover:bg-surface-2 hover:text-text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
        active[cmd] && 'bg-surface-2 text-text-primary',
      )}
    >
      <Icon className="h-4 w-4" aria-hidden="true" />
    </button>
  );

  return (
    <div className="space-y-1.5">
      <span id={`${id}-label`} className="text-sm font-medium text-text-primary">
        {label}
      </span>
      <div className="overflow-hidden rounded-md border border-input bg-surface-0 focus-within:ring-2 focus-within:ring-ring">
        <div role="toolbar" aria-label={t('rte.toolbar')} aria-controls={`${id}-area`} className="flex gap-0.5 border-b border-border bg-surface-1 p-1">
          {tool('bold', t('rte.bold'), Bold)}
          {tool('italic', t('rte.italic'), Italic)}
          <button
            type="button"
            aria-label={t('rte.link')}
            title={t('rte.link')}
            aria-expanded={linkOpen}
            onMouseDown={(e) => e.preventDefault()}
            onClick={openLink}
            className="inline-flex h-8 w-8 items-center justify-center rounded text-text-secondary hover:bg-surface-2 hover:text-text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            <LinkIcon className="h-4 w-4" aria-hidden="true" />
          </button>
          {tool('insertUnorderedList', t('rte.bullets'), List)}
          {tool('insertOrderedList', t('rte.numbers'), ListOrdered)}
        </div>
        <div
          ref={ref}
          id={`${id}-area`}
          contentEditable
          suppressContentEditableWarning
          role="textbox"
          aria-multiline="true"
          aria-labelledby={`${id}-label`}
          tabIndex={0}
          onInput={emit}
          onBlur={emit}
          onKeyUp={refreshState}
          onMouseUp={refreshState}
          onPaste={onPaste}
          className="min-h-[7rem] px-3 py-2 text-sm text-text-primary outline-none [&_a]:text-brand-600 [&_a]:underline [&_ol]:list-decimal [&_ol]:pl-6 [&_p]:my-1.5 [&_ul]:list-disc [&_ul]:pl-6 dark:[&_a]:text-brand-300"
        />
        {linkOpen && (
          <div className="flex flex-wrap items-end gap-2 border-t border-border bg-surface-1 p-2">
            <div className="min-w-[12rem] flex-1 space-y-1">
              <label htmlFor={`${id}-url`} className="text-xs font-medium text-text-primary">
                {t('rte.linkUrl')}
              </label>
              <Input
                id={`${id}-url`}
                value={linkUrl}
                onChange={(e) => setLinkUrl(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    e.preventDefault();
                    applyLink();
                  }
                  if (e.key === 'Escape') setLinkOpen(false);
                }}
                className="h-9 bg-surface-0 text-text-primary"
                autoFocus
              />
            </div>
            <Button type="button" size="sm" onClick={applyLink}>
              {t('rte.addLink')}
            </Button>
            <Button type="button" size="sm" variant="outline" onClick={() => setLinkOpen(false)}>
              {t('cancel')}
            </Button>
          </div>
        )}
      </div>
    </div>
  );
}
