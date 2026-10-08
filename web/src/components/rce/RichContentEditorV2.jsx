import React, { useEffect, useRef, useState, useCallback, useMemo } from 'react';
import { useEditor, EditorContent } from '@tiptap/react';
import { BubbleMenu, FloatingMenu } from '@tiptap/react/menus';
import {
  Sparkles, Loader2, ChevronDown, Accessibility, FileCode2, Maximize2, Minimize2, Keyboard,
} from 'lucide-react';
import { sanitizeHTML } from '../RichContentViewer';
import { api, getCSRFToken } from '../../services/api';
import { useClickOutside } from '../../hooks/useClickOutside';
import RCEToolbar from './RCEToolbar';
import RestoreAutosaveModal from './RestoreAutosaveModal';
import ImageOptionsDialog from './ImageOptionsDialog';
import LinkDialog from './LinkDialog';
import A11yCheckerPanel from './A11yCheckerPanel';
import FindReplaceBar from './FindReplaceBar';
import KeyboardShortcutsDialog from './KeyboardShortcutsDialog';
import { buildRceExtensions } from './extensions';
import { checkDocument } from './a11yRules';
import { countWords } from './findReplace';
import { prepareEditorHtml, formatHtml, cleanPastedHtml } from './contentTransforms';
import { readRceDraft, scheduleRceDraftWrite, cancelRceDraftWrite, clearRceDraft } from './rceDrafts';
import { normalizeHTML, plainTextToHtml } from '../../utils/html';

/**
 * @typedef {Object} RCEV2Props
 * @property {string} value - Initial HTML content
 * @property {(html: string) => void} onChange - Called with sanitized HTML on every change
 * @property {string=} placeholder - Empty-state placeholder
 * @property {(string|number)=} courseId - Course context: content picker, image upload / course images
 * @property {string=} autoSaveKey - If set, drafts are kept (encrypted, per user) under this key.
 *   Scope it to the record (course + entity id) and call clearRceDraft(key)
 *   from ./rceDrafts after a successful save.
 * @property {string=} label - Accessible name of the editing area
 * @property {string=} className - Pass-through wrapper class
 */

const AUTOSAVE_DEBOUNCE_MS = 2000;
const RESTORE_THRESHOLD_MS = 5000; // ignore drafts older than props.value by less than this
const A11Y_RECHECK_MS = 600;

/** Draft vs saved value, ignoring sanitizer/serializer differences. */
function sameContent(a, b) {
  return normalizeHTML(sanitizeHTML(a || '')) === normalizeHTML(sanitizeHTML(b || ''));
}

const isImageFile = (f) => f && /^image\//.test(f.type || '');

/**
 * RichContentEditorV2 — the TipTap-based rich content editor used for every
 * rich-text field (pages, assignments, quizzes and questions, discussions,
 * announcements, syllabus).
 *
 * @param {RCEV2Props} props
 */
export default function RichContentEditorV2({
  value = '',
  onChange,
  placeholder = 'Start typing…',
  courseId,
  autoSaveKey,
  label = 'Rich content editor',
  className,
}) {
  const [restorePrompt, setRestorePrompt] = useState(null); // { html, savedAt }
  // Last HTML passed to onChange, so the value-sync effect can tell our own
  // echo apart from a real external change (parent reset, restore, ...).
  const lastEmittedRef = useRef(null);
  // AI Assist UI state
  const [aiMenuOpen, setAiMenuOpen] = useState(false);
  const [aiBusy, setAiBusy] = useState(null); // null | 'outline' | 'summarize' | 'rewrite'
  const [aiError, setAiError] = useState(null);
  const aiMenuRef = useRef(null);
  // Editor chrome
  const [sourceMode, setSourceMode] = useState(false);
  const [sourceHtml, setSourceHtml] = useState('');
  const [fullscreen, setFullscreen] = useState(false);
  const [a11yOpen, setA11yOpen] = useState(false);
  const [a11yCount, setA11yCount] = useState(0);
  const [findOpen, setFindOpen] = useState(false);
  const [shortcutsOpen, setShortcutsOpen] = useState(false);
  const [imageDialog, setImageDialog] = useState(null); // null | { mode, initial }
  const [linkDialog, setLinkDialog] = useState(null); // null | { initial, needsText, isEditing }
  const [upload, setUpload] = useState(null); // null | { progress } | { error }
  const wrapperRef = useRef(null);
  const courseIdRef = useRef(courseId);
  courseIdRef.current = courseId;
  const uploadImagesRef = useRef(null);

  const emit = useCallback((html) => {
    lastEmittedRef.current = html;
    onChange?.(html);
    if (autoSaveKey) scheduleRceDraftWrite(autoSaveKey, html, AUTOSAVE_DEBOUNCE_MS);
  }, [onChange, autoSaveKey]);
  const emitRef = useRef(emit);
  emitRef.current = emit;

  const extensions = useMemo(() => buildRceExtensions({ placeholder }), [placeholder]);

  const editor = useEditor({
    extensions,
    content: prepareEditorHtml(value || ''),
    editorProps: {
      attributes: {
        class: 'rce-v2-content prose prose-sm sm:prose-base max-w-none focus:outline-none min-h-[200px] px-4 py-3',
        role: 'textbox',
        'aria-multiline': 'true',
        'aria-label': label,
      },
      transformPastedHTML: (html) => cleanPastedHtml(html),
      // Pasted / dropped image files are uploaded to the course files and
      // inserted (G006). Without this they were silently dropped.
      handlePaste: (view, event) => {
        const files = Array.from(event.clipboardData?.files || []).filter(isImageFile);
        if (files.length === 0) return false;
        event.preventDefault();
        uploadImagesRef.current?.(files, null);
        return true;
      },
      handleDrop: (view, event, _slice, moved) => {
        if (moved) return false;
        const files = Array.from(event.dataTransfer?.files || []).filter(isImageFile);
        if (files.length === 0) return false;
        event.preventDefault();
        const at = view.posAtCoords({ left: event.clientX, top: event.clientY });
        uploadImagesRef.current?.(files, at ? at.pos : null);
        return true;
      },
    },
    onUpdate: ({ editor: ed }) => {
      emitRef.current(sanitizeHTML(ed.getHTML()));
    },
  });

  const uploadImages = useCallback(async (files, pos) => {
    if (!editor) return;
    if (courseIdRef.current == null) {
      setUpload({ error: 'Images can only be uploaded inside a course. Use Insert image to add one by URL.' });
      return;
    }
    let insertAt = pos;
    for (const file of files) {
      setUpload({ progress: 0 });
      try {
        const named = file.name && file.name !== 'image.png'
          ? file
          : new File([file], `pasted-image-${Date.now()}.${(file.type.split('/')[1] || 'png').replace('jpeg', 'jpg')}`, { type: file.type });
        // eslint-disable-next-line no-await-in-loop
        const attachment = await api.uploadCourseFileWithProgress(courseIdRef.current, named, (p) => setUpload({ progress: p }));
        if (!attachment?.id) throw new Error('Upload failed');
        const node = { type: 'image', attrs: { src: api.getFileDownloadUrl(attachment.id), alt: null } };
        if (insertAt != null) {
          editor.chain().focus().insertContentAt(insertAt, node).run();
          insertAt = null;
        } else {
          editor.chain().focus().insertContent(node).run();
        }
        setUpload(null);
      } catch (e) {
        setUpload({ error: e?.message || 'Image upload failed' });
        return;
      }
    }
  }, [editor]);
  uploadImagesRef.current = uploadImages;

  // Restore-prompt logic on mount
  useEffect(() => {
    if (!autoSaveKey || !editor) return undefined;
    let cancelled = false;
    readRceDraft(autoSaveKey).then((draft) => {
      if (cancelled || !draft || !draft.html) return;
      // Compare text, not markup: the server re-serializes saved HTML (rel /
      // target / entity changes), which made every draft look "different".
      if (sameContent(draft.html, value)) {
        clearRceDraft(autoSaveKey);
        return;
      }
      // Show prompt only when draft is meaningfully newer than what we received
      if (Date.now() - draft.savedAt > RESTORE_THRESHOLD_MS) {
        setRestorePrompt(draft);
      }
    });
    return () => { cancelled = true; };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [editor, autoSaveKey]);

  // Sync external value changes (e.g. parent reset)
  useEffect(() => {
    if (!editor) return;
    // Our own onChange echoing back. It is the *sanitized* HTML, which need
    // not match editor.getHTML() byte for byte (the sanitizer disables task
    // list checkboxes, for instance); treating that as an external change
    // reset the document and threw the caret on every keystroke.
    if (value !== undefined && value === lastEmittedRef.current) return;
    const current = editor.getHTML();
    if (value !== undefined && value !== current) {
      editor.commands.setContent(prepareEditorHtml(value || ''), { emitUpdate: false });
      if (sourceMode) setSourceHtml(formatHtml(sanitizeHTML(value || '')));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value, editor]);

  // Cleanup pending autosave on unmount (a save that unmounts the editor
  // must not be followed by a late draft write).
  useEffect(() => () => {
    if (autoSaveKey) cancelRceDraftWrite(autoSaveKey);
  }, [autoSaveKey]);

  // Accessibility issue count for the status bar (debounced).
  useEffect(() => {
    if (!editor) return undefined;
    let timer = null;
    const recheck = () => {
      clearTimeout(timer);
      timer = setTimeout(() => setA11yCount(checkDocument(editor).length), A11Y_RECHECK_MS);
    };
    setA11yCount(checkDocument(editor).length);
    editor.on('update', recheck);
    return () => { clearTimeout(timer); editor.off('update', recheck); };
  }, [editor]);

  const handleRestore = useCallback(() => {
    if (!restorePrompt || !editor) return;
    const clean = sanitizeHTML(restorePrompt.html);
    editor.commands.setContent(prepareEditorHtml(clean));
    onChange?.(clean);
    setRestorePrompt(null);
  }, [restorePrompt, editor, onChange]);

  const handleDiscard = useCallback(() => {
    if (autoSaveKey) clearRceDraft(autoSaveKey);
    setRestorePrompt(null);
  }, [autoSaveKey]);

  // ----- HTML source view (G004) --------------------------------------------
  const toggleSource = useCallback(() => {
    if (!editor) return;
    if (!sourceMode) {
      setSourceHtml(formatHtml(sanitizeHTML(editor.getHTML())));
      setFindOpen(false);
      setSourceMode(true);
      return;
    }
    // Back to the rich view: parse the edited source. emitUpdate makes the
    // parent receive the editor's normalized HTML.
    editor.commands.setContent(prepareEditorHtml(sanitizeHTML(sourceHtml)), { emitUpdate: true });
    setSourceMode(false);
    setTimeout(() => editor.commands.focus(), 0);
  }, [editor, sourceMode, sourceHtml]);

  const onSourceChange = useCallback((e) => {
    const raw = e.target.value;
    setSourceHtml(raw);
    // Saving while the source view is open saves what is typed here.
    emit(sanitizeHTML(raw));
  }, [emit]);

  // ----- Images (G006) ------------------------------------------------------
  const openInsertImage = useCallback(() => setImageDialog({ mode: 'insert', initial: null }), []);
  const openEditImage = useCallback(() => {
    if (!editor || !editor.isActive('image')) return;
    const a = editor.getAttributes('image');
    setImageDialog({ mode: 'edit', initial: { src: a.src, alt: a.alt, decorative: a.decorative, width: a.width } });
  }, [editor]);
  const submitImage = useCallback((attrs) => {
    if (!editor) return;
    const { src, alt, decorative, width } = attrs;
    if (imageDialog?.mode === 'edit') {
      editor.chain().focus().updateImage({ alt: decorative ? '' : alt, decorative, width }).run();
    } else {
      editor.chain().focus().insertContent({ type: 'image', attrs: { src, alt: decorative ? '' : alt, decorative, width } }).run();
    }
    setImageDialog(null);
  }, [editor, imageDialog]);

  // ----- Links (G014) -------------------------------------------------------
  const openLink = useCallback(() => {
    if (!editor) return;
    const isEditing = editor.isActive('link');
    const attrs = editor.getAttributes('link');
    setLinkDialog({
      isEditing,
      needsText: !isEditing && editor.state.selection.empty,
      initial: { href: attrs.href || '', newTab: attrs.target === '_blank', text: '' },
    });
  }, [editor]);
  const submitLink = useCallback(({ href, newTab, text }) => {
    if (!editor) return;
    const linkAttrs = { href, target: newTab ? '_blank' : null };
    if (linkDialog?.needsText) {
      editor.chain().focus().insertContent({
        type: 'text', text: text || href, marks: [{ type: 'link', attrs: linkAttrs }],
      }).run();
    } else {
      editor.chain().focus().extendMarkRange('link').setLink(linkAttrs).run();
    }
    setLinkDialog(null);
  }, [editor, linkDialog]);
  const removeLink = useCallback(() => {
    editor?.chain().focus().extendMarkRange('link').unsetLink().run();
    setLinkDialog(null);
  }, [editor]);

  // ----- Keyboard shortcuts (G016) ------------------------------------------
  const onKeyDown = useCallback((e) => {
    // React bubbles events out of portaled dialogs too; only handle keys
    // pressed in the editor itself.
    if (!wrapperRef.current?.contains(e.target)) return;
    const mod = e.metaKey || e.ctrlKey;
    if (e.altKey && !mod && (e.key === '0' || e.code === 'Digit0')) {
      e.preventDefault(); setShortcutsOpen(true);
    } else if (e.altKey && e.key === 'F10') {
      e.preventDefault();
      wrapperRef.current?.querySelector('.rce-toolbar button:not([disabled])')?.focus();
    } else if (e.altKey && e.key === 'F8') {
      e.preventDefault(); setA11yOpen(true);
    } else if (mod && e.shiftKey && (e.key === 'F' || e.key === 'f')) {
      e.preventDefault(); setFullscreen((f) => !f);
    } else if (mod && !e.shiftKey && !e.altKey && (e.key === 'f' || e.key === 'F') && !sourceMode) {
      e.preventDefault(); setFindOpen(true);
    } else if (mod && !e.shiftKey && (e.key === 'k' || e.key === 'K') && !sourceMode) {
      e.preventDefault(); openLink();
    } else if (e.key === 'Escape' && fullscreen && !findOpen) {
      setFullscreen(false);
    }
  }, [sourceMode, fullscreen, findOpen, openLink]);

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

      // The model returns plain text with newlines / "- " bullets. A bare
      // string is inserted as one text run (newlines collapse), so convert
      // it to escaped HTML paragraphs and lists first (B108).
      const resultHTML = plainTextToHtml(result, { markdownLite: true });
      const chain = editor.chain().focus();
      if (selectedText) {
        chain.deleteRange({ from, to }).insertContent(resultHTML).run();
      } else {
        chain.insertContent(resultHTML).run();
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
  const wordCount = sourceMode ? countWords(editor.getText()) : (editor.storage.characterCount?.words?.() ?? countWords(editor.getText()));

  return (
    <div
      ref={wrapperRef}
      onKeyDown={onKeyDown}
      className={[
        'rce-v2 rounded-md border border-input bg-background',
        fullscreen ? 'fixed inset-0 z-50 flex flex-col rounded-none' : '',
        className,
      ].filter(Boolean).join(' ')}
      data-fullscreen={fullscreen ? 'true' : undefined}
    >
      <RCEToolbar
        editor={editor}
        courseId={courseId}
        disabled={sourceMode}
        onInsertImage={openInsertImage}
        onEditImage={openEditImage}
        onLink={openLink}
        onFindReplace={() => setFindOpen(true)}
      />

      {/* AI Assist dropdown — proxies to /api/v1/ai_assist/:action (Anthropic Messages API). */}
      <div className="flex items-center gap-2 border-b border-input bg-muted/20 px-2 py-1">
        <div className="relative" ref={aiMenuRef}>
          <button
            type="button"
            aria-haspopup="menu"
            aria-expanded={aiMenuOpen}
            aria-label="AI Assist"
            disabled={!!aiBusy || sourceMode}
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

      {findOpen && !sourceMode && <FindReplaceBar editor={editor} onClose={() => setFindOpen(false)} />}
      {a11yOpen && !sourceMode && <A11yCheckerPanel editor={editor} onClose={() => setA11yOpen(false)} />}

      {upload && (
        <div role={upload.error ? 'alert' : 'status'} className={`flex items-center gap-2 border-b border-input px-3 py-1.5 text-xs ${upload.error ? 'text-destructive' : 'text-muted-foreground'}`}>
          {upload.error ? (
            <>
              <span className="flex-1">{upload.error}</span>
              <button type="button" className="underline" onClick={() => setUpload(null)}>dismiss</button>
            </>
          ) : (
            <span>Uploading image… {Math.round((upload.progress || 0) * 100)}%</span>
          )}
        </div>
      )}

      <BubbleMenu editor={editor} className="flex gap-1 rounded-md border bg-popover p-1 shadow-md">
        <button type="button" aria-label="Bold" onClick={() => editor.chain().focus().toggleBold().run()} className={`px-2 py-1 text-xs rounded hover:bg-accent ${editor.isActive('bold') ? 'bg-accent' : ''}`}><b>B</b></button>
        <button type="button" aria-label="Italic" onClick={() => editor.chain().focus().toggleItalic().run()} className={`px-2 py-1 text-xs rounded hover:bg-accent ${editor.isActive('italic') ? 'bg-accent' : ''}`}><i>I</i></button>
        <button type="button" aria-label="Underline" onClick={() => editor.chain().focus().toggleUnderline().run()} className={`px-2 py-1 text-xs rounded hover:bg-accent ${editor.isActive('underline') ? 'bg-accent' : ''}`}><u>U</u></button>
        <button type="button" aria-label="Code" onClick={() => editor.chain().focus().toggleCode().run()} className={`px-2 py-1 text-xs rounded hover:bg-accent ${editor.isActive('code') ? 'bg-accent' : ''}`}>{'</>'}</button>
        {editor.isActive('image') && (
          <button type="button" onClick={openEditImage} className="px-2 py-1 text-xs rounded hover:bg-accent">Alt text &amp; size</button>
        )}
        {editor.isActive('link') && (
          <button type="button" onClick={openLink} className="px-2 py-1 text-xs rounded hover:bg-accent">Edit link</button>
        )}
      </BubbleMenu>

      <FloatingMenu editor={editor} className="flex gap-1 rounded-md border bg-popover p-1 shadow-md">
        <button type="button" aria-label="Heading 2" onClick={() => editor.chain().focus().toggleHeading({ level: 2 }).run()} className="px-2 py-1 text-xs rounded hover:bg-accent">H2</button>
        <button type="button" aria-label="Bullet list" onClick={() => editor.chain().focus().toggleBulletList().run()} className="px-2 py-1 text-xs rounded hover:bg-accent">• List</button>
        <button type="button" aria-label="Quote" onClick={() => editor.chain().focus().toggleBlockquote().run()} className="px-2 py-1 text-xs rounded hover:bg-accent">&ldquo;</button>
      </FloatingMenu>

      <div className={fullscreen ? 'min-h-0 flex-1 overflow-auto' : undefined}>
        <div hidden={sourceMode}>
          <EditorContent editor={editor} />
        </div>
        {sourceMode && (
          <textarea
            aria-label="HTML source"
            spellCheck={false}
            value={sourceHtml}
            onChange={onSourceChange}
            className={`block w-full resize-y bg-background px-4 py-3 font-mono text-xs leading-5 focus:outline-none ${fullscreen ? 'h-full min-h-full' : 'min-h-[240px]'}`}
          />
        )}
      </div>

      <div className="flex flex-wrap items-center justify-between gap-2 border-t border-input px-3 py-1.5 text-xs text-muted-foreground">
        {/* Not a live region: announcing every keystroke drowned out
            dictation and review for screen-reader users (B107). */}
        <span>
          {wordCount.toLocaleString()} {wordCount === 1 ? 'word' : 'words'} · {charCount.toLocaleString()} characters
          {autoSaveKey && <span className="ml-2 italic">Autosave on</span>}
        </span>
        <span className="flex items-center gap-1">
          <button type="button" onClick={() => setA11yOpen((o) => !o)} disabled={sourceMode}
            aria-pressed={a11yOpen} title="Accessibility checker (Alt+F8)"
            aria-label={`Accessibility checker, ${a11yCount} ${a11yCount === 1 ? 'issue' : 'issues'}`}
            className="inline-flex items-center gap-1 rounded px-1.5 py-0.5 hover:bg-accent disabled:opacity-50">
            <Accessibility className="size-4" aria-hidden="true" />
            {a11yCount > 0 && <span className="rounded-full bg-destructive px-1.5 text-[10px] font-semibold text-destructive-foreground">{a11yCount}</span>}
          </button>
          <button type="button" onClick={toggleSource} aria-pressed={sourceMode}
            title={sourceMode ? 'Switch to the rich text editor' : 'Switch to the HTML editor'}
            aria-label={sourceMode ? 'Switch to the rich text editor' : 'Switch to the HTML editor'}
            className="inline-flex items-center rounded px-1.5 py-0.5 hover:bg-accent">
            <FileCode2 className="size-4" aria-hidden="true" />
          </button>
          <button type="button" onClick={() => setShortcutsOpen(true)} title="Keyboard shortcuts (Alt+0)"
            aria-label="Keyboard shortcuts" className="inline-flex items-center rounded px-1.5 py-0.5 hover:bg-accent">
            <Keyboard className="size-4" aria-hidden="true" />
          </button>
          <button type="button" onClick={() => setFullscreen((f) => !f)} aria-pressed={fullscreen}
            title={fullscreen ? 'Exit full screen' : 'Full screen'} aria-label={fullscreen ? 'Exit full screen' : 'Full screen'}
            className="inline-flex items-center rounded px-1.5 py-0.5 hover:bg-accent">
            {fullscreen ? <Minimize2 className="size-4" aria-hidden="true" /> : <Maximize2 className="size-4" aria-hidden="true" />}
          </button>
        </span>
      </div>

      <RestoreAutosaveModal
        open={!!restorePrompt}
        savedAt={restorePrompt?.savedAt}
        onRestore={handleRestore}
        onDiscard={handleDiscard}
      />
      <ImageOptionsDialog
        open={!!imageDialog}
        mode={imageDialog?.mode || 'insert'}
        initial={imageDialog?.initial}
        courseId={courseId}
        onClose={() => setImageDialog(null)}
        onSubmit={submitImage}
      />
      <LinkDialog
        open={!!linkDialog}
        initial={linkDialog?.initial}
        needsText={!!linkDialog?.needsText}
        isEditing={!!linkDialog?.isEditing}
        onClose={() => setLinkDialog(null)}
        onSubmit={submitLink}
        onRemove={removeLink}
      />
      <KeyboardShortcutsDialog open={shortcutsOpen} onClose={() => setShortcutsOpen(false)} />
    </div>
  );
}
