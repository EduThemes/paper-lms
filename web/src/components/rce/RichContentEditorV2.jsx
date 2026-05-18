import React, { useEffect, useRef, useState, useCallback, useMemo } from 'react';
import { useEditor, EditorContent } from '@tiptap/react';
import { BubbleMenu, FloatingMenu } from '@tiptap/react/menus';
import { Sparkles, Loader2, ChevronDown } from 'lucide-react';
import StarterKit from '@tiptap/starter-kit';
import Link from '@tiptap/extension-link';
import Image from '@tiptap/extension-image';
import Placeholder from '@tiptap/extension-placeholder';
import { Table } from '@tiptap/extension-table';
import TableRow from '@tiptap/extension-table-row';
import TableCell from '@tiptap/extension-table-cell';
import TableHeader from '@tiptap/extension-table-header';
import TaskList from '@tiptap/extension-task-list';
import TaskItem from '@tiptap/extension-task-item';
import CharacterCount from '@tiptap/extension-character-count';
import Typography from '@tiptap/extension-typography';
import Underline from '@tiptap/extension-underline';
import TextAlign from '@tiptap/extension-text-align';
import Highlight from '@tiptap/extension-highlight';
import { sanitizeHTML } from '../RichContentViewer';
import { getCSRFToken } from '../../services/api';
import { useClickOutside } from '../../hooks/useClickOutside';
import RCEToolbar from './RCEToolbar';
import RestoreAutosaveModal from './RestoreAutosaveModal';

/**
 * @typedef {Object} RCEV2Props
 * @property {string} value - Initial HTML content
 * @property {(html: string) => void} onChange - Called with sanitized HTML on every change
 * @property {string=} placeholder - Empty-state placeholder
 * @property {(string|number)=} courseId - Course context, used by ContentPicker
 * @property {string=} autoSaveKey - If set, drafts persist to localStorage[`paperlms.rce.${autoSaveKey}`]
 * @property {string=} className - Pass-through wrapper class
 */

const AUTOSAVE_PREFIX = 'paperlms.rce.';
const AUTOSAVE_DEBOUNCE_MS = 2000;
const RESTORE_THRESHOLD_MS = 5000; // ignore drafts older than props.value by less than this

/** Read draft from localStorage. Returns { html, savedAt } or null. */
function readDraft(key) {
  if (!key || typeof window === 'undefined') return null;
  try {
    const raw = window.localStorage.getItem(AUTOSAVE_PREFIX + key);
    if (!raw) return null;
    return JSON.parse(raw);
  } catch {
    return null;
  }
}

/** Persist a draft. Silent on quota errors. */
function writeDraft(key, html) {
  if (!key || typeof window === 'undefined') return;
  try {
    window.localStorage.setItem(
      AUTOSAVE_PREFIX + key,
      JSON.stringify({ html, savedAt: Date.now() })
    );
  } catch {
    /* quota exceeded — ignore */
  }
}

function clearDraft(key) {
  if (!key || typeof window === 'undefined') return;
  try {
    window.localStorage.removeItem(AUTOSAVE_PREFIX + key);
  } catch { /* ignore */ }
}

/**
 * RichContentEditorV2 — TipTap-based rich content editor.
 * Drop-in compatible with the existing RichContentEditor surface (value/onChange).
 *
 * @param {RCEV2Props} props
 */
export default function RichContentEditorV2({
  value = '',
  onChange,
  placeholder = 'Start typing…',
  courseId,
  autoSaveKey,
  className,
}) {
  const [restorePrompt, setRestorePrompt] = useState(null); // { html, savedAt }
  const debounceRef = useRef(null);
  // AI Assist UI state
  const [aiMenuOpen, setAiMenuOpen] = useState(false);
  const [aiBusy, setAiBusy] = useState(null); // null | 'outline' | 'summarize' | 'rewrite'
  const [aiError, setAiError] = useState(null);
  const aiMenuRef = useRef(null);

  const extensions = useMemo(() => [
    StarterKit.configure({
      // StarterKit v3 already includes Underline + Link, but we override below for full config.
      link: false,
      underline: false,
      codeBlock: { HTMLAttributes: { class: 'rce-code-block' } },
    }),
    Underline,
    Link.configure({
      openOnClick: false,
      autolink: true,
      HTMLAttributes: { rel: 'noopener noreferrer nofollow', target: '_blank' },
    }),
    Image.configure({
      inline: false,
      allowBase64: false,
      HTMLAttributes: { class: 'rce-image' },
    }),
    Placeholder.configure({ placeholder }),
    Table.configure({ resizable: true }),
    TableRow, TableHeader, TableCell,
    TaskList,
    TaskItem.configure({ nested: true }),
    CharacterCount,
    Typography,
    TextAlign.configure({ types: ['heading', 'paragraph'] }),
    Highlight.configure({ multicolor: true }),
  ], [placeholder]);

  const editor = useEditor({
    extensions,
    content: value || '',
    editorProps: {
      attributes: {
        class: 'rce-v2-content prose prose-sm sm:prose-base max-w-none focus:outline-none min-h-[200px] px-4 py-3',
        role: 'textbox',
        'aria-multiline': 'true',
        'aria-label': 'Rich content editor',
      },
    },
    onUpdate: ({ editor: ed }) => {
      const html = sanitizeHTML(ed.getHTML());
      onChange?.(html);
      if (autoSaveKey) {
        if (debounceRef.current) clearTimeout(debounceRef.current);
        debounceRef.current = setTimeout(() => writeDraft(autoSaveKey, html), AUTOSAVE_DEBOUNCE_MS);
      }
    },
  });

  // Restore-prompt logic on mount
  useEffect(() => {
    if (!autoSaveKey || !editor) return;
    const draft = readDraft(autoSaveKey);
    if (!draft || !draft.html) return;
    if (draft.html === (value || '')) return;
    // Show prompt only when draft is meaningfully newer than what we received
    if (Date.now() - draft.savedAt > RESTORE_THRESHOLD_MS) {
      setRestorePrompt(draft);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [editor, autoSaveKey]);

  // Sync external value changes (e.g. parent reset)
  useEffect(() => {
    if (!editor) return;
    const current = editor.getHTML();
    if (value !== undefined && value !== current) {
      editor.commands.setContent(value || '', { emitUpdate: false });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value, editor]);

  // Cleanup pending autosave on unmount
  useEffect(() => () => {
    if (debounceRef.current) clearTimeout(debounceRef.current);
  }, []);

  const handleRestore = useCallback(() => {
    if (!restorePrompt || !editor) return;
    editor.commands.setContent(restorePrompt.html);
    onChange?.(sanitizeHTML(restorePrompt.html));
    setRestorePrompt(null);
  }, [restorePrompt, editor, onChange]);

  const handleDiscard = useCallback(() => {
    if (autoSaveKey) clearDraft(autoSaveKey);
    setRestorePrompt(null);
  }, [autoSaveKey]);

  // ----- AI Assist ---------------------------------------------------------
  // Close the AI dropdown on outside click / Escape. The hook attaches
  // unconditionally; the close fn is a no-op when the menu is already closed.
  const closeAiMenu = useCallback(() => {
    if (aiMenuOpen) setAiMenuOpen(false);
  }, [aiMenuOpen]);
  useClickOutside(aiMenuRef, closeAiMenu);

  /**
   * Run an AI Assist action against the current selection (or whole doc).
   * Replaces the selection (or inserts at cursor) with the returned text.
   */
  const runAiAssist = useCallback(async (action) => {
    if (!editor || aiBusy) return;
    setAiError(null);

    const { from, to, empty } = editor.state.selection;
    const selectedText = empty ? '' : editor.state.doc.textBetween(from, to, '\n\n');
    const fullText = editor.getText();
    const text = selectedText || fullText;
    if (!text || !text.trim()) {
      setAiError('Nothing to send — write or select some text first.');
      return;
    }

    setAiBusy(action);
    try {
      const res = await fetch(`/api/v1/ai_assist/${action}`, {
        method: 'POST',
        credentials: 'include',
        headers: {
          'Content-Type': 'application/json',
          'X-CSRF-Token': getCSRFToken(),
        },
        body: JSON.stringify({ text, style: action === 'rewrite' ? 'clearer' : '' }),
      });
      if (!res.ok) {
        let msg = `AI Assist failed (${res.status})`;
        try {
          const body = await res.json();
          if (body?.errors?.[0]?.message) msg = body.errors[0].message;
        } catch { /* non-JSON body */ }
        throw new Error(msg);
      }
      const data = await res.json();
      const result = (data?.result || '').trim();
      if (!result) throw new Error('AI Assist returned an empty response.');

      const chain = editor.chain().focus();
      if (selectedText) {
        chain.deleteRange({ from, to }).insertContent(result).run();
      } else {
        chain.insertContent(result).run();
      }
      setAiMenuOpen(false);
    } catch (err) {
      setAiError(err?.message || 'AI Assist request failed');
    } finally {
      setAiBusy(null);
    }
  }, [editor, aiBusy]);

  if (!editor) {
    return <div className="rce-v2-loading min-h-[240px] rounded-md border border-input bg-background p-4 text-sm text-muted-foreground">Loading editor…</div>;
  }

  const charCount = editor.storage.characterCount?.characters() ?? 0;

  return (
    <div className={['rce-v2 rounded-md border border-input bg-background', className].filter(Boolean).join(' ')}>
      <RCEToolbar editor={editor} courseId={courseId} />

      {/* AI Assist dropdown — proxies to /api/v1/ai_assist/:action (Anthropic Messages API). */}
      <div className="flex items-center gap-2 border-b border-input bg-muted/20 px-2 py-1">
        <div className="relative" ref={aiMenuRef}>
          <button
            type="button"
            aria-haspopup="menu"
            aria-expanded={aiMenuOpen}
            aria-label="AI Assist"
            disabled={!!aiBusy}
            onClick={() => setAiMenuOpen((s) => !s)}
            className="inline-flex items-center gap-1 rounded-md border border-input bg-background px-2 py-1 text-xs font-medium hover:bg-accent disabled:opacity-60"
          >
            {aiBusy ? <Loader2 className="size-3.5 animate-spin" /> : <Sparkles className="size-3.5 text-purple-600" />}
            <span>AI Assist</span>
            <ChevronDown className="size-3" />
          </button>
          {aiMenuOpen && (
            <div role="menu" className="absolute left-0 top-full z-50 mt-1 flex w-48 flex-col rounded-md border bg-popover p-1 shadow-md">
              <button role="menuitem" type="button" disabled={!!aiBusy}
                onClick={() => runAiAssist('outline')}
                className="flex items-center justify-between px-3 py-1.5 text-left text-sm rounded hover:bg-accent disabled:opacity-60">
                <span>Outline</span>
                {aiBusy === 'outline' && <Loader2 className="size-3.5 animate-spin" />}
              </button>
              <button role="menuitem" type="button" disabled={!!aiBusy}
                onClick={() => runAiAssist('summarize')}
                className="flex items-center justify-between px-3 py-1.5 text-left text-sm rounded hover:bg-accent disabled:opacity-60">
                <span>Summarize</span>
                {aiBusy === 'summarize' && <Loader2 className="size-3.5 animate-spin" />}
              </button>
              <button role="menuitem" type="button" disabled={!!aiBusy}
                onClick={() => runAiAssist('rewrite')}
                className="flex items-center justify-between px-3 py-1.5 text-left text-sm rounded hover:bg-accent disabled:opacity-60">
                <span>Rewrite (clearer)</span>
                {aiBusy === 'rewrite' && <Loader2 className="size-3.5 animate-spin" />}
              </button>
            </div>
          )}
        </div>
        {aiError && (
          <div role="alert" className="flex items-center gap-2 text-xs text-destructive">
            <span>{aiError}</span>
            <button type="button" onClick={() => setAiError(null)} className="underline">dismiss</button>
          </div>
        )}
      </div>

      <BubbleMenu editor={editor} className="flex gap-1 rounded-md border bg-popover p-1 shadow-md">
        <button type="button" aria-label="Bold" onClick={() => editor.chain().focus().toggleBold().run()} className={`px-2 py-1 text-xs rounded hover:bg-accent ${editor.isActive('bold') ? 'bg-accent' : ''}`}><b>B</b></button>
        <button type="button" aria-label="Italic" onClick={() => editor.chain().focus().toggleItalic().run()} className={`px-2 py-1 text-xs rounded hover:bg-accent ${editor.isActive('italic') ? 'bg-accent' : ''}`}><i>I</i></button>
        <button type="button" aria-label="Underline" onClick={() => editor.chain().focus().toggleUnderline().run()} className={`px-2 py-1 text-xs rounded hover:bg-accent ${editor.isActive('underline') ? 'bg-accent' : ''}`}><u>U</u></button>
        <button type="button" aria-label="Code" onClick={() => editor.chain().focus().toggleCode().run()} className={`px-2 py-1 text-xs rounded hover:bg-accent ${editor.isActive('code') ? 'bg-accent' : ''}`}>{'</>'}</button>
      </BubbleMenu>

      <FloatingMenu editor={editor} className="flex gap-1 rounded-md border bg-popover p-1 shadow-md">
        <button type="button" aria-label="Heading 2" onClick={() => editor.chain().focus().toggleHeading({ level: 2 }).run()} className="px-2 py-1 text-xs rounded hover:bg-accent">H2</button>
        <button type="button" aria-label="Bullet list" onClick={() => editor.chain().focus().toggleBulletList().run()} className="px-2 py-1 text-xs rounded hover:bg-accent">• List</button>
        <button type="button" aria-label="Quote" onClick={() => editor.chain().focus().toggleBlockquote().run()} className="px-2 py-1 text-xs rounded hover:bg-accent">&ldquo;</button>
      </FloatingMenu>

      <EditorContent editor={editor} />

      <div className="flex items-center justify-between border-t border-input px-3 py-1.5 text-xs text-muted-foreground">
        <span aria-live="polite">{charCount.toLocaleString()} characters</span>
        {autoSaveKey && <span className="italic">Autosave on</span>}
      </div>

      <RestoreAutosaveModal
        open={!!restorePrompt}
        savedAt={restorePrompt?.savedAt}
        onRestore={handleRestore}
        onDiscard={handleDiscard}
      />
    </div>
  );
}
